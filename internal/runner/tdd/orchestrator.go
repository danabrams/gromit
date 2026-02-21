package tdd

import (
	"context"
	"fmt"
	"io"

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

type refactorOutcome string

const (
	refactorOutcomeContinue         refactorOutcome = "continue"
	refactorOutcomeRevertedContinue refactorOutcome = "reverted_continue"
	refactorOutcomeFailed           refactorOutcome = "failed"
)

func isSupportedRefactorOutcome(outcome refactorOutcome) bool {
	switch outcome {
	case refactorOutcomeContinue, refactorOutcomeRevertedContinue, refactorOutcomeFailed:
		return true
	default:
		return false
	}
}

// CycleOrchestrator runs TDD red-green-refactor cycles with fresh context per phase.
type CycleOrchestrator struct {
	renderRedFn    RenderRedFn
	renderGreenFn  RenderGreenFn
	invokeFn       InvokeFn
	validateFn     ValidateFn
	runRefactorFn  RunRefactorFn
	escalateTierFn EscalateTierFn
	getDiffFn      GetDiffFn
	readFileFn     ReadFileFn
	getGitHeadFn   GetGitHeadFn
	gitResetFn     GitResetFn
	output         io.Writer
	cfg            *config.Config
}

// CycleOrchestratorDeps holds callback dependencies for building a CycleOrchestrator.
type CycleOrchestratorDeps struct {
	RenderRedFn    RenderRedFn
	RenderGreenFn  RenderGreenFn
	InvokeFn       InvokeFn
	ValidateFn     ValidateFn
	RunRefactorFn  RunRefactorFn
	EscalateTierFn EscalateTierFn
	GetDiffFn      GetDiffFn
	ReadFileFn     ReadFileFn
	GetGitHeadFn   GetGitHeadFn
	GitResetFn     GitResetFn
}

// NewCycleOrchestrator creates a TDD cycle orchestrator with injected callbacks.
func NewCycleOrchestrator(cfg *config.Config, output io.Writer, deps CycleOrchestratorDeps) *CycleOrchestrator {
	return &CycleOrchestrator{
		renderRedFn:    deps.RenderRedFn,
		renderGreenFn:  deps.RenderGreenFn,
		invokeFn:       deps.InvokeFn,
		validateFn:     deps.ValidateFn,
		runRefactorFn:  deps.RunRefactorFn,
		escalateTierFn: deps.EscalateTierFn,
		getDiffFn:      deps.GetDiffFn,
		readFileFn:     deps.ReadFileFn,
		getGitHeadFn:   deps.GetGitHeadFn,
		gitResetFn:     deps.GitResetFn,
		output:         output,
		cfg:            cfg,
	}
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

// invokeWithRetryAndEscalation attempts invocation, retries once on failure,
// then escalates tier. Returns error if all attempts fail.
func (o *CycleOrchestrator) invokeWithRetryAndEscalation(ctx context.Context, prompt string, tier *string) error {
	// First attempt
	err := o.invokeFn(ctx, prompt, *tier)
	if err == nil {
		return nil
	}

	// Retry once on same tier
	err = o.invokeFn(ctx, prompt, *tier)
	if err == nil {
		return nil
	}

	// Escalate tier
	if o.escalateTierFn != nil {
		nextTier := o.escalateTierFn(*tier)
		if nextTier != "" {
			*tier = nextTier
			return o.invokeFn(ctx, prompt, *tier)
		}
	}

	return fmt.Errorf("invocation failed after retry and escalation: %w", err)
}

func (o *CycleOrchestrator) logf(format string, args ...interface{}) {
	if o.output != nil {
		_, _ = fmt.Fprintf(o.output, format+"\n", args...)
	}
}

func (o *CycleOrchestrator) runPhaseInvocation(
	ctx context.Context,
	bc *runtypes.BeadContext,
	prompt string,
	phaseName string,
) error {
	tier := bc.Tier
	if err := o.invokeWithRetryAndEscalation(ctx, prompt, &tier); err != nil {
		return fmt.Errorf("%s phase: %w", phaseName, err)
	}
	bc.Tier = tier
	return nil
}

func (o *CycleOrchestrator) runOneCycle(ctx context.Context, bc *runtypes.BeadContext, state *CycleState) error {
	o.logf("cycle %d: red phase — writing failing test", state.CycleNumber+1)

	// RED: assemble handoff -> render prompt -> invoke
	redHandoff, err := AssembleRedHandoff(*state, o.readFileFn, o.getDiffFn)
	if err != nil {
		return fmt.Errorf("red handoff assembly: %w", err)
	}

	redPrompt, err := o.renderRedFn(redHandoff, bc)
	if err != nil {
		return fmt.Errorf("red prompt render: %w", err)
	}

	if err := o.runPhaseInvocation(ctx, bc, redPrompt, "red"); err != nil {
		return err
	}

	// VALIDATE RED: expect tests to fail
	redValidationOutput, passed, err := o.validateFn(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("red validation: %w", err)
	}
	if passed {
		// Tests pass unexpectedly — nothing left to implement.
		// Skip refactor: no implementation was written in this cycle.
		if err := o.runFinalValidation(ctx); err != nil {
			return err
		}

		state.Done = true
		*state = AssembleCycleState(*state, "")
		return nil
	}

	o.logf("cycle %d: green phase — implementing to pass", state.CycleNumber+1)

	// GREEN: assemble handoff -> render prompt -> invoke
	greenHandoff, err := AssembleGreenHandoff(redValidationOutput, o.readFileFn, state.TouchedFiles)
	if err != nil {
		return fmt.Errorf("green handoff assembly: %w", err)
	}

	greenPrompt, err := o.renderGreenFn(greenHandoff, bc)
	if err != nil {
		return fmt.Errorf("green prompt render: %w", err)
	}

	if err := o.runPhaseInvocation(ctx, bc, greenPrompt, "green"); err != nil {
		return err
	}

	// VALIDATE GREEN: expect tests to pass
	_, passed, err = o.validateFn(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("green validation: %w", err)
	}
	if !passed {
		return fmt.Errorf("green validation failed: tests still failing after green phase")
	}

	if err := o.runRefactorAndFinalValidation(ctx, bc); err != nil {
		return err
	}

	// Advance state
	*state = AssembleCycleState(*state, "")
	return nil
}

func (o *CycleOrchestrator) runRefactorAndFinalValidation(ctx context.Context, bc *runtypes.BeadContext) error {
	outcome := o.executeRefactorPhase(ctx, bc)
	if !isSupportedRefactorOutcome(outcome) {
		return fmt.Errorf("refactor phase: unsupported outcome %q", outcome)
	}
	if outcome == refactorOutcomeFailed {
		return fmt.Errorf("refactor phase failed")
	}
	return o.runFinalValidation(ctx)
}

func (o *CycleOrchestrator) executeRefactorPhase(ctx context.Context, bc *runtypes.BeadContext) refactorOutcome {
	if o.runRefactorFn == nil {
		return refactorOutcomeContinue
	}

	// Capture pre-refactor state for revert.
	preRefactorCommit := ""
	if o.getGitHeadFn != nil {
		commit, _ := o.getGitHeadFn()
		preRefactorCommit = commit
	}

	if err := o.runRefactorFn(ctx, bc); err != nil {
		return refactorOutcomeFailed
	}

	// Verify tests still pass after refactor.
	_, passed, validateErr := o.validateFn(ctx, nil, "")
	if validateErr != nil {
		return refactorOutcomeContinue
	}
	if passed {
		return refactorOutcomeContinue
	}

	// Refactor broke tests — revert and continue.
	if preRefactorCommit == "" || o.gitResetFn == nil {
		return refactorOutcomeContinue
	}

	_ = o.gitResetFn(preRefactorCommit)
	return refactorOutcomeRevertedContinue
}

func (o *CycleOrchestrator) runFinalValidation(ctx context.Context) error {
	// Final validation must run immediately after refactor in the same cycle.
	_, finalPassed, finalErr := o.validateFn(ctx, nil, "")
	if finalErr != nil {
		return fmt.Errorf("final validation: %w", finalErr)
	}
	if !finalPassed {
		return fmt.Errorf("final validation failed: tests failing after refactor phase")
	}
	return nil
}
