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
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/prompt"
	v2review "github.com/danabrams/gromit/internal/v2/review"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagesreview "github.com/danabrams/gromit/internal/v2/stages/review"
)

// ReviewArtifacts captures data emitted by the review stage.
type ReviewArtifacts struct {
	CreatedBeads []*tasktracker.Bead
	OutOfScope   []v2review.Finding
}

// Stage executes the review stage of the v2 run loop.
type Stage struct {
	name     string
	cfg      *config.Config
	git      adapter.GitAdapter
	llm      llm.LLMProvider
	tracker  tasktracker.TaskTracker
	base     string
	project  string
	fragment string
	events.EmitterMixin
}

// New constructs a review stage with the provided dependencies.
func New(cfg *config.Config, gitAdapter adapter.GitAdapter, provider llm.LLMProvider, tracker tasktracker.TaskTracker, base, project, fragment string) (*Stage, error) {
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

	return &Stage{
		name:     stagesreview.Describe(cfg),
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
	promptText := prompt.NewPromptAssembler(s.base, s.project, instance, s.fragment).Assemble()

	model, err := reviewModel(cfg)
	if err != nil {
		return nil, err
	}

	resp, err := s.llm.Invoke(ctx, llm.InvokeRequest{Prompt: promptText, Model: model})
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

	created, err := s.createBeads(ctx, parent.ID, classified.Beads)
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

func (s *Stage) createBeads(ctx context.Context, parentID string, proposals []*bead.Bead) ([]*tasktracker.Bead, error) {
	if len(proposals) == 0 {
		return nil, nil
	}
	deps := copyStrings([]string{parentID})
	if parentID == "" {
		deps = nil
	}
	created := make([]*tasktracker.Bead, 0, len(proposals))
	for _, proposal := range proposals {
		labels := copyStrings(proposal.Labels)
		trackerBead, err := s.tracker.CreateBead(ctx, proposal.Title, proposal.Description, proposal.Priority, labels, deps)
		if err != nil {
			return nil, fmt.Errorf("review: creating bead %q: %w", proposal.Title, err)
		}
		created = append(created, trackerBead)
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
