package remediation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
	"github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/finding"
)

// RemediationRunnerConfig captures dependencies for remediation orchestration.
type RemediationRunnerConfig struct {
	PlanStage       stage.Stage
	GapStage        stage.Stage
	Findings        []finding.Finding
	DecomposeStage  stage.Stage
	BeadRunner      BeadRunner
	Config          *config.Config
	GenerationCap   int
	Presenter       adapter.PresenterAdapter
	Emitter         *events.Emitter
	WorktreeCleaner WorktreeCleaner
}

// BeadRunner executes the loop that processes each generated bead.
type BeadRunner interface {
	Run(ctx context.Context, beads []*bead.Bead) error
}

// WorktreeCleaner cleans up the spec worktree after a successful run.
type WorktreeCleaner interface {
	Cleanup(ctx context.Context, specID string) error
}

// planContentProvider is implemented by artifacts that carry generated plan text.
// The plan stage's PlanArtifacts satisfies this interface when a GetPlanContent
// method is added, allowing the remediation runner to persist the actual
// remediation plan rather than just the gap analysis.
type planContentProvider interface {
	GetPlanContent() string
}

// planContentProvider is implemented by artifacts that carry generated plan text.
// The plan stage's PlanArtifacts satisfies this interface when a GetPlanContent
// method is added, allowing the remediation runner to persist the actual
// remediation plan rather than just the gap analysis.
type planContentProvider interface {
	GetPlanContent() string
}

var (
	ErrSpecIDRequired               = errors.New("spec ID required")
	ErrBeadRunnerRequired           = errors.New("bead runner required")
	ErrDecomposeStageRequired       = errors.New("decompose stage required")
	ErrUnexpectedDecomposeArtifacts = errors.New("unexpected artifacts type from decompose stage")
)

const (
	DefaultGenerationCap = 3
	defaultGromitDir     = ".gromit"
	v2DirName            = "v2"
	gapAnalysisFilename  = "gap-analysis.md"
)

// RemediationRunner drives the accept-gap-decompose-bead loop cycle.
type RemediationRunner struct {
	cfg             RemediationRunnerConfig
	generationCount int
}

// NewRemediationRunner constructs a remediation runner using the provided config.
func NewRemediationRunner(cfg RemediationRunnerConfig) *RemediationRunner {
	return &RemediationRunner{cfg: cfg}
}

// Run executes the remediation cycle for the provided spec using the supplied findings.
func (r *RemediationRunner) Run(ctx context.Context, specID, worktree string, findings []stage.SpecFinding) error {
	r.generationCount = 0
	if specID == "" {
		return ErrSpecIDRequired
	}

	req := stage.Request{
		Bead:        stage.BeadInfo{ID: specID},
		Worktree:    worktree,
		SpecFindings: append([]stage.SpecFinding(nil), findings...),
	}

	gapAnalysis := r.resolveGapAnalysis(worktree, findings)

	if err := r.executeRemediation(ctx, &req, gapAnalysis); err != nil {
		return err
	}

	return nil
}

func (r *RemediationRunner) resolveGapAnalysis(worktree string, findings []stage.SpecFinding) string {
	if summary := formatFindingSummaries(findings); summary != "" {
		return summary
	}
	return r.readGapAnalysisFromDisk(worktree)
}

func formatFindingSummaries(findings []stage.SpecFinding) string {
	if len(findings) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, finding := range findings {
		text := strings.TrimSpace(finding.Description)
		if text == "" {
			text = strings.TrimSpace(finding.Title)
		}
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}
	return builder.String()
}

func (r *RemediationRunner) readGapAnalysisFromDisk(worktree string) string {
	path := r.gapAnalysisPath(worktree)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ""
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (r *RemediationRunner) gapAnalysisPath(worktree string) string {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return ""
	}
	gromitDir := defaultGromitDir
	if cfg := r.cfg.Config; cfg != nil {
		if dir := strings.TrimSpace(cfg.Paths.GromitDir); dir != "" {
			gromitDir = dir
		}
	}
	return filepath.Join(trimmed, gromitDir, v2DirName, gapAnalysisFilename)
}

func (r *RemediationRunner) cleanup(ctx context.Context, specID string) error {
	if cleaner := r.cfg.WorktreeCleaner; cleaner != nil {
		return cleaner.Cleanup(ctx, specID)
	}
	return nil
}

func extractPlanContent(res *stage.Result) string {
	if res == nil || res.Artifacts == nil {
		return ""
	}
	if pp, ok := res.Artifacts.(planContentProvider); ok {
		return pp.GetPlanContent()
	}
	return ""
}

func extractPlanContent(res *stage.Result) string {
	if res == nil || res.Artifacts == nil {
		return ""
	}
	if pp, ok := res.Artifacts.(planContentProvider); ok {
		return pp.GetPlanContent()
	}
	return ""
}

func (r *RemediationRunner) executeRemediation(ctx context.Context, req *stage.Request, gapAnalysis string) error {
	specID := req.Bead.ID
	if !r.canRemediate() {
		return r.handleGenerationCap(ctx, specID)
	}

	req.Remediation = true
	req.GapAnalysis = gapAnalysis
	req.Findings = cloneFindings(convertSpecFindings(req.SpecFindings))

	if r.cfg.GapStage != nil {
		if _, err := r.cfg.GapStage.Run(ctx, req); err != nil {
			return err
		}
	}
	skipPlan := len(req.Findings) > 0
	var planContent string
	if !skipPlan && r.cfg.PlanStage != nil {
		planRes, err := r.cfg.PlanStage.Run(ctx, req)
		if err != nil {
			return fmt.Errorf("remediation plan: %w", err)
		}
		planContent = extractPlanContent(planRes)
	}

	if req.Worktree != "" {
		// Persist the plan stage output when available; otherwise fall back to
		// the gap analysis so there is always a remediation record on disk.
		content := planContent
		if content == "" {
			content = gapAnalysis
		}
		planPath := r.remediationPlanPath(req.Worktree)
		if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
			return fmt.Errorf("create remediation plan dir: %w", err)
		}
		if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("persist remediation plan: %w", err)
		}
	}

	var planContent string
	if r.cfg.PlanStage != nil {
		planRes, err := r.cfg.PlanStage.Run(ctx, req)
		if err != nil {
			return fmt.Errorf("remediation plan: %w", err)
		}
		planContent = extractPlanContent(planRes)
	}

	if req.Worktree != "" {
		// Persist the plan stage output when available; otherwise fall back to
		// the gap analysis so there is always a remediation record on disk.
		content := planContent
		if content == "" {
			content = gapAnalysis
		}
		planPath := r.remediationPlanPath(req.Worktree)
		if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
			return fmt.Errorf("create remediation plan dir: %w", err)
		}
		if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("persist remediation plan: %w", err)
		}
	}

	beads, err := r.decompose(ctx, req)
	if err != nil {
		return err
	}

	if r.cfg.BeadRunner == nil {
		return ErrBeadRunnerRequired
	}
	if err := r.cfg.BeadRunner.Run(ctx, beads); err != nil {
		return err
	}

	r.generationCount++

	return nil
}

func (r *RemediationRunner) remediationPlanPath(worktree string) string {
	gromitDir := filepath.Join(worktree, ".gromit", "v2")
	return filepath.Join(gromitDir, fmt.Sprintf("remediation-%d.md", r.generationCount+1))
}

func (r *RemediationRunner) generationCap() int {
	if cap := r.cfg.GenerationCap; cap >= 0 {
		return cap
	}
	return DefaultGenerationCap
}

func (r *RemediationRunner) canRemediate() bool {
	return r.generationCount < r.generationCap()
}

func (r *RemediationRunner) handleGenerationCap(ctx context.Context, specID string) error {
	cap := r.generationCap()
	reason := "generation cap reached"
	if emitter := r.cfg.Emitter; emitter != nil {
		emitter.Emit(&events.GenerationCapReachedEvent{
			SpecID:        specID,
			GenerationCap: cap,
		})
		emitter.Emit(&events.AndonTriggeredEvent{
			SpecID:       specID,
			Reason:       reason,
			FindingCount: len(r.cfg.Findings),
		})
	}
	if err := r.presentFailureSummary(ctx, specID, reason); err != nil {
		return err
	}
	return errors.New(reason)
}

func (r *RemediationRunner) presentFailureSummary(ctx context.Context, specID, reason string) error {
	if presenter := r.cfg.Presenter; presenter != nil {
		summary := presentation.PresentationSummary{
			SpecName:          specID,
			SpecBranch:        presentation.SpecBranchName(specID),
			IntegrationBranch: presentation.DefaultIntegrationBranch(),
			Success:           false,
			FailureSummary:    fmt.Sprintf("spec %s remediation halted: %s", specID, reason),
			RemainingWork:     []string{},
		}
		return presenter.PresentSummary(ctx, specID, summary)
	}
	return nil
}

func cloneFindings(src []finding.Finding) []finding.Finding {
	if len(src) == 0 {
		return nil
	}
	clones := make([]finding.Finding, len(src))
	for i, f := range src {
		clones[i] = f
		clones[i].AffectedFiles = append([]string(nil), f.AffectedFiles...)
	}
	return clones
}

func convertSpecFindings(src []stage.SpecFinding) []finding.Finding {
	if len(src) == 0 {
		return nil
	}
	converted := make([]finding.Finding, 0, len(src))
	for _, spec := range src {
		converted = append(converted, finding.Finding{
			Severity:    mapSpecSeverity(spec.Severity),
			Category:    mapSpecCategory(spec.Category),
			Scope:       strings.TrimSpace(string(spec.Scope)),
			Description: strings.TrimSpace(spec.Description),
		})
	}
	return converted
}

func mapSpecSeverity(severity stage.SpecFindingSeverity) finding.Severity {
	switch severity {
	case stage.SpecFindingSeverityCritical:
		return finding.SeverityCritical
	case stage.SpecFindingSeverityHigh, stage.SpecFindingSeverityMedium:
		return finding.SeverityWarning
	default:
		return finding.SeveritySuggestion
	}
}

func mapSpecCategory(category stage.SpecFindingCategory) finding.Category {
	switch category {
	case stage.SpecFindingCategoryAcceptance:
		return finding.CategoryAcceptance
	case stage.SpecFindingCategoryScope:
		return finding.CategoryArchitecture
	case stage.SpecFindingCategoryQuality:
		return finding.CategoryQuality
	case stage.SpecFindingCategorySafety:
		return finding.CategorySecurity
	default:
		return finding.CategoryQuality
	}
}

func (r *RemediationRunner) decompose(ctx context.Context, req *stage.Request) ([]*bead.Bead, error) {
	if r.cfg.DecomposeStage == nil {
		return nil, ErrDecomposeStageRequired
	}

	res, err := r.cfg.DecomposeStage.Run(ctx, req)
	if err != nil {
		return nil, err
	}

	if res == nil || res.Artifacts == nil {
		return nil, fmt.Errorf("decompose stage returned no artifacts")
	}

	artifacts, ok := res.Artifacts.(*stage.DecomposeArtifacts)
	if !ok {
		return nil, ErrUnexpectedDecomposeArtifacts
	}

	return append([]*bead.Bead(nil), artifacts.Beads...), nil
}
