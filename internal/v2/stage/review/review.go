package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/events"
	legacyReview "github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/prompt"
	v2review "github.com/danabrams/gromit/internal/v2/review"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

const defaultReviewFragment = `# Post-Build Code Review Instructions

You are reviewing code changes from a build iteration to catch issues early. The diff and acceptance criteria are provided in the instance context above.

## Review Dimensions

Review the changes across these 7 dimensions:

### 1. Intent & Spec Drift
- Do changes fulfill the bead's intent, not just pass tests?
- Does the implementation match what was actually requested?
- Are there unnecessary scope additions?

### 2. Correctness
- Does the code work beyond the test coverage?
- Are there edge cases not handled?
- Are error conditions properly handled?

### 3. Security
- SQL injection, XSS, command injection risks?
- Authentication/authorization bypass?
- Data exposure or logging of secrets?
- OWASP top 10 concerns?

### 4. Test Gaps
- Are there untested code paths?
- Missing edge case tests?
- Are tests actually validating behavior or just passing?

### 5. Consistency
- Does new code match existing patterns in the project?
- Naming conventions followed?

### 6. Code Quality
- Dead code or unused imports?
- Missing or incorrect error handling?
- Overly complex logic that should be simplified?

### 7. Wiring Completeness
- Are new interfaces/stages actually called with real data, not empty/placeholder values?
- Are prompt layers, config fields, and dependency injections populated with meaningful content?
- Does the integration point (where components are assembled) pass the same quality of data that tests assume?

## Issue Triage

Categorize each issue you find:

**Fix immediately** (trivial issues):
- Missing error checks, poor naming, dead code removal, simple refactoring

**Create bead** (significant work needing dedicated iteration):
- New functionality, complex refactoring, multiple files/systems
- Provide: title, description, priority (0-2), labels

**Backlog** (needs design discussion or product owner input):
- Architectural decisions, unclear requirements, cross-system impacts
- Provide: title, description, reason

## Output Format

**Learnings**: Only emit learnings for violations of project rules, novel patterns worth noting, or failure gotchas. Do NOT emit learnings that merely confirm existing practices.

Return a JSON object:
` + "```" + `json
{
  "passed": true,
  "fixes_applied": [],
  "fix_categories": [],
  "beads_to_create": [{"title": "...", "description": "...", "priority": 1, "labels": ["from-review"]}],
  "backlog_items": [{"title": "...", "description": "...", "reason": "..."}],
  "learnings": [],
  "summary": "1-2 sentence summary."
}
` + "```" + `

Notes:
- passed: true if no blocking issues, false if major problems exist
- All array fields default to empty arrays if nothing to report
- All review-created beads automatically get a from-review label
- Be concise but specific. Each finding should be actionable

Respond with ONLY the JSON object. No markdown wrapper, no explanation.
`

// GitDiffer provides the git diff capability needed by the review stage.
type GitDiffer interface {
	Diff(ctx context.Context, worktree string) (string, error)
}

// ReviewArtifacts captures data emitted by the review stage.
type ReviewArtifacts struct {
	CreatedBeads []*trackertypes.Bead
	OutOfScope   []v2review.Finding
}

// Stage executes the review stage of the v2 run loop.
type Stage struct {
	name     string
	cfg      *config.Config
	git      GitDiffer
	llm      llmtypes.LLMProvider
	tracker  trackertypes.TaskTracker
	base     string
	project  string
	fragment string
	events.EmitterMixin
}

// New constructs a review stage with the provided dependencies.
func New(cfg *config.Config, gitAdapter GitDiffer, provider llmtypes.LLMProvider, tracker trackertypes.TaskTracker, base, project, fragment string) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if gitAdapter == nil {
		return nil, fmt.Errorf("git adapter required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	if tracker == nil {
		return nil, fmt.Errorf("task tracker required")
	}

	if strings.TrimSpace(fragment) == "" {
		fragment = defaultReviewFragment
	}
	return &Stage{
		name:     stagedesc.Describe("review", cfg),
		cfg:      cfg,
		git:      gitAdapter,
		llm:      provider,
		tracker:  tracker,
		base:     base,
		project:  project,
		fragment: fragment,
	}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// WithEmitter attaches the provided emitter for audit logging.
func (s *Stage) WithEmitter(emitter *events.Emitter) *Stage {
	s.EmitterMixin.SetEmitter(emitter)
	return s
}

// Name returns the canonical stage name.
func (s *Stage) Name() string {
	return s.name
}

// Run invokes the review LLM, classifies findings, and emits artifacts.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	cfg, err := s.resolveConfig(req)
	if err != nil {
		return nil, err
	}
	if !cfg.Review.Enabled {
		return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
	}
	specID := strings.TrimSpace(req.Bead.ID)
	if specID == "" {
		return nil, fmt.Errorf("spec ID required")
	}

	root, err := s.rootPath(req, cfg)
	if err != nil {
		return nil, fmt.Errorf("review: %w", err)
	}

	diff, err := s.git.Diff(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("review: git diff: %w", err)
	}

	acceptance, err := s.acceptanceLayer(cfg, req, root)
	if err != nil {
		return nil, fmt.Errorf("review: acceptance: %w", err)
	}

	instance := buildInstanceLayer(diff, acceptance)
	promptText := prompt.NewPromptAssembler(s.base, s.project, instance, s.fragment).Assemble("review", prompt.BeadInfo{Title: req.Bead.Title})

	provider := s.llm
	if req.Provider != nil {
		provider = req.Provider
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		var err2 error
		model, err2 = reviewModel(cfg)
		if err2 != nil {
			return nil, err2
		}
	}

	resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: promptText, Model: model, Dir: req.Worktree})
	if err != nil {
		return nil, fmt.Errorf("review: invoking llm: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("review: llm response nil")
	}
	if !resp.Success {
		return nil, fmt.Errorf("review: llm invocation failed: %s", resp.Output)
	}

	reviewResult, err := legacyReview.ParseReviewResult(resp.Output)
	if err != nil {
		return nil, fmt.Errorf("review: parse result: %w", err)
	}

	findings := convertToFindings(reviewResult)
	parent := &bead.Bead{ID: specID, Labels: append([]string(nil), req.Bead.Labels...)}
	classifier := v2review.NewClassifier(s.Emitter)
	classified := classifier.Classify(parent, findings)

	created, err := s.createBeads(ctx, parent.ID, parent.Labels, classified.Beads)
	if err != nil {
		return nil, err
	}

	artifacts := &ReviewArtifacts{CreatedBeads: created, OutOfScope: classified.OutOfScope}
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed, Artifacts: artifacts}, nil
}

func (s *Stage) resolveConfig(req *stagepkg.Request) (*config.Config, error) {
	cfg := req.Config
	if cfg == nil {
		cfg = s.cfg
	}
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return cfg, nil
}

func (s *Stage) acceptanceLayer(cfg *config.Config, req *stagepkg.Request, root string) (string, error) {
	path, err := s.specFilePath(cfg, req, root)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read spec: %w", err)
	}
	criteria, err := coverage.ParseCriteria(string(data))
	if err != nil {
		return "", fmt.Errorf("parse acceptance criteria: %w", err)
	}
	return formatCriteria(criteria), nil
}

func (s *Stage) specFilePath(cfg *config.Config, req *stagepkg.Request, root string) (string, error) {
	specID := strings.TrimSpace(req.Bead.ID)
	if specID == "" {
		return "", fmt.Errorf("spec ID required")
	}
	specDir := cfg.Paths.Specs
	if specDir == "" {
		specDir = ".gromit/specs"
	}
	specName := specID
	if filepath.Ext(specName) == "" {
		specName += ".md"
	}
	if !filepath.IsAbs(specDir) {
		specDir = filepath.Join(root, specDir)
	}
	return filepath.Join(specDir, specName), nil
}

func (s *Stage) rootPath(req *stagepkg.Request, cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config required")
	}
	if req == nil {
		return "", fmt.Errorf("request required")
	}
	root := strings.TrimSpace(req.Worktree)
	if root == "" {
		root = cfg.ProjectRoot
	}
	if root == "" {
		root = "."
	}
	return root, nil
}

func formatCriteria(criteria []coverage.Criterion) string {
	if len(criteria) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("## Acceptance Criteria")
	for _, c := range criteria {
		builder.WriteString(fmt.Sprintf("\n%d. %s", c.Number, c.Text))
	}
	return builder.String()
}

func buildInstanceLayer(diff, acceptance string) string {
	var sections []string
	if trimmed := strings.TrimSpace(diff); trimmed != "" {
		sections = append(sections, fmt.Sprintf("## Diff\n%s", trimmed))
	}
	if acceptance != "" {
		sections = append(sections, acceptance)
	}
	return strings.Join(sections, "\n\n")
}

func reviewModel(cfg *config.Config) (string, error) {
	if tier := strings.TrimSpace(cfg.Review.Tier); tier != "" {
		return tier, nil
	}
	if model := strings.TrimSpace(cfg.Review.Model); model != "" {
		return model, nil
	}
	return "", fmt.Errorf("review model or tier required")
}

func convertToFindings(result *legacyReview.ReviewResult) []v2review.Finding {
	findings := make([]v2review.Finding, 0, len(result.BeadsToCreate)+len(result.BacklogItems))
	for _, proposal := range result.BeadsToCreate {
		findings = append(findings, v2review.Finding{
			Title:       proposal.Title,
			Description: proposal.Description,
			Priority:    proposal.Priority,
			InScope:     true,
		})
	}
	for _, backlog := range result.BacklogItems {
		findings = append(findings, v2review.Finding{
			Title:       backlog.Title,
			Description: backlog.Description,
			Priority:    1,
			InScope:     false,
		})
	}
	return findings
}

func (s *Stage) createBeads(ctx context.Context, parentID string, parentLabels []string, proposals []*bead.Bead) ([]*trackertypes.Bead, error) {
	if len(proposals) == 0 {
		return nil, nil
	}
	specLabel := bead.FindSpecLabel(parentLabels)
	created := make([]*trackertypes.Bead, 0, len(proposals))
	for _, proposal := range proposals {
		labels := copyStrings(proposal.Labels)
		if parentID != "" {
			labels = append(labels, "review-source:"+parentID)
		}
		if specLabel != "" {
			labels = append(labels, tracker.SpecLabelFor(specLabel))
		}
		trackerResp, err := s.tracker.CreateBead(ctx, trackertypes.TaskTrackerCreateBeadRequest{
			Title:       proposal.Title,
			Description: proposal.Description,
			Priority:    proposal.Priority,
			Labels:      labels,
		})
		if err != nil {
			return nil, fmt.Errorf("review: creating bead %q: %w", proposal.Title, err)
		}
		if trackerResp == nil || trackerResp.Bead == nil {
			return nil, fmt.Errorf("review: create bead response missing for %q", proposal.Title)
		}
		created = append(created, trackerResp.Bead)
	}
	return created, nil
}

func copyStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
