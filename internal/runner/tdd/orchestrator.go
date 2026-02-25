package tdd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

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

// ListChangedFilesFn returns the list of files changed since a given commit.
type ListChangedFilesFn func(sinceCommit string) ([]string, error)

// RestoreTestFilesFn restores test files to their pre-invocation state.
// This prevents the green phase from modifying test files written in the red phase.
type RestoreTestFilesFn func(testFiles []string) error

// LogPhaseFn logs TDD phase transitions for observability.
type LogPhaseFn func(cycle int, phase string, detail string)

// RecordPhaseMetricFn records per-invocation phase metrics with snapshot-based deltas.
type RecordPhaseMetricFn func(bc *runtypes.BeadContext, phase string, cycleNumber int, beforeCostUSD float64, beforeInputTokens int, beforeOutputTokens int, startTime time.Time)

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
	renderRedFn          RenderRedFn
	renderGreenFn        RenderGreenFn
	invokeFn             InvokeFn
	validateFn           ValidateFn
	runRefactorFn        RunRefactorFn
	escalateTierFn       EscalateTierFn
	getDiffFn            GetDiffFn
	readFileFn           ReadFileFn
	getGitHeadFn         GetGitHeadFn
	gitResetFn           GitResetFn
	listChangedFilesFn   ListChangedFilesFn
	restoreTestFilesFn   RestoreTestFilesFn
	logPhaseFn           LogPhaseFn
	recordPhaseMetricFn  RecordPhaseMetricFn
	output               io.Writer
	cfg                  *config.Config
}

// CycleOrchestratorDeps holds callback dependencies for building a CycleOrchestrator.
type CycleOrchestratorDeps struct {
	RenderRedFn          RenderRedFn
	RenderGreenFn        RenderGreenFn
	InvokeFn             InvokeFn
	ValidateFn           ValidateFn
	RunRefactorFn        RunRefactorFn
	EscalateTierFn       EscalateTierFn
	GetDiffFn            GetDiffFn
	ReadFileFn           ReadFileFn
	GetGitHeadFn         GetGitHeadFn
	GitResetFn           GitResetFn
	ListChangedFilesFn   ListChangedFilesFn
	RestoreTestFilesFn   RestoreTestFilesFn
	LogPhaseFn           LogPhaseFn
	RecordPhaseMetricFn  RecordPhaseMetricFn
}

// NewCycleOrchestrator creates a TDD cycle orchestrator with injected callbacks.
func NewCycleOrchestrator(cfg *config.Config, output io.Writer, deps CycleOrchestratorDeps) *CycleOrchestrator {
	return &CycleOrchestrator{
		renderRedFn:          deps.RenderRedFn,
		renderGreenFn:        deps.RenderGreenFn,
		invokeFn:             deps.InvokeFn,
		validateFn:           deps.ValidateFn,
		runRefactorFn:        deps.RunRefactorFn,
		escalateTierFn:       deps.EscalateTierFn,
		getDiffFn:            deps.GetDiffFn,
		readFileFn:           deps.ReadFileFn,
		getGitHeadFn:         deps.GetGitHeadFn,
		gitResetFn:           deps.GitResetFn,
		listChangedFilesFn:   deps.ListChangedFilesFn,
		restoreTestFilesFn:   deps.RestoreTestFilesFn,
		logPhaseFn:           deps.LogPhaseFn,
		recordPhaseMetricFn:  deps.RecordPhaseMetricFn,
		output:               output,
		cfg:                  cfg,
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
	if o.invokeFn == nil {
		return fmt.Errorf("invokeFn is not configured")
	}
	// First attempt
	err := o.invokeFn(ctx, prompt, *tier)
	if err == nil {
		return nil
	}

	// Retry once on same tier
	o.logPhase(0, "retry", fmt.Sprintf("retrying on tier %s", *tier))
	err = o.invokeFn(ctx, prompt, *tier)
	if err == nil {
		return nil
	}

	// Escalate tier
	if o.escalateTierFn != nil {
		nextTier := o.escalateTierFn(*tier)
		if nextTier != "" {
			o.logPhase(0, "escalate", fmt.Sprintf("escalating to tier %s", nextTier))
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

func (o *CycleOrchestrator) logPhase(cycle int, phase string, detail string) {
	if o.logPhaseFn != nil {
		o.logPhaseFn(cycle, phase, detail)
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
	o.logPhase(state.CycleNumber+1, "red", "writing failing test")

	// RED: assemble handoff -> render prompt -> invoke
	redHandoff, err := AssembleRedHandoff(*state, o.readFileFn, o.getDiffFn)
	if err != nil {
		return fmt.Errorf("red handoff assembly: %w", err)
	}

	redPrompt, err := o.renderRedFn(redHandoff, bc)
	if err != nil {
		return fmt.Errorf("red prompt render: %w", err)
	}

	// Take snapshot before red phase invocation
	beforeRedCostUSD, beforeRedInputTokens, beforeRedOutputTokens := runtypes.SnapshotIterationUsage(bc.Result)
	redStartTime := time.Now()

	if err := o.runPhaseInvocation(ctx, bc, redPrompt, "red"); err != nil {
		return err
	}

	// Record red phase metrics with snapshot-based deltas
	o.recordPhaseMetric(bc, "red", state.CycleNumber+1, beforeRedCostUSD, beforeRedInputTokens, beforeRedOutputTokens, redStartTime)

	// Discover files touched by the red phase so the green handoff includes them.
	o.updateTouchedFiles(state)

	// VALIDATE RED: expect tests to fail
	if o.validateFn == nil {
		return fmt.Errorf("validateFn is not configured")
	}
	redValidationOutput, passed, err := o.validateFn(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("red validation: %w", err)
	}
	if passed {
		// Tests pass — criterion is already implemented.
		// Advance to next remaining criterion instead of stopping.
		// AssembleCycleState sets Done=true when Remaining is empty.
		o.logPhase(state.CycleNumber+1, "red-skip", "criterion already passing, advancing")
		*state = AssembleCycleState(*state, "")
		return nil
	}

	o.logf("cycle %d: green phase — implementing to pass", state.CycleNumber+1)
	o.logPhase(state.CycleNumber+1, "green", "implementing to pass")

	// GREEN: assemble handoff -> render prompt -> invoke
	greenHandoff, err := AssembleGreenHandoff(redValidationOutput, o.readFileFn, state.TouchedFiles)
	if err != nil {
		return fmt.Errorf("green handoff assembly: %w", err)
	}

	greenPrompt, err := o.renderGreenFn(greenHandoff, bc)
	if err != nil {
		return fmt.Errorf("green prompt render: %w", err)
	}

	// Take snapshot before green phase invocation
	beforeGreenCostUSD, beforeGreenInputTokens, beforeGreenOutputTokens := runtypes.SnapshotIterationUsage(bc.Result)
	greenStartTime := time.Now()

	if err := o.runGreenPhaseUntilValidated(ctx, bc, state.CycleNumber+1, greenPrompt, state.TouchedFiles); err != nil {
		return err
	}

	// Record green phase metrics with snapshot-based deltas
	o.recordPhaseMetric(bc, "green", state.CycleNumber+1, beforeGreenCostUSD, beforeGreenInputTokens, beforeGreenOutputTokens, greenStartTime)

	o.logPhase(state.CycleNumber+1, "green-pass", "tests passing")

	if err := o.runRefactorAndFinalValidation(ctx, bc, state.CycleNumber+1); err != nil {
		return err
	}

	// Advance state
	*state = AssembleCycleState(*state, "")
	return nil
}

func (o *CycleOrchestrator) runGreenPhaseUntilValidated(
	ctx context.Context,
	bc *runtypes.BeadContext,
	cycleNumber int,
	greenPrompt string,
	touchedFiles []string,
) error {
	testFiles, _ := ClassifyTouchedFiles(touchedFiles)

	if err := o.runPhaseInvocation(ctx, bc, greenPrompt, "green"); err != nil {
		return err
	}
	o.restoreTestFiles(testFiles)

	for {
		// VALIDATE GREEN: expect tests to pass
		if o.validateFn == nil {
			return fmt.Errorf("validateFn is not configured")
		}
		validationOutput, passed, err := o.validateFn(ctx, nil, "")
		if err != nil {
			return fmt.Errorf("green validation: %w", err)
		}
		if passed {
			return nil
		}

		if o.escalateTierFn == nil {
			return fmt.Errorf("green validation failed: tests still failing after green phase")
		}
		nextTier := o.escalateTierFn(bc.Tier)
		if nextTier == "" || nextTier == bc.Tier {
			return fmt.Errorf("green validation failed: tests still failing after green phase")
		}

		bc.Tier = nextTier
		o.logPhase(cycleNumber, "green-retry", fmt.Sprintf("green validation failed, retrying at tier %s", nextTier))

		// Re-assemble the green handoff with updated implementation files
		// and validation failure context so the retry has full context.
		retryPrompt := greenPrompt
		if o.renderGreenFn != nil && o.readFileFn != nil {
			retryHandoff, assembleErr := AssembleGreenHandoff(validationOutput, o.readFileFn, touchedFiles)
			if assembleErr == nil {
				if rendered, renderErr := o.renderGreenFn(retryHandoff, bc); renderErr == nil {
					retryPrompt = rendered
				}
			}
		}

		// Restore the escalated tier: renderGreenFn's PhaseModelTier() side
		// effect resets bc.Tier to the default phase tier, which would cause
		// escalateTierFn to return the same "next" tier infinitely.
		bc.Tier = nextTier

		if err := o.runPhaseInvocation(ctx, bc, retryPrompt, "green"); err != nil {
			return err
		}
		o.restoreTestFiles(testFiles)
	}
}

// restoreTestFiles reverts any unauthorized modifications to test files
// made by the green phase model, which should only modify implementation files.
func (o *CycleOrchestrator) restoreTestFiles(testFiles []string) {
	if o.restoreTestFilesFn == nil || len(testFiles) == 0 {
		return
	}
	if err := o.restoreTestFilesFn(testFiles); err != nil {
		fmt.Fprintf(os.Stderr, "[tdd] warning: failed to restore test files: %v\n", err)
	}
}

func (o *CycleOrchestrator) runRefactorAndFinalValidation(ctx context.Context, bc *runtypes.BeadContext, cycleNumber int) error {
	o.logPhase(cycleNumber, "refactor", "refactoring")

	// Take snapshot before refactor phase invocation
	beforeRefactorCostUSD, beforeRefactorInputTokens, beforeRefactorOutputTokens := runtypes.SnapshotIterationUsage(bc.Result)
	refactorStartTime := time.Now()

	outcome := o.executeRefactorPhase(ctx, bc)
	if !isSupportedRefactorOutcome(outcome) {
		return fmt.Errorf("refactor phase: unsupported outcome %q", outcome)
	}
	if outcome == refactorOutcomeFailed {
		return fmt.Errorf("refactor phase failed")
	}

	// Record refactor phase metrics with snapshot-based deltas
	o.recordPhaseMetric(bc, "refactor", cycleNumber, beforeRefactorCostUSD, beforeRefactorInputTokens, beforeRefactorOutputTokens, refactorStartTime)

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

// SetInvokeFn replaces the orchestrator's invoke callback. This allows
// callers to inject a telemetry-accumulating wrapper once the BeadContext
// is available (which is after orchestrator construction).
func (o *CycleOrchestrator) SetInvokeFn(fn InvokeFn) {
	o.invokeFn = fn
}

// updateTouchedFiles merges newly changed files into state.TouchedFiles.
// Uses listChangedFilesFn when available, falling back to getGitHeadFn +
// getDiffFn-based parsing for backwards compatibility.
func (o *CycleOrchestrator) updateTouchedFiles(state *CycleState) {
	if o.listChangedFilesFn == nil {
		return
	}
	// List files changed since the parent of the current commit (i.e. the
	// commit before the phase invocation).
	files, err := o.listChangedFilesFn("HEAD~1")
	if err != nil {
		return
	}
	existing := make(map[string]bool, len(state.TouchedFiles))
	for _, f := range state.TouchedFiles {
		existing[f] = true
	}
	for _, f := range files {
		if !existing[f] {
			state.TouchedFiles = append(state.TouchedFiles, f)
			existing[f] = true
		}
	}
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


// recordPhaseMetric records a phase metric with snapshot-based deltas.
func (o *CycleOrchestrator) recordPhaseMetric(bc *runtypes.BeadContext, phase string, cycleNumber int, beforeCostUSD float64, beforeInputTokens int, beforeOutputTokens int, startTime time.Time) {
	if o.recordPhaseMetricFn != nil {
		o.recordPhaseMetricFn(bc, phase, cycleNumber, beforeCostUSD, beforeInputTokens, beforeOutputTokens, startTime)
	}
}
