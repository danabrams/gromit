package remediation

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
	DecomposeStage stage.Stage
	BeadRunner     BeadRunner
	GenerationCap  int
	Presenter      adapter.PresenterAdapter
	Emitter        *events.Emitter
}

// BeadRunner executes the loop that processes each generated bead.
type BeadRunner interface {
	Run(ctx context.Context, beads []*bead.Bead) error
}

var (
	ErrSpecIDRequired               = errors.New("spec ID required")
	ErrBeadRunnerRequired           = errors.New("bead runner required")
	ErrDecomposeStageRequired       = errors.New("decompose stage required")
	ErrUnexpectedDecomposeArtifacts = errors.New("unexpected artifacts type from decompose stage")
)

const DefaultGenerationCap = 3

// RemediationRunner orchestrates remediation generations driven by findings.
type RemediationRunner struct {
	cfg             RemediationRunnerConfig
	generationCount int
}

// NewRemediationRunner constructs a remediation runner using the provided config.
func NewRemediationRunner(cfg RemediationRunnerConfig) *RemediationRunner {
	return &RemediationRunner{cfg: cfg}
}

// Run executes one remediation generation based on the provided findings.
func (r *RemediationRunner) Run(ctx context.Context, specID, worktree string, findings []stage.Finding) error {
	if specID == "" {
		return ErrSpecIDRequired
	}
	if !r.canRemediate() {
		return r.handleGenerationCap(ctx, specID)
	}

	req := stage.Request{
		Bead:        stage.BeadInfo{ID: specID},
		Worktree:    worktree,
		Remediation: true,
		Findings:    append([]stage.Finding{}, findings...),
	}

	return r.executeRemediation(ctx, &req)
}

func (r *RemediationRunner) executeRemediation(ctx context.Context, req *stage.Request) error {
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
