package methodology

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
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
type RefactorInvokeFn func(ctx context.Context, prompt string, tier string) (*claude.Result, error)

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
// if refactoring fails - it logs a warning and continues.
func (e *Executor) RunRefactorPhase(ctx context.Context, bc *runtypes.BeadContext) error {
	_ = ctx
	_ = bc
	return fmt.Errorf("RunRefactorPhase not yet implemented")
}

// handleRefactorValidationFailure reverts the refactor changes and retries once.
// Returns nil (not an error) after handling - refactor failures are non-blocking.
func (e *Executor) handleRefactorValidationFailure(ctx context.Context, bc *runtypes.BeadContext, preRefactorCommit string, reason string) error {
	_ = ctx
	_ = bc
	_ = preRefactorCommit
	_ = reason
	return fmt.Errorf("handleRefactorValidationFailure not yet implemented")
}

// RunAcceptanceTestsWithRetry runs the acceptance test phase with retry and escalation logic.
func (e *Executor) RunAcceptanceTestsWithRetry(ctx context.Context, bc *runtypes.BeadContext) error {
	_ = ctx
	_ = bc
	return fmt.Errorf("RunAcceptanceTestsWithRetry not yet implemented")
}

// VerifyTestsFailWithRetry runs the verify-tests-fail phase with retry logic.
func (e *Executor) VerifyTestsFailWithRetry(ctx context.Context, bc *runtypes.BeadContext) error {
	_ = ctx
	_ = bc
	return fmt.Errorf("VerifyTestsFailWithRetry not yet implemented")
}
