package specreview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

const (
	defaultGromitDir = ".gromit"
	v2DirName        = "v2"
	planFileName     = "plan.md"
)

// GitDiffer provides the git diff capability needed by the spec review stage.
type GitDiffer interface {
	DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// SpecReviewArtifacts captures the parsed output from the spec review prompt.
type SpecReviewArtifacts struct {
	Summary      string
	Findings     []SpecReviewFinding
	Verdict      string
	CreatedBeads []*trackertypes.Bead
}

// SpecReviewFinding describes a single issue or pass item produced by the review.
type SpecReviewFinding struct {
	Title         string
	Description   string
	Verdict       string
	Severity      stagepkg.SpecFindingSeverity
	Category      stagepkg.SpecFindingCategory
	Scope         stagepkg.SpecFindingScope
	AffectedFiles []string
}

// parseSpecReviewOutput reads the JSON output produced by review_spec_v2.md and
// returns the parsed artifact model.
func parseSpecReviewOutput(output string) (*SpecReviewArtifacts, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, fmt.Errorf("spec review output required")
	}

	var payload struct {
		Findings []struct {
			Verdict       string   `json:"verdict"`
			Severity      string   `json:"severity"`
			Category      string   `json:"category"`
			Scope         string   `json:"scope"`
			Description   string   `json:"description"`
			AffectedFiles []string `json:"affected_files"`
		} `json:"findings"`
		Summary string `json:"summary"`
	}

	if err := jsonutil.ExtractObject(trimmed, &payload); err != nil {
		return nil, fmt.Errorf("parse spec review output: %w", err)
	}

	findings := make([]SpecReviewFinding, 0, len(payload.Findings))
	for _, raw := range payload.Findings {
		findings = append(findings, SpecReviewFinding{
			Title:         normalizeTitle(raw.Scope),
			Description:   strings.TrimSpace(raw.Description),
			Verdict:       strings.ToLower(strings.TrimSpace(raw.Verdict)),
			Severity:      stagepkg.SpecFindingSeverity(strings.ToLower(strings.TrimSpace(raw.Severity))),
			Category:      stagepkg.SpecFindingCategory(strings.ToLower(strings.TrimSpace(raw.Category))),
			Scope:         stagepkg.SpecFindingScope(strings.TrimSpace(raw.Scope)),
			AffectedFiles: cloneStrings(raw.AffectedFiles),
		})
	}

	return &SpecReviewArtifacts{
		Summary:  strings.TrimSpace(payload.Summary),
		Findings: findings,
		Verdict:  verdictFromFindings(findings),
	}, nil
}

func normalizeTitle(scope string) string {
	if trimmed := strings.TrimSpace(scope); trimmed != "" {
		return trimmed
	}
	return "spec review finding"
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func verdictFromFindings(findings []SpecReviewFinding) string {
	for _, finding := range findings {
		verdict := strings.ToLower(strings.TrimSpace(finding.Verdict))
		if verdict == "issue" || verdict == "fail" {
			return "fail"
		}
	}
	return "pass"
}

// Stage implements the spec-level review stage.
type Stage struct {
	name     string
	cfg      *config.Config
	git      GitDiffer
	llm      llmtypes.LLMProvider
	tracker  trackertypes.TaskTracker
	base     string
	project  string
	fragment string
}

var _ stagepkg.Stage = (*Stage)(nil)

// New creates a spec review stage backed by the provided configuration.
func New(cfg *config.Config, git GitDiffer, provider llmtypes.LLMProvider, tracker trackertypes.TaskTracker, base, project, fragment string) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if git == nil {
		return nil, fmt.Errorf("git adapter required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}

	return &Stage{
		name:     stagedesc.Describe("spec-review", cfg),
		cfg:      cfg,
		git:      git,
		llm:      provider,
		tracker:  tracker,
		base:     base,
		project:  project,
		fragment: fragment,
	}, nil
}

// Name returns the stage identifier.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run executes the spec review stage.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	cfg, err := s.resolveConfig(req)
	if err != nil {
		return nil, err
	}

	specID := strings.TrimSpace(req.Bead.ID)
	if specID == "" {
		return nil, fmt.Errorf("spec ID required")
	}

	root := s.resolveRoot(req)
	planPath := specPlanPath(root, cfg)
	planData, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("read plan: %w", err)
	}
	planText := strings.TrimSpace(string(planData))

	diff, err := s.git.DiffFromBase(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	instance := s.buildInstanceLayer(specID, req.Bead, planText, diff)
	promptText := prompt.NewPromptAssembler(s.base, s.project, instance, s.fragment).Assemble("spec-review", prompt.BeadInfo{Title: req.Bead.Title})

	provider := s.llm
	if req.Provider != nil {
		provider = req.Provider
	}

	model := s.selectModel(cfg, req)
	resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: promptText, Model: model, Dir: root})
	if err != nil {
		return nil, fmt.Errorf("spec review: invoking llm: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("spec review: llm response nil")
	}
	if !resp.Success {
		detail := strings.TrimSpace(resp.Output)
		if detail == "" {
			detail = "no detail available"
		}
		return nil, fmt.Errorf("spec review: llm invocation failed: %s", detail)
	}

	artifacts, err := parseSpecReviewOutput(resp.Output)
	if err != nil {
		return nil, fmt.Errorf("spec review: parse output: %w", err)
	}

	decision := decisionFromArtifacts(artifacts)
	if err := s.createFromReviewBeads(ctx, specID, artifacts, decision); err != nil {
		return nil, err
	}
	return &stagepkg.Result{Decision: decision, Artifacts: artifacts}, nil
}

func (s *Stage) resolveConfig(req *stagepkg.Request) (*config.Config, error) {
	if req != nil && req.Config != nil {
		return req.Config, nil
	}
	if s.cfg != nil {
		return s.cfg, nil
	}
	return nil, fmt.Errorf("config required")
}

func (s *Stage) resolveRoot(req *stagepkg.Request) string {
	if req != nil {
		if trimmed := strings.TrimSpace(req.Worktree); trimmed != "" {
			return trimmed
		}
	}
	if s.cfg != nil {
		if trimmed := strings.TrimSpace(s.cfg.ProjectRoot); trimmed != "" {
			return trimmed
		}
	}
	return "."
}

func specPlanPath(root string, cfg *config.Config) string {
	gromitDir := strings.TrimSpace(cfg.Paths.GromitDir)
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	gromitDir = resolveCandidatePath(root, gromitDir)
	return filepath.Join(gromitDir, v2DirName, planFileName)
}

func (s *Stage) buildInstanceLayer(specID string, bead stagepkg.BeadInfo, plan, diff string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Spec: %s\n", specID))
	if title := strings.TrimSpace(bead.Title); title != "" {
		builder.WriteString("Bead Title: ")
		builder.WriteString(title)
		builder.WriteString("\n")
	}
	if description := strings.TrimSpace(bead.Description); description != "" {
		builder.WriteString("\nBead Description:\n")
		builder.WriteString(description)
		builder.WriteString("\n")
	}

	builder.WriteString("\nPlan:\n")
	if trimmed := strings.TrimSpace(plan); trimmed != "" {
		builder.WriteString(trimmed)
	} else {
		builder.WriteString("(no plan available)")
	}

	builder.WriteString("\n\nDiff:\n")
	if trimmed := strings.TrimSpace(diff); trimmed != "" {
		builder.WriteString(trimmed)
	} else {
		builder.WriteString("(no diff provided)")
	}
	builder.WriteString("\n")

	return builder.String()
}

func resolveCandidatePath(root, candidate string) string {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return root
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(root, trimmed)
}

func (s *Stage) selectModel(cfg *config.Config, req *stagepkg.Request) string {
	if req != nil {
		if trimmed := strings.TrimSpace(req.Model); trimmed != "" {
			return trimmed
		}
	}
	if cfg != nil {
		if trimmed := strings.TrimSpace(cfg.SpecGate.Model); trimmed != "" {
			return trimmed
		}
	}
	return config.ModelOpus
}

func decisionFromArtifacts(artifacts *SpecReviewArtifacts) stagepkg.Decision {
	if artifacts == nil {
		return stagepkg.DecisionProceed
	}
	verdict := strings.ToLower(strings.TrimSpace(artifacts.Verdict))
	if verdict == "issue" || verdict == "fail" {
		return stagepkg.DecisionFail
	}
	for _, finding := range artifacts.Findings {
		if strings.EqualFold(strings.TrimSpace(finding.Verdict), "issue") {
			return stagepkg.DecisionFail
		}
		switch finding.Severity {
		case stagepkg.SpecFindingSeverityCritical, stagepkg.SpecFindingSeverityHigh:
			return stagepkg.DecisionFail
		}
	}
	return stagepkg.DecisionProceed
}

func (s *Stage) createFromReviewBeads(ctx context.Context, specID string, artifacts *SpecReviewArtifacts, decision stagepkg.Decision) error {
	if artifacts == nil || len(artifacts.Findings) == 0 || s.tracker == nil {
		return nil
	}
	if decision != stagepkg.DecisionProceed {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(artifacts.Verdict), "pass") {
		return nil
	}

	created := make([]*trackertypes.Bead, 0, len(artifacts.Findings))
	for _, finding := range artifacts.Findings {
		description := strings.TrimSpace(finding.Description)
		if description == "" {
			continue
		}

		labels := []string{"from-review"}
		if strings.EqualFold(strings.TrimSpace(string(finding.Scope)), string(stagepkg.SpecFindingScopeSpec)) {
			labels = append(labels, tracker.SpecLabelFor(specID))
		}

		resp, err := s.tracker.CreateBead(ctx, trackertypes.TaskTrackerCreateBeadRequest{
			Title:       description,
			Description: description,
			Priority:    2,
			Labels:      labels,
		})
		if err != nil {
			return fmt.Errorf("spec review: create bead %q: %w", description, err)
		}
		if resp == nil || resp.Bead == nil {
			return fmt.Errorf("spec review: create bead response missing for %q", description)
		}
		created = append(created, resp.Bead)
	}

	if len(created) > 0 {
		artifacts.CreatedBeads = created
	}
	return nil
}

func labelsForFinding(specID, scope string) []string {
	labels := []string{"from-review"}
	if strings.EqualFold(strings.TrimSpace(scope), "spec") {
		if spec := strings.TrimSpace(specID); spec != "" {
			labels = append(labels, tracker.SpecLabelFor(spec))
		}
	}
	return labels
}

func beadTitleForFinding(f stagepkg.Finding) string {
	if trimmed := strings.TrimSpace(f.Description); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(f.Category); trimmed != "" {
		return fmt.Sprintf("Spec review: %s", trimmed)
	}
	if trimmed := strings.TrimSpace(f.Scope); trimmed != "" {
		return fmt.Sprintf("Spec review: %s finding", trimmed)
	}
	return "Spec review finding"
}

func priorityForSeverity(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 0
	case "warning":
		return 1
	case "suggestion":
		return 2
	default:
		return 2
	}
}
