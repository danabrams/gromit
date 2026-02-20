package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// makeValidationExecuteFn creates a validation.ExecuteFn callback that wraps
// the escalation handler's ExecuteWithRetry for Claude-based validation fix attempts.
func (r *Runner) makeValidationExecuteFn() validation.ExecuteFn {
	return func(ctx context.Context, bc *runtypes.BeadContext) bool {
		if bc == nil || bc.Result == nil {
			return false
		}
		if bc.PromptCtx == nil {
			bc.PromptCtx = &prompt.Context{}
		}
		if bc.PromptCtx.Bead == nil {
			bc.PromptCtx.Bead = bc.Bead
		}

		// Render a build prompt for the validation fix
		bc.PromptCtx.IsRetry = true
		bc.PromptCtx.PrevFailure = runtypes.TruncateOutput(bc.Result.Output)
		bc.PromptCtx.FailureContext = "Validation (tests/lint) failed after your build succeeded. Fix the validation errors."

		var renderErr error
		bc.BuildPrompt, renderErr = r.renderer.RenderBuild(r.shapeMethodologyPromptContext("green", bc.PromptCtx))
		if renderErr != nil {
			r.log("Warning: rendering validation fix prompt: %v", renderErr)
			return false
		}

		// Save retry state and enforce bounded validation recovery attempts.
		savedMaxRetries := bc.MaxRetries
		savedRetriesThisModel := bc.RetriesThisModel
		savedTotalRetries := bc.TotalRetriesThisBead
		valPolicy := r.ensureValidationPolicy()
		maxAttempts := 0
		shouldEscalate := false
		if valPolicy != nil {
			maxAttempts = valPolicy.MaxRecoveryAttempts()
			shouldEscalate = valPolicy.ShouldEscalateRecovery()
		}
		bc.MaxRetries = 0
		bc.RetriesThisModel = 0

		runSingleAttempt := func() bool {
			// Disable escalation inside ExecuteWithRetry so each call is exactly one attempt.
			return r.escalationHandler.ExecuteWithRetryWithEscalation(ctx, bc, r.makeInvokeFn(), false)
		}

		success := false
		for attempt := 0; attempt < maxAttempts; attempt++ {
			if attempt > 0 && shouldEscalate {
				r.ensureEscalationPolicy()
				if nextTier := r.escalationPolicy.NextTier(bc.Tier); nextTier != "" {
					r.escalationHandler.EscalateTier(bc, nextTier)
				}
			}
			if runSingleAttempt() {
				success = true
				break
			}
		}

		// Restore retry state
		bc.MaxRetries = savedMaxRetries
		bc.RetriesThisModel = savedRetriesThisModel
		bc.TotalRetriesThisBead = savedTotalRetries

		return success
	}
}

// makeReviewValidateFn creates a ValidateFn that wraps runDirectValidationCheck
// for use by the reviewpkg.Reviewer's re-validation after review fixes.
func (r *Runner) makeReviewValidateFn() reviewpkg.ValidateFn {
	return func(ctx context.Context, commands []string, workDir string) (bool, error) {
		result, err := r.runDirectValidationCheck(ctx, commands, workDir)
		if err != nil {
			return false, err
		}
		return result != nil && claude.IsValidationPassed(result), nil
	}
}

func summarizeATDDProviderOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "no provider output"
	}
	const maxChars = 1400
	if len(trimmed) <= maxChars {
		return trimmed
	}
	const side = 650
	return trimmed[:side] + "...[truncated]..." + trimmed[len(trimmed)-side:]
}

func formatATDDProviderFailure(p provider.Provider, modelName string, result *provider.Result) string {
	if result == nil {
		return fmt.Sprintf("provider=%s model=%s nil result", p.Name(), modelName)
	}
	return fmt.Sprintf(
		"provider=%s model=%s exit_code=%d failure_category=%s stderr=%s output=%s diagnostics=%s",
		p.Name(),
		modelName,
		result.ExitCode,
		result.FailureCategory,
		summarizeATDDProviderOutput(result.Stderr),
		summarizeATDDProviderOutput(result.Output),
		summarizeATDDProviderOutput(result.Diagnostics),
	)
}
