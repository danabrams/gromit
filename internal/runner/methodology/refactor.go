package methodology

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// ErrATDDAlreadyDone is returned by VerifyTestsFailWithRetry when acceptance
// tests pass before implementation after retry. This signals that the work is
// already done (e.g., a sibling bead completed it), not that the tests are bad.
var ErrATDDAlreadyDone = errors.New("atdd: acceptance tests pass — work already done")

// IsATDDAlreadyDone returns true if the error is the ATDD already-done sentinel.
func IsATDDAlreadyDone(err error) bool {
	return errors.Is(err, ErrATDDAlreadyDone)
}

// RefactorInvokeFn executes a refactor Claude invocation with a prompt and tier.
type RefactorInvokeFn func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error)

// RenderRefactorFn renders the refactor prompt from a prompt context.
type RenderRefactorFn func(ctx *prompt.Context) (string, error)

// GetDiffFn returns the git diff from a start commit.
type GetDiffFn func(startCommit string) (string, error)

// GitResetFn resets git to a given commit.
type GitResetFn func(commit string) error

// GetGitHeadFn returns the current git HEAD commit hash.
type GetGitHeadFn func() (string, error)

// EscalateTierFn escalates a bead context to a new tier.
type EscalateTierFn func(bc *runtypes.BeadContext, nextTier string)

// AnalyzeFn analyzes a failure and returns a suggestion.
type AnalyzeFn func(ctx context.Context, b *bead.Bead, failureOutput string) (string, error)

// RefactorDeps holds the dependencies needed for refactor phase operations.
type RefactorDeps struct {
	getDiffFn        GetDiffFn
	renderRefactorFn RenderRefactorFn
	refactorInvokeFn RefactorInvokeFn
	validateFn       ValidateDirectFn
	gitResetFn       GitResetFn
	getGitHeadFn     GetGitHeadFn
}

// NewRefactorDeps creates a RefactorDeps with the given callbacks.
func NewRefactorDeps(
	getDiffFn GetDiffFn,
	renderRefactorFn RenderRefactorFn,
	refactorInvokeFn RefactorInvokeFn,
	validateFn ValidateDirectFn,
	gitResetFn GitResetFn,
	getGitHeadFn GetGitHeadFn,
) RefactorDeps {
	return RefactorDeps{
		getDiffFn:        getDiffFn,
		renderRefactorFn: renderRefactorFn,
		refactorInvokeFn: refactorInvokeFn,
		validateFn:       validateFn,
		gitResetFn:       gitResetFn,
		getGitHeadFn:     getGitHeadFn,
	}
}

// NewExecutorWithRefactor creates an Executor configured for refactor phase operations.
func NewExecutorWithRefactor(cfg *config.Config, output io.Writer, deps RefactorDeps) *Executor {
	return &Executor{
		cfg:              cfg,
		output:           output,
		validateFn:       deps.validateFn,
		getDiffFn:        deps.getDiffFn,
		renderRefactorFn: deps.renderRefactorFn,
		refactorInvokeFn: deps.refactorInvokeFn,
		gitResetFn:       deps.gitResetFn,
		getGitHeadFn:     deps.getGitHeadFn,
	}
}

// NewExecutorWithEscalation creates an Executor with escalation support for retry wrappers.
func NewExecutorWithEscalation(cfg *config.Config, output io.Writer, renderFn RenderFn, invokeFn InvokeFn, validateFn ValidateDirectFn, escalateTierFn EscalateTierFn) *Executor {
	return &Executor{
		cfg:            cfg,
		output:         output,
		renderFn:       renderFn,
		invokeFn:       invokeFn,
		validateFn:     validateFn,
		escalateTierFn: escalateTierFn,
	}
}

// NewExecutorWithAnalysis creates an Executor with analysis support for VerifyTestsFailWithRetry.
func NewExecutorWithAnalysis(cfg *config.Config, output io.Writer, renderFn RenderFn, invokeFn InvokeFn, validateFn ValidateDirectFn, analyzeFn AnalyzeFn, getDiffFn GetDiffFn) *Executor {
	return &Executor{
		cfg:        cfg,
		output:     output,
		renderFn:   renderFn,
		invokeFn:   invokeFn,
		validateFn: validateFn,
		analyzeFn:  analyzeFn,
		getDiffFn:  getDiffFn,
	}
}

// ShouldRunRefactor determines whether the refactor phase should run based on
// bead complexity tier and number of files changed.
func (e *Executor) ShouldRunRefactor(bc *runtypes.BeadContext, diff string) bool {
	// Skip refactor for haiku-tier beads
	if bc.Tier == provider.TierLow {
		e.log("Skipping refactor: haiku-tier bead")
		return false
	}

	// Check file count threshold
	minFiles := e.cfg.Refactor.MinFilesChanged
	if minFiles == 0 {
		// Threshold of 0 means always run refactor (no file count check)
		return true
	}

	filesChanged := len(ParseDiffFiles(diff))
	if filesChanged < minFiles {
		e.log("Skipping refactor: only %d files changed (threshold: %d)", filesChanged, minFiles)
		return false
	}

	return true
}

// RunRefactorPhase runs the refactoring phase after validation passes.
// Returns nil on success or if refactoring is skipped. Does not return an error
// if refactoring fails - it logs a warning and continues (working code without
// refactoring is better than broken code).
func (e *Executor) RunRefactorPhase(ctx context.Context, bc *runtypes.BeadContext) error {
	if e.getDiffFn == nil {
		e.log("Warning: getDiffFn not configured, skipping refactor")
		return nil
	}

	// Check if there are any changes to refactor
	diff, err := e.getDiffFn(bc.StartCommit)
	if err != nil {
		e.log("Warning: could not get git diff: %v", err)
		return nil
	}
	if diff == "" {
		e.log("No changes to refactor, skipping refactor phase")
		return nil
	}

	// Check if refactor should run based on complexity and file count
	if !e.ShouldRunRefactor(bc, diff) {
		return nil
	}

	// Capture pre-refactor commit for potential revert
	if e.getGitHeadFn == nil {
		e.log("Warning: getGitHeadFn not configured, skipping refactor")
		return nil
	}
	preRefactorCommit, err := e.getGitHeadFn()
	if err != nil {
		e.log("Warning: could not capture pre-refactor commit: %v", err)
		return nil
	}

	// Render refactor prompt
	if e.renderRefactorFn == nil {
		e.log("Warning: renderRefactorFn not configured, skipping refactor")
		return nil
	}
	refactorPrompt, err := e.renderRefactorFn(bc.PromptCtx)
	if err != nil {
		e.log("Warning: could not render refactor prompt: %v", err)
		return nil
	}

	// Execute refactor invocation
	if e.refactorInvokeFn == nil {
		e.log("Warning: refactorInvokeFn not configured, skipping refactor")
		return nil
	}
	refactorResult, refactorStats, err := e.refactorInvokeFn(ctx, refactorPrompt, bc.Tier)
	if err != nil {
		e.log("Warning: refactor invocation failed: %v", err)
		return nil
	}
	if refactorResult == nil || !refactorResult.Success {
		e.log("Warning: refactor phase failed")
		return nil
	}
	e.applyRefactorStreamStats(bc, refactorStats)

	e.log("Refactor phase complete, re-validating...")

	// Re-validate after refactoring
	if !e.cfg.Validation.Enabled {
		e.log("Validation not enabled, cannot verify refactoring")
		return nil
	}

	if e.validateFn == nil {
		e.log("Warning: validateFn not configured, cannot re-validate after refactor")
		return nil
	}

	validationCommands := e.refactorValidationCommands(bc)
	valResult, err := e.validateFn(ctx, validationCommands, bc.PromptCtx.WorkDir)
	if err != nil {
		e.log("Warning: refactor re-validation invocation failed: %v", err)
		return e.handleRefactorValidationFailure(ctx, bc, preRefactorCommit, "re-validation invocation failed")
	}

	if valResult == nil || !claude.IsValidationPassed(valResult) {
		return e.handleRefactorValidationFailure(ctx, bc, preRefactorCommit, "tests failed after refactoring")
	}

	e.log("Refactor re-validation passed")
	return nil
}

func (e *Executor) applyRefactorStreamStats(bc *runtypes.BeadContext, stats *logger.StreamStats) {
	if bc == nil || bc.Result == nil || stats == nil {
		return
	}

	costUSD, inputTokens, outputTokens := stats.CostData()
	bc.Result.CostUSD += costUSD
	bc.Result.InputTokens += inputTokens
	bc.Result.OutputTokens += outputTokens
	bc.CumulativeInputTokens += inputTokens
}

func (e *Executor) refactorValidationCommands(bc *runtypes.BeadContext) []string {
	commands := e.cfg.Validation.FastCommandsOrDefault()
	if bc == nil {
		return commands
	}

	e.updateTouchedPackagesFromDiff(bc)
	return config.ScopeGoTestCommands(commands, bc.TouchedPackages)
}

func (e *Executor) updateTouchedPackagesFromDiff(bc *runtypes.BeadContext) bool {
	if e.getDiffFn == nil || bc == nil {
		return false
	}

	diff, err := e.getDiffFn(bc.StartCommit)
	if err != nil {
		return false
	}

	if touched := DetectTouchedPackages(diff); len(touched) > 0 {
		bc.TouchedPackages = touched
		return true
	}

	return false
}

// handleRefactorValidationFailure reverts the refactor changes and retries once.
// Returns nil (not an error) after handling - refactor failures are non-blocking.
func (e *Executor) handleRefactorValidationFailure(ctx context.Context, bc *runtypes.BeadContext, preRefactorCommit string, reason string) error {
	e.log("Refactor validation failed: %s", reason)
	e.log("Reverting to pre-refactor state: %s", preRefactorCommit)

	// Revert to pre-refactor commit
	if e.gitResetFn != nil {
		if err := e.gitResetFn(preRefactorCommit); err != nil {
			e.log("Warning: could not revert refactor changes: %v", err)
			return nil
		}
	}

	e.log("Reverted to pre-refactor state, retrying refactor once...")

	// Retry refactor with analysis context
	bc.PromptCtx.IsRetry = true
	bc.PromptCtx.FailureContext = fmt.Sprintf("Previous refactoring broke tests: %s. Be more conservative this time.", reason)

	if e.renderRefactorFn == nil {
		return nil
	}
	refactorPrompt, err := e.renderRefactorFn(bc.PromptCtx)
	if err != nil {
		e.log("Warning: could not render retry refactor prompt: %v", err)
		return nil
	}

	// Execute retry refactor
	if e.refactorInvokeFn == nil {
		return nil
	}
	retryResult, retryStats, err := e.refactorInvokeFn(ctx, refactorPrompt, bc.Tier)
	if err != nil {
		e.log("Warning: retry refactor invocation failed: %v - skipping refactoring", err)
		return nil
	}
	if retryResult == nil || !retryResult.Success {
		e.log("Warning: retry refactor failed - skipping refactoring")
		return nil
	}
	e.applyRefactorStreamStats(bc, retryStats)

	e.log("Retry refactor complete, re-validating...")

	if e.validateFn == nil {
		return nil
	}
	validationCommands := e.refactorValidationCommands(bc)
	valResult, err := e.validateFn(ctx, validationCommands, bc.PromptCtx.WorkDir)

	if err != nil || valResult == nil || !claude.IsValidationPassed(valResult) {
		e.log("Warning: retry refactor also failed validation - skipping refactoring")
		// Revert again
		if e.gitResetFn != nil {
			if err := e.gitResetFn(preRefactorCommit); err != nil {
				e.log("Warning: could not revert retry refactor changes: %v", err)
			}
		}
		return nil
	}

	e.log("Retry refactor re-validation passed")
	return nil
}

// RunAcceptanceTestsWithRetry runs the acceptance test phase with retry and escalation logic.
// Returns nil on success or error on failure.
func (e *Executor) RunAcceptanceTestsWithRetry(ctx context.Context, bc *runtypes.BeadContext) error {
	retries := 0
	maxRetries := e.cfg.Escalation.MaxRetriesPerModel
	currentTier := bc.Tier
	visitedTiers := map[string]struct{}{currentTier: {}}

	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("acceptance tests phase aborted: %w", err)
		}

		if retries > 0 {
			e.log("Retrying acceptance tests (attempt %d/%d)...", retries+1, maxRetries+1)
		}
		e.log("ATDD acceptance attempt context: tier=%s retry_index=%d", currentTier, retries)

		err := e.RunAcceptanceTests(ctx, bc)
		if err == nil {
			return nil
		}
		e.log("ATDD acceptance attempt failed: %v", err)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("acceptance tests phase aborted: %w", err)
		}

		// Retry with same tier
		if retries < maxRetries {
			retries++
			continue
		}

		// Escalate tier
		nextTier := e.cfg.NextEscalationTier(currentTier)
		if nextTier == "" {
			return fmt.Errorf("acceptance tests failed with all tiers: %w", err)
		}

		if _, visited := visitedTiers[nextTier]; visited {
			return fmt.Errorf("acceptance tests failed with all tiers: %w", err)
		}

		e.log("Escalating acceptance tests from tier %s to %s", currentTier, nextTier)
		if e.escalateTierFn != nil {
			e.escalateTierFn(bc, nextTier)
		}
		currentTier = nextTier
		visitedTiers[currentTier] = struct{}{}
		retries = 0
	}
}

// VerifyTestsFailWithRetry runs the verify-tests-fail phase with retry logic.
// If tests pass (unexpected), retries once with analysis, then fails.
func (e *Executor) VerifyTestsFailWithRetry(ctx context.Context, bc *runtypes.BeadContext) error {
	err := e.VerifyTestsFail(ctx, bc)
	if err == nil {
		return nil // Tests failed as expected
	}

	// Tests passed before implementation - this is unexpected
	// Run failure analysis to understand why
	e.log("Unexpected: tests passed before implementation. Analyzing...")

	if e.analyzeFn != nil {
		suggestion, analyzeErr := e.analyzeFn(ctx, bc.Bead, err.Error())
		if analyzeErr != nil {
			e.log("Warning: failure analysis failed: %v — treating as already done", analyzeErr)
			return ErrATDDAlreadyDone
		}

		// Retry acceptance tests once with analysis context
		e.log("Retrying acceptance tests with analysis context...")
		bc.PromptCtx.IsRetry = true
		bc.PromptCtx.FailureContext = suggestion

		if retryErr := e.RunAcceptanceTests(ctx, bc); retryErr != nil {
			e.log("ATDD retry with analysis failed: %v", retryErr)
			return fmt.Errorf("acceptance tests retry failed: %w", retryErr)
		}

		// Verify tests fail again
		err = e.VerifyTestsFail(ctx, bc)
		if err == nil {
			return nil // Tests now fail as expected
		}
	}

	// Still passing after retry with analysis — check if this is a false positive
	// by examining the git diff. If only test files changed (no implementation),
	// the tests are likely checking existing behavior rather than new behavior.
	if bc.StartCommit != "" && e.getDiffFn != nil {
		diff, diffErr := e.getDiffFn(bc.StartCommit)
		if diffErr == nil && IsTestOnlyDiff(diff) {
			e.log("Tests pass but only test files changed — likely testing existing behavior, retrying...")
			bc.PromptCtx.IsRetry = true
			bc.PromptCtx.FailureContext = "Tests pass but no implementation code was changed — tests are likely checking existing behavior. Rewrite tests to assert on behavior that does not exist yet."

			if retryErr := e.RunAcceptanceTests(ctx, bc); retryErr == nil {
				// Verify tests fail again after diff-aware retry
				if err2 := e.VerifyTestsFail(ctx, bc); err2 == nil {
					return nil // Tests now fail as expected
				}
			} else {
				e.log("ATDD diff-aware retry failed: %v", retryErr)
			}
			// If retry failed or tests still pass, fall through to ErrATDDAlreadyDone
		}
	}

	e.log("Acceptance tests pass after retry — work appears already done")
	return ErrATDDAlreadyDone
}
