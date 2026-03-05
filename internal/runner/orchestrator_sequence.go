package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/pipeline"
)

func (o *Orchestrator) RunSequence(
	ctx context.Context,
	beadIDs []string,
	maxIterations int,
	deadline time.Time,
	stopCh <-chan struct{},
) error {
	if len(beadIDs) == 0 {
		return o.Run(ctx, maxIterations, deadline, stopCh)
	}
	if o.cfg.GetBeadByID == nil {
		return fmt.Errorf("orchestrator: GetBeadByID is not configured")
	}

	index := 0
	getFromSequence := func(seqCtx context.Context) (*bead.Bead, error) {
		if index >= len(beadIDs) {
			return nil, nil
		}
		id := beadIDs[index]
		index++
		b, err := o.cfg.GetBeadByID(seqCtx, id)
		if err != nil {
			return nil, fmt.Errorf("resolving bead %s: %w", id, err)
		}
		if b == nil {
			return nil, fmt.Errorf("bead %s not found", id)
		}
		return b, nil
	}

	cloned := *o
	cloned.cfg.GetBead = getFromSequence
	return cloned.Run(ctx, maxIterations, deadline, stopCh)
}

func (o *Orchestrator) buildInput(b *bead.Bead, iteration int, deadline time.Time, validationFailures, touchedPackages []string, startCommit string) pipeline.Input {
	cfg := o.cfg.Config
	escalationEnabled := cfg != nil && cfg.Escalation.Enabled
	complexity := ""
	if cfg != nil {
		complexity = cfg.SelectTier(b.Priority, b.Labels)
	}
	return pipeline.Input{
		Bead:               b,
		Config:             cfg,
		Emitter:            o.emitter,
		Iteration:          iteration,
		Deadline:           deadline,
		ValidationFailures: validationFailures,
		StartCommit:        startCommit,
		EscalationEnabled:  escalationEnabled,
		TouchedPackages:    touchedPackages,
		ComplexityRouting: pipeline.ComplexityRouting{
			Complexity: complexity,
		},
	}
}

func (o *Orchestrator) runEpilogue(ctx context.Context, in pipeline.Input, buildSucceeded bool) pipeline.Output {
	if o.cfg.Epilogue == nil {
		return pipeline.Output{}
	}
	in.BuildSucceeded = buildSucceeded
	out, err := o.cfg.Epilogue.Run(ctx, in)
	if err != nil {
		o.logWarning("Warning: epilogue error for bead %s (iteration %d): %v", in.Bead.ID, in.Iteration, err)
	}
	return out
}

func (o *Orchestrator) cleanupAfterFailedIteration(ctx context.Context) {
	if o.cfg.GitCheckout == nil {
		return
	}
	// RevertAndReturnToBase runs three steps sequentially:
	//   1. git checkout -- .   (revert tracked changes)
	//   2. git clean -fd       (remove untracked files)
	//   3. git checkout <base> (switch to base branch)
	//
	// In session worktree mode, step 3 fails because the base branch (main)
	// is already checked out in the main worktree (git exit 128). However,
	// steps 1 and 2 complete before step 3 runs, so the worktree is cleaned
	// of the failed build's edits. The step-3 error is logged as a warning
	// and is benign — the next iteration will checkout the correct branch.
	if err := o.cfg.GitCheckout.RevertAndReturnToBase(ctx); err != nil {
		o.logWarning("Warning: worktree cleanup after failed iteration: %v", err)
	}
}
