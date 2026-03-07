package decompose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/generation"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
	"github.com/danabrams/gromit/internal/validate"
	"github.com/danabrams/gromit/skills"
)

const (
	defaultGromitDir            = ".gromit"
	v2DirName                   = "v2"
	planFileName                = "plan.md"
	gapFileName                 = "gap-analysis.md"
	specLabelFormat             = "spec:%s"
	complexityHighLabel         = "complexity:high"
	estimatedFilesLabelFormat   = "estimated-files:%d"
	highComplexityFileThreshold = 5
	providerOutputPreviewLimit  = 500
)

const decomposePromptTemplate = `# Decompose Plan: %s

You are decomposing an implementation plan into bd beads following the gromit-decompose skill.

## Plan Content

%s

## Skill Instructions

%s

## Output

Output ONLY a JSON array of bead definitions. No markdown, no explanations, no wrapper.
Each bead must include: title, description, priority, acceptance_criteria, expected_outputs, covers_tasks, depends_on_index.

expected_outputs: list each individual deliverable, function, or independently testable item as a separate entry. These drive TDD RED-GREEN cycles — one cycle per entry. Do not summarize or group; enumerate fine-grained items.
covers_tasks: list the 1-based Task numbers from the plan that this bead covers. Every Task in the plan must be covered by at least one bead.
depends_on_index: array of 0-based indices of prerequisite beads in THIS output array. If bead at index 2 needs types or functions introduced by beads at indices 0 and 1, set "depends_on_index": [0, 1]. Plans with sequential tasks MUST produce dependency chains — only root beads with no prerequisites should have an empty array. Most beads should depend on at least one earlier bead.

The spec label will be added automatically: spec:%s
`

// Stage implements the decompose stage of the run loop.
type Stage struct {
	name    string
	cfg     *config.Config
	llm     llm.LLMProvider
	tracker tasktracker.TaskTracker
}

// New constructs a decompose stage backed by the provided dependencies.
func New(cfg *config.Config, provider llm.LLMProvider, tracker tasktracker.TaskTracker) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	if tracker == nil {
		return nil, fmt.Errorf("task tracker required")
	}
	return &Stage{
		name:    stagedesc.Describe("decompose", cfg),
		cfg:     cfg,
		llm:     provider,
		tracker: tracker,
	}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the stage identifier consumed by the loop.
func (s *Stage) Name() string {
	return s.name
}

// Run executes the decompose stage.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	specID := strings.TrimSpace(req.Bead.ID)
	if specID == "" {
		return nil, fmt.Errorf("spec ID required")
	}

	planPath, err := s.planPath(req)
	if err != nil {
		return nil, err
	}

	planBody, err := os.ReadFile(planPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plan not found: %s", planPath)
		}
		return nil, fmt.Errorf("read plan: %w", err)
	}

	promptText := buildDecomposePrompt(specID, string(planBody), skills.DecomposeSkill)
	model := s.modelForPhase()
	currentPrompt := promptText
	maxSubBeads := s.cfg.Validation.PlanMaxSubBeadsValue()
	maxRetries := normalizeMaxValidationRetries(s.cfg.Validation.MaxValidationRetries)

	var beadDefs []beadDef
	for attempt := 0; ; attempt++ {
		defs, err := s.invokeProvider(ctx, currentPrompt, model)
		if err != nil {
			return nil, err
		}
		if len(defs) == 0 {
			return nil, fmt.Errorf("no beads extracted from plan")
		}
		candidates := toBeadCandidates(defs)
		validation := validate.ValidateDecomposeOutputWithMax(candidates, validate.DecomposeValidationModePipeline, "", maxSubBeads)
		violations := append([]validate.Violation(nil), validation.Violations...)
		violations = append(violations, validation.BatchViolations...)
		if len(violations) == 0 {
			beadDefs = defs
			break
		}
		if attempt >= maxRetries {
			return nil, fmt.Errorf("decomposition validation failed: %s", formatViolations(violations))
		}
		currentPrompt = validate.BuildReprompt(promptText, candidates, violations)
		if feedback := buildComplexityRepromptFeedback(defs); feedback != "" {
			currentPrompt += "\n\n" + feedback
		}
	}

	createdIDs := make([]string, 0, len(beadDefs))
	createdBeads := make([]*bead.Bead, 0, len(beadDefs))

	for idx, def := range beadDefs {
		priority := parsePriority(def.Priority)
		labels := s.buildLabels(specID, def.EstimatedFiles, req)
		dependencies := resolveDependencies(def.DependsOnIndex, createdIDs, idx)

		trackerResp, err := s.tracker.CreateBead(ctx, tasktracker.CreateBeadRequest{
			Title:        def.Title,
			Description:  def.Description,
			Priority:     priority,
			Labels:       labels,
			Dependencies: dependencies,
		})
		if err != nil {
			return nil, fmt.Errorf("creating bead %d: %w", idx, err)
		}
		if trackerResp == nil || trackerResp.Bead == nil {
			return nil, fmt.Errorf("create bead response missing result")
		}
		createdIDs = append(createdIDs, trackerResp.Bead.ID)
		createdBeads = append(createdBeads, convertTrackerBead(trackerResp.Bead))
	}

	return &stagepkg.Result{
		Decision:  stagepkg.DecisionProceed,
		Artifacts: &stagepkg.DecomposeArtifacts{Beads: createdBeads},
	}, nil
}

func (s *Stage) planPath(req *stagepkg.Request) (string, error) {
	cfg := req.Config
	if cfg == nil {
		cfg = s.cfg
	}
	if cfg == nil {
		return "", fmt.Errorf("config required")
	}
	root := strings.TrimSpace(req.Worktree)
	if root == "" {
		root = cfg.ProjectRoot
	}
	if root == "" {
		root = "."
	}
	gromitDir := cfg.Paths.GromitDir
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	fileName := planFileName
	if req.Remediation {
		fileName = gapFileName
	}
	return filepath.Join(root, gromitDir, v2DirName, fileName), nil
}

func (s *Stage) modelForPhase() string {
	tier := s.cfg.PhaseTierForStrategy("decompose", provider.TierMedium)
	if tier == "" {
		tier = provider.TierMedium
	}
	return provider.TierToLegacyModel(tier)
}

func (s *Stage) invokeProvider(ctx context.Context, prompt, model string) ([]beadDef, error) {
	resp, err := s.llm.Invoke(ctx, llm.InvokeRequest{Prompt: prompt, Model: model})
	if err != nil {
		return nil, fmt.Errorf("invoking provider: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("provider returned nil response")
	}
	if !resp.Success {
		return nil, fmt.Errorf("provider invocation failed: %s", resp.Output)
	}
	output := strings.TrimSpace(resp.Output)
	if output == "" {
		return nil, fmt.Errorf("provider returned empty output")
	}
	var defs []beadDef
	if err := jsonutil.ExtractJSON(output, &defs); err != nil {
		preview := output
		if len(preview) > providerOutputPreviewLimit {
			preview = preview[:providerOutputPreviewLimit] + "... (truncated)"
		}
		return nil, fmt.Errorf("parsing bead definitions: %w\n\nProvider output:\n%s", err, preview)
	}
	return defs, nil
}

func (s *Stage) buildLabels(specID string, estimatedFiles int, req *stagepkg.Request) []string {
	labels := []string{specLabel(specID), generation.Format(s.generationForRequest(req))}
	if estimatedFiles > highComplexityFileThreshold {
		labels = append(labels, complexityHighLabel)
	}
	if estimatedFiles > 0 {
		labels = append(labels, fmt.Sprintf(estimatedFilesLabelFormat, estimatedFiles))
	}
	return labels
}

func (s *Stage) generationForRequest(req *stagepkg.Request) int {
	gen := generation.Current(req.Bead.Labels)
	if req.Remediation {
		gen++
	}
	return gen
}

func resolveDependencies(indexes []int, createdIDs []string, current int) []string {
	var deps []string
	for _, idx := range indexes {
		if idx == current {
			continue
		}
		if idx < 0 || idx >= len(createdIDs) {
			continue
		}
		deps = append(deps, createdIDs[idx])
	}
	return deps
}

func convertTrackerBead(src *tasktracker.Bead) *bead.Bead {
	if src == nil {
		return nil
	}
	return &bead.Bead{
		ID:          src.ID,
		Title:       src.Title,
		Description: src.Description,
		Priority:    src.Priority,
		Labels:      append([]string(nil), src.Labels...),
		DependsOn:   dependencyList(src.DependsOn),
	}
}

func dependencyList(ids []string) []bead.Dependency {
	if len(ids) == 0 {
		return nil
	}
	deps := make([]bead.Dependency, 0, len(ids))
	for _, id := range ids {
		deps = append(deps, bead.Dependency{ID: id})
	}
	return deps
}

func specLabel(specID string) string {
	return fmt.Sprintf(specLabelFormat, specID)
}

func buildDecomposePrompt(planName, planBody, skillContent string) string {
	return fmt.Sprintf(decomposePromptTemplate, planName, planBody, skillContent, planName)
}

func toBeadCandidates(defs []beadDef) []validate.BeadCandidate {
	res := make([]validate.BeadCandidate, len(defs))
	for i, def := range defs {
		expected := def.ExpectedOutputs
		if len(expected) == 0 {
			expected = def.AcceptanceCriteria
		}
		res[i] = validate.BeadCandidate{
			Title:              def.Title,
			Description:        def.Description,
			CoversTasks:        def.CoversTasks,
			DependsOnIndex:     def.DependsOnIndex,
			EstimatedFiles:     def.EstimatedFiles,
			AcceptanceCriteria: def.AcceptanceCriteria,
			ExpectedOutputs:    expected,
		}
	}
	return res
}

func formatViolations(violations []validate.Violation) string {
	rules := make([]string, 0, len(violations))
	for _, v := range violations {
		rules = append(rules, fmt.Sprintf("[%s] %s", v.Rule, v.Message))
	}
	return strings.Join(rules, ", ")
}

func buildComplexityRepromptFeedback(defs []beadDef) string {
	classification := validate.ValidateDecomposeCandidates(toBeadCandidates(defs))
	feedback := validate.BuildComplexityRepromptFeedback(classification.ComplexityOutcome.HighComplexity)
	if feedback == "" {
		return ""
	}
	return "Complexity feedback:\n" + feedback
}

func normalizeMaxValidationRetries(maxRetries int) int {
	if maxRetries < 0 {
		return 0
	}
	return maxRetries
}

func parsePriority(p string) int {
	switch strings.ToUpper(p) {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	default:
		return 1
	}
}

type beadDef struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Priority           string   `json:"priority"`
	EstimatedFiles     int      `json:"estimated_files,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	ExpectedOutputs    []string `json:"expected_outputs,omitempty"`
	CoversTasks        []int    `json:"covers_tasks,omitempty"`
	DependsOnIndex     []int    `json:"depends_on_index"`
}
