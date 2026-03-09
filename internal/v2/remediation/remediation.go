package remediation

import (
	"context"
	"errors"
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/findings"
	"github.com/danabrams/gromit/internal/v2/presentation"
	"github.com/danabrams/gromit/internal/v2/stage"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
)

// RemediationRunnerConfig captures dependencies for remediation orchestration.
type RemediationRunnerConfig struct {
	AcceptStage     stage.Stage
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

type findingsProvider interface {
	GetFindings() []findings.Finding
}

var (
	ErrSpecIDRequired               = errors.New("spec ID required")
	ErrAcceptStageRequired          = errors.New("accept stage required")
	ErrBeadRunnerRequired           = errors.New("bead runner required")
	ErrDecomposeStageRequired       = errors.New("decompose stage required")
	ErrUnexpectedDecomposeArtifacts = errors.New("unexpected artifacts type from decompose stage")
)

const DefaultGenerationCap = 3

// RemediationRunner drives the accept-gap-decompose-bead loop cycle.
type RemediationRunner struct {
	cfg             RemediationRunnerConfig
	generationCount int
}

// NewRemediationRunner constructs a remediation runner using the provided config.
func NewRemediationRunner(cfg RemediationRunnerConfig) *RemediationRunner {
	return &RemediationRunner{cfg: cfg}
}

// Run executes the remediation cycle for the provided spec.
func (r *RemediationRunner) Run(ctx context.Context, specID, worktree string) error {
	r.generationCount = 0
	if specID == "" {
		return ErrSpecIDRequired
	}
	if r.cfg.AcceptStage == nil {
		return ErrAcceptStageRequired
	}

	reqTemplate := stage.Request{Bead: stage.BeadInfo{ID: specID}, Worktree: worktree}
	for {
		req := reqTemplate
		res, err := r.cfg.AcceptStage.Run(ctx, &req)
		if err != nil {
			return err
		}
		if res != nil && res.Decision == stage.DecisionFail {
			findings := extractFindings(res)
			req.GapAnalysis = extractGapSummary(res)
			if err := r.executeRemediation(ctx, &req, findings); err != nil {
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

func extractFindings(res *stage.Result) []findings.Finding {
	if res == nil || res.Artifacts == nil {
		return nil
	}
	if fp, ok := res.Artifacts.(findingsProvider); ok {
		return copyFindings(fp.GetFindings())
	}
	if aa, ok := res.Artifacts.(*stageaccept.AcceptArtifacts); ok {
		return copyFindings(aa.Findings)
	}
	return nil
}

func copyFindings(src []findings.Finding) []findings.Finding {
	if len(src) == 0 {
		return nil
	}
	dst := make([]findings.Finding, len(src))
	copy(dst, src)
	return dst
}

func (r *RemediationRunner) executeRemediation(ctx context.Context, req *stage.Request, findings []findings.Finding) error {
	specID := req.Bead.ID
	if !r.canRemediate() {
		return r.handleGenerationCap(ctx, specID)
	}

	req.Remediation = true
	req.Findings = copyFindings(findings)

	if len(req.Findings) == 0 && r.cfg.GapStage != nil {
		if _, err := r.cfg.GapStage.Run(ctx, req); err != nil {
			return err
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
