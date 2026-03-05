package v2

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/stage"
)

// RemediationRunnerConfig captures dependencies for remediation orchestration.
type RemediationRunnerConfig struct {
	AcceptStage    stage.Stage
	GapStage       stage.Stage
	DecomposeStage stage.Stage
	BeadRunner     BeadRunner
	GenerationCap  int
}

// BeadRunner executes the loop that processes each generated bead.
type BeadRunner interface {
	Run(ctx context.Context, beads []*bead.Bead) error
}

// RemediationRunner drives the accept-gap-decompose-bead loop cycle.
type RemediationRunner struct {
	cfg RemediationRunnerConfig
}

// NewRemediationRunner constructs a remediation runner using the provided config.
func NewRemediationRunner(cfg RemediationRunnerConfig) *RemediationRunner {
	return &RemediationRunner{cfg: cfg}
}

// DecomposeArtifacts holds artifacts emitted by the decompose stage.
type DecomposeArtifacts struct {
	Beads []*bead.Bead
}

// Run executes the remediation cycle for the provided spec.
func (r *RemediationRunner) Run(ctx context.Context, specID string) error {
	if specID == "" {
		return fmt.Errorf("spec ID required")
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
		return nil
	}
}

func (r *RemediationRunner) executeRemediation(ctx context.Context, req *stage.Request) error {
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

	artifacts, ok := res.Artifacts.(*DecomposeArtifacts)
	if !ok {
		return nil, fmt.Errorf("unexpected artifacts type from decompose stage")
	}

	return append([]*bead.Bead(nil), artifacts.Beads...), nil
}
