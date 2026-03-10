package remediation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
	"github.com/danabrams/gromit/internal/v2/stage"
)

// RemediationRunnerConfig captures dependencies for remediation orchestration.
type RemediationRunnerConfig struct {
	AcceptStage     stage.Stage
	PlanStage       stage.Stage
	GapStage        stage.Stage
	DecomposeStage  stage.Stage
	BeadRunner      BeadRunner
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

// gapSummaryProvider is implemented by artifacts that carry a gap analysis summary.
type gapSummaryProvider interface {
	GetGapSummary() string
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
	ErrAcceptStageRequired          = errors.New("accept stage required")
	ErrBeadRunnerRequired           = errors.New("bead runner required")
	ErrDecomposeStageRequired       = errors.New("decompose stage required")
	ErrUnexpectedDecomposeArtifacts = errors.New("unexpected artifacts type from decompose stage")
)

const DefaultGenerationCap = 3

// acceptResultsProvider is implemented by artifacts that carry acceptance results.
type acceptResultsProvider interface {
	GetAcceptanceResults() []presentation.AcceptanceResult
}

// RemediationRunner drives the accept-gap-decompose-bead loop cycle.
type RemediationRunner struct {
	cfg                 RemediationRunnerConfig
	generationCount     int
	completedBeadTitles []string
}

// NewRemediationRunner constructs a remediation runner using the provided config.
func NewRemediationRunner(cfg RemediationRunnerConfig) *RemediationRunner {
	return &RemediationRunner{cfg: cfg}
}

// SetCompletedBeadTitles records bead titles that have already been closed,
// so the decompose stage can avoid re-creating equivalent work.
func (r *RemediationRunner) SetCompletedBeadTitles(titles []string) {
	r.completedBeadTitles = append([]string(nil), titles...)
}

// Run executes the remediation cycle for the provided spec.
// When initialResult is non-nil, it is used as the first accept evaluation
// result, avoiding a redundant accept call when the caller already knows
// the current state failed.
func (r *RemediationRunner) Run(ctx context.Context, specID, worktree string, initialResult *stage.Result) error {
	if specID == "" {
		return ErrSpecIDRequired
	}
	if r.cfg.AcceptStage == nil {
		return ErrAcceptStageRequired
	}

	reqTemplate := stage.Request{Bead: stage.BeadInfo{ID: specID}, Worktree: worktree}
	first := true
	for {
		req := reqTemplate
		var res *stage.Result
		var err error
		if first && initialResult != nil {
			res = initialResult
			first = false
		} else {
			first = false
			res, err = r.cfg.AcceptStage.Run(ctx, &req)
			if err != nil {
				return err
			}
		}
		if res != nil && res.Decision == stage.DecisionFail {
			gapAnalysis := extractGapSummary(res)
			failedCriteria := extractFailedCriteria(res)
			if err := r.executeRemediation(ctx, &req, gapAnalysis, failedCriteria); err != nil {
				return err
			}
			continue
		}
		if err := r.cleanup(ctx, specID); err != nil {
			return err
		}
		return nil
	}
}

func (r *RemediationRunner) cleanup(ctx context.Context, specID string) error {
	if cleaner := r.cfg.WorktreeCleaner; cleaner != nil {
		return cleaner.Cleanup(ctx, specID)
	}
	return nil
}

func extractGapSummary(res *stage.Result) string {
	if res == nil || res.Artifacts == nil {
		return ""
	}
	if gp, ok := res.Artifacts.(gapSummaryProvider); ok {
		return gp.GetGapSummary()
	}
	return ""
}

func extractFailedCriteria(res *stage.Result) []string {
	if res == nil || res.Artifacts == nil {
		return nil
	}
	arp, ok := res.Artifacts.(acceptResultsProvider)
	if !ok {
		return nil
	}
	var failed []string
	for _, r := range arp.GetAcceptanceResults() {
		if strings.HasPrefix(r.Description, "FAIL:") {
			failed = append(failed, fmt.Sprintf("%s: %s", r.Title, r.Description))
		}
	}
	return failed
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

func (r *RemediationRunner) executeRemediation(ctx context.Context, req *stage.Request, gapAnalysis string, failedCriteria []string) error {
	specID := req.Bead.ID
	if !r.canRemediate() {
		return r.handleGenerationCap(ctx, specID)
	}

	req.Remediation = true
	req.GapAnalysis = gapAnalysis
	req.CompletedBeadTitles = append([]string(nil), r.completedBeadTitles...)
	req.FailedAcceptanceCriteria = append([]string(nil), failedCriteria...)

	if r.cfg.GapStage != nil {
		if _, err := r.cfg.GapStage.Run(ctx, req); err != nil {
			return err
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
			SpecID: specID,
			Reason: reason,
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
