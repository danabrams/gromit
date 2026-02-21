package methodology

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// ErrATDDAlreadyDone is returned when acceptance tests pass before implementation
// and diagnostic analysis classifies the bead as already done.
var ErrATDDAlreadyDone = errors.New("atdd: acceptance tests pass — work already done")

// IsATDDAlreadyDone returns true if the error is the ATDD already-done sentinel.
func IsATDDAlreadyDone(err error) bool {
	return errors.Is(err, ErrATDDAlreadyDone)
}

const (
	diagnosticVerdictAlreadyDone = "ALREADY_DONE"
	diagnosticVerdictRewrite     = "REWRITE"
	diagnosticVerdictPrefix      = "VERDICT:"
	rewriteErrorMessage          = "atdd: acceptance tests need rewrite"
)

// ErrATDDRewrite is returned when diagnostic analysis says acceptance tests
// should be rewritten instead of treating the bead as already done.
type ErrATDDRewrite struct {
	Feedback string
}

// Error implements the error interface.
func (e *ErrATDDRewrite) Error() string {
	if e == nil {
		return rewriteErrorMessage
	}
	feedback := strings.TrimSpace(e.Feedback)
	if feedback == "" {
		return rewriteErrorMessage
	}
	return fmt.Sprintf("%s: %s", rewriteErrorMessage, feedback)
}

// AsATDDRewrite extracts an ErrATDDRewrite from err, if present.
func AsATDDRewrite(err error) (*ErrATDDRewrite, bool) {
	var rewriteErr *ErrATDDRewrite
	if !errors.As(err, &rewriteErr) {
		return nil, false
	}
	return rewriteErr, true
}

func parseDiagnosticVerdict(output string) (verdict string, feedback string) {
	lines := strings.Split(output, "\n")
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, diagnosticVerdictPrefix) {
			continue
		}

		marker := strings.TrimSpace(strings.TrimPrefix(trimmed, diagnosticVerdictPrefix))
		switch marker {
		case diagnosticVerdictAlreadyDone:
			return diagnosticVerdictAlreadyDone, ""
		case diagnosticVerdictRewrite:
			remainder := strings.TrimSpace(strings.Join(lines[idx+1:], "\n"))
			return diagnosticVerdictRewrite, remainder
		}
	}

	return diagnosticVerdictAlreadyDone, ""
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

// RefactorDeps holds the dependencies needed for refactor phase operations.
type RefactorDeps struct {
	getDiffFn        GetDiffFn
	renderRefactorFn RenderRefactorFn
	refactorInvokeFn RefactorInvokeFn
	validateFn       ValidateDirectFn
	gitResetFn       GitResetFn
	getGitHeadFn     GetGitHeadFn
}

// RefactorPhaseResult reports whether the refactor phase completed successfully.
// Skipped outcomes are considered successful phase completion.
type RefactorPhaseResult struct {
	Successful bool
	Attempted  bool
	Skipped    bool
	Reason     string
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

// ShouldRunRefactor determines whether the refactor phase should run based on
// bead complexity tier and number of files changed.
func (e *Executor) ShouldRunRefactor(bc *runtypes.BeadContext, diff string) bool {
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
	_ = e.RunRefactorPhaseWithResult(ctx, bc)
	return nil
}

// RunRefactorPhaseWithResult runs the refactoring phase and returns whether it
// completed successfully.
func (e *Executor) RunRefactorPhaseWithResult(ctx context.Context, bc *runtypes.BeadContext) RefactorPhaseResult {
	if e.getDiffFn == nil {
		e.log("Warning: getDiffFn not configured, skipping refactor")
		return RefactorPhaseResult{Successful: true, Skipped: true, Reason: "diff_not_configured"}
	}

	// Check if there are any changes to refactor
	diff, err := e.getDiffFn(bc.StartCommit)
	if err != nil {
		e.log("Warning: could not get git diff: %v", err)
		return RefactorPhaseResult{Successful: true, Skipped: true, Reason: "diff_unavailable"}
	}
	if diff == "" {
		e.log("No changes to refactor, skipping refactor phase")
		return RefactorPhaseResult{Successful: true, Skipped: true, Reason: "no_changes"}
	}

	// Check if refactor should run based on complexity and file count
	if !e.ShouldRunRefactor(bc, diff) {
		return RefactorPhaseResult{Successful: true, Skipped: true, Reason: "policy_skipped"}
	}

	// Capture pre-refactor commit for potential revert
	if e.getGitHeadFn == nil {
		e.log("Warning: getGitHeadFn not configured, skipping refactor")
		return RefactorPhaseResult{Successful: true, Skipped: true, Reason: "git_head_not_configured"}
	}
	preRefactorCommit, err := e.getGitHeadFn()
	if err != nil {
		e.log("Warning: could not capture pre-refactor commit: %v", err)
		return RefactorPhaseResult{Successful: true, Skipped: true, Reason: "git_head_unavailable"}
	}

	// Render refactor prompt
	if e.renderRefactorFn == nil {
		e.log("Warning: renderRefactorFn not configured, skipping refactor")
		return RefactorPhaseResult{Successful: true, Skipped: true, Reason: "render_not_configured"}
	}
	refactorPrompt, err := e.renderRefactorFn(bc.PromptCtx)
	if err != nil {
		e.log("Warning: could not render refactor prompt: %v", err)
		return RefactorPhaseResult{Successful: true, Skipped: true, Reason: "render_failed"}
	}

	// Execute refactor invocation
	if e.refactorInvokeFn == nil {
		e.log("Warning: refactorInvokeFn not configured, skipping refactor")
		return RefactorPhaseResult{Successful: true, Skipped: true, Reason: "invoke_not_configured"}
	}
	refactorResult, refactorStats, err := e.refactorInvokeFn(ctx, refactorPrompt, bc.Tier)
	if err != nil {
		e.log("Warning: refactor invocation failed: %v", err)
		return RefactorPhaseResult{Successful: false, Attempted: true, Reason: "invoke_failed"}
	}
	if refactorResult == nil || !refactorResult.Success {
		e.log("Warning: refactor phase failed")
		return RefactorPhaseResult{Successful: false, Attempted: true, Reason: "invoke_unsuccessful"}
	}
	e.applyRefactorStreamStats(bc, refactorStats)

	e.log("Refactor phase complete, re-validating...")

	// Re-validate after refactoring
	if !e.cfg.Validation.Enabled {
		e.log("Validation not enabled, cannot verify refactoring")
		return RefactorPhaseResult{Successful: true, Attempted: true, Reason: "validation_disabled"}
	}

	if e.validateFn == nil {
		e.log("Warning: validateFn not configured, cannot re-validate after refactor")
		return RefactorPhaseResult{Successful: false, Attempted: true, Reason: "validation_not_configured"}
	}

	validationCommands := e.refactorValidationCommands(bc)
	valResult, err := e.validateFn(ctx, validationCommands, bc.PromptCtx.WorkDir)
	if err != nil {
		e.log("Warning: refactor re-validation invocation failed: %v", err)
		if e.handleRefactorValidationFailure(ctx, bc, preRefactorCommit, "re-validation invocation failed") {
			return RefactorPhaseResult{Successful: true, Attempted: true, Reason: "retry_succeeded"}
		}
		return RefactorPhaseResult{Successful: false, Attempted: true, Reason: "revalidation_invocation_failed"}
	}

	if valResult == nil || !claude.IsValidationPassed(valResult) {
		if e.handleRefactorValidationFailure(ctx, bc, preRefactorCommit, "tests failed after refactoring") {
			return RefactorPhaseResult{Successful: true, Attempted: true, Reason: "retry_succeeded"}
		}
		return RefactorPhaseResult{Successful: false, Attempted: true, Reason: "revalidation_failed"}
	}

	e.log("Refactor re-validation passed")
	return RefactorPhaseResult{Successful: true, Attempted: true, Reason: "passed"}
}

func (e *Executor) applyRefactorStreamStats(bc *runtypes.BeadContext, stats *logger.StreamStats) {
	if bc == nil || bc.Result == nil || stats == nil {
		return
	}

	costUSD, refactorInputTokens, outputTokens := stats.CostData()
	bc.Result.CostUSD += costUSD
	bc.Result.InputTokens += refactorInputTokens
	bc.Result.OutputTokens += outputTokens
	bc.CumulativeInputTokens += refactorInputTokens
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
// Returns true when the retry path succeeds.
func (e *Executor) handleRefactorValidationFailure(ctx context.Context, bc *runtypes.BeadContext, preRefactorCommit string, reason string) bool {
	e.log("Refactor validation failed: %s", reason)
	e.log("Reverting to pre-refactor state: %s", preRefactorCommit)

	// Revert to pre-refactor commit
	if e.gitResetFn != nil {
		if err := e.gitResetFn(preRefactorCommit); err != nil {
			e.log("Warning: could not revert refactor changes: %v", err)
			return false
		}
	}

	e.log("Reverted to pre-refactor state, retrying refactor once...")

	// Retry refactor with analysis context
	bc.PromptCtx.IsRetry = true
	bc.PromptCtx.FailureContext = fmt.Sprintf("Previous refactoring broke tests: %s. Be more conservative this time.", reason)

	if e.renderRefactorFn == nil {
		return false
	}
	refactorPrompt, err := e.renderRefactorFn(bc.PromptCtx)
	if err != nil {
		e.log("Warning: could not render retry refactor prompt: %v", err)
		return false
	}

	// Execute retry refactor
	if e.refactorInvokeFn == nil {
		return false
	}
	retryResult, retryStats, err := e.refactorInvokeFn(ctx, refactorPrompt, bc.Tier)
	if err != nil {
		e.log("Warning: retry refactor invocation failed: %v - skipping refactoring", err)
		return false
	}
	if retryResult == nil || !retryResult.Success {
		e.log("Warning: retry refactor failed - skipping refactoring")
		return false
	}
	e.applyRefactorStreamStats(bc, retryStats)

	e.log("Retry refactor complete, re-validating...")

	if e.validateFn == nil {
		return false
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
		return false
	}

	e.log("Retry refactor re-validation passed")
	return true
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
