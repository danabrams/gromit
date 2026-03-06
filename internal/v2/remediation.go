package v2

import (
	"context"
	"errors"
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
	"github.com/danabrams/gromit/internal/v2/stage"
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

var ErrSpecIDRequired = errors.New("spec ID required")

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
func (r *RemediationRunner) Run(ctx context.Context, specID string) error {
	if specID == "" {
		return ErrSpecIDRequired
	}
	if r.cfg.AcceptStage == nil {
		return fmt.Errorf("accept stage required")
	}

	reqTemplate := stage.Request{Bead: stage.BeadInfo{ID: specID}}
	for {
		req := reqTemplate
		res, err := r.cfg.AcceptStage.Run(ctx, &req)
		if err != nil {
			return err
		}
		if res != nil && res.Decision == stage.DecisionFail {
			if err := r.executeRemediation(ctx, &req); err != nil {
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

func (r *RemediationRunner) executeRemediation(ctx context.Context, req *stage.Request) error {
	specID := req.Bead.ID
	if !r.canRemediate() {
		return r.handleGenerationCap(ctx, specID)
	}

	if r.cfg.GapStage != nil {
		if _, err := r.cfg.GapStage.Run(ctx, req); err != nil {
			return err
		}
	}

	beads, err := r.decompose(ctx, req)
	if err != nil {
		return err
	}

	if r.cfg.BeadRunner == nil {
		return fmt.Errorf("bead runner required")
	}
	if err := r.cfg.BeadRunner.Run(ctx, beads); err != nil {
		return err
	}

	r.generationCount++

	return nil
}

func (r *RemediationRunner) canRemediate() bool {
	cap := r.cfg.GenerationCap
	if cap <= 0 {
		return false
	}
	return r.generationCount < cap
}

func (r *RemediationRunner) handleGenerationCap(ctx context.Context, specID string) error {
	reason := "generation cap reached"
	if emitter := r.cfg.Emitter; emitter != nil {
		emitter.Emit(&events.GenerationCapReachedEvent{
			SpecID:        specID,
			GenerationCap: r.cfg.GenerationCap,
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
		return nil, fmt.Errorf("decompose stage required")
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
		return nil, fmt.Errorf("unexpected artifacts type from decompose stage")
	}

	return append([]*bead.Bead(nil), artifacts.Beads...), nil
}
