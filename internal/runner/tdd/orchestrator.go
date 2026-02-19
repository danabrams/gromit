package tdd

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// RenderRedFn renders a red-phase prompt from a handoff and bead context.
type RenderRedFn func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error)

// RenderGreenFn renders a green-phase prompt from a handoff and bead context.
type RenderGreenFn func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error)

// InvokeFn invokes Claude with a prompt at a given tier.
type InvokeFn func(ctx context.Context, prompt, tier string) error

// ValidateFn runs validation commands and returns output, pass/fail, and error.
type ValidateFn func(ctx context.Context, commands []string, workDir string) (output string, passed bool, err error)

// RunRefactorFn runs the refactor phase.
type RunRefactorFn func(ctx context.Context, bc *runtypes.BeadContext) error

// EscalateTierFn returns the next escalation tier, or empty string if none available.
type EscalateTierFn func(currentTier string) string

// GetGitHeadFn returns the current git HEAD commit hash.
type GetGitHeadFn func() (string, error)

// GitResetFn resets the working tree to a given commit.
type GitResetFn func(commit string) error

// CycleOrchestrator runs TDD red-green-refactor cycles with fresh context per phase.
type CycleOrchestrator struct {
	renderRedFn   RenderRedFn
	renderGreenFn RenderGreenFn
	invokeFn      InvokeFn
	validateFn    ValidateFn
	runRefactorFn RunRefactorFn
	getDiffFn     GetDiffFn
	readFileFn    ReadFileFn
	cfg           *config.Config
}

// RunCycles executes TDD cycles until the state is complete.
func (o *CycleOrchestrator) RunCycles(ctx context.Context, bc *runtypes.BeadContext, state CycleState) error {
	for !state.IsComplete() {
		if err := o.runOneCycle(ctx, bc, &state); err != nil {
			return err
		}
	}
	return nil
}

func (o *CycleOrchestrator) runOneCycle(ctx context.Context, bc *runtypes.BeadContext, state *CycleState) error {
	// RED: assemble handoff -> render prompt -> invoke
	redHandoff, err := AssembleRedHandoff(*state, o.readFileFn, o.getDiffFn)
	if err != nil {
		return fmt.Errorf("red handoff assembly: %w", err)
	}

	redPrompt, err := o.renderRedFn(redHandoff, bc)
	if err != nil {
		return fmt.Errorf("red prompt render: %w", err)
	}

	err = o.invokeFn(ctx, redPrompt, bc.Tier)
	if err != nil {
		return fmt.Errorf("red invocation: %w", err)
	}

	// VALIDATE RED: expect tests to fail
	valOutput, passed, err := o.validateFn(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("red validation: %w", err)
	}
	if passed {
		// Tests pass unexpectedly — nothing left to implement
		state.Done = true
		*state = AssembleCycleState(*state, "")
		return nil
	}

	// GREEN: assemble handoff -> render prompt -> invoke
	greenHandoff, err := AssembleGreenHandoff(valOutput, o.readFileFn, state.TouchedFiles)
	if err != nil {
		return fmt.Errorf("green handoff assembly: %w", err)
	}

	greenPrompt, err := o.renderGreenFn(greenHandoff, bc)
	if err != nil {
		return fmt.Errorf("green prompt render: %w", err)
	}

	err = o.invokeFn(ctx, greenPrompt, bc.Tier)
	if err != nil {
		return fmt.Errorf("green invocation: %w", err)
	}

	// VALIDATE GREEN: expect tests to pass
	_, passed, err = o.validateFn(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("green validation: %w", err)
	}
	if !passed {
		return fmt.Errorf("green validation failed: tests still failing after green phase")
	}

	// REFACTOR: behavior-preserving cleanup (failure non-blocking)
	if o.runRefactorFn != nil {
		_ = o.runRefactorFn(ctx, bc)
	}

	// VALIDATE REFACTOR: verify tests still pass
	_, passed, err = o.validateFn(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("refactor validation: %w", err)
	}
	if !passed {
		// Refactor broke tests — this is handled in a later cycle (revert)
		return fmt.Errorf("refactor validation failed: tests broken after refactor")
	}

	// Advance state
	*state = AssembleCycleState(*state, "")
	return nil
}
