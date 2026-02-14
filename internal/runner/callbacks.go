package runner

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
	"github.com/danabrams/gromit/internal/usagelimit"
)

// makeInvokeFn creates an InvokeFn callback that wraps the Runner's Claude invocation,
// handling cost data, scope-too-large, usage limit detection, and timeout classification.
func (r *Runner) makeInvokeFn() escalation.InvokeFn {
	return func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*escalation.InvocationResult, error) {
		// Re-render build prompt on retry so failure context is included
		if bc.PromptCtx != nil && bc.PromptCtx.IsRetry && r.renderer != nil {
			rendered, renderErr := r.renderer.RenderBuild(bc.PromptCtx)
			if renderErr == nil {
				bc.BuildPrompt = rendered
			}
		}

		r.log("Running Claude with model: %s", bc.Model)

		claudeResult, stats, stallFired, err := r.executeClaudeInvocation(ctx, bc)

		if err != nil {
			if stallFired && ctx.Err() == nil {
				return &escalation.InvocationResult{StallFired: true}, err
			}
			// Classify timeout type
			if ctx.Err() != nil && bc.ParentCtx.Err() == nil {
				bc.Result.TimeoutType = "bead"
				escalation.ExtractTimeoutLearning(bc, r.renderer.GetLearningsFile())
				return nil, fmt.Errorf("bead timeout: exceeded %v total processing time", bc.BeadTimeout)
			} else if bc.ParentCtx.Err() != nil {
				return nil, fmt.Errorf("context cancelled: %w", bc.ParentCtx.Err())
			}
			bc.Result.TimeoutType = "invocation"
			return nil, fmt.Errorf("claude invocation: %w", err)
		}

		if claudeResult == nil {
			return nil, fmt.Errorf("claude returned nil result")
		}

		// Populate cost/token data
		if stats != nil {
			costUSD, inputTokens, outputTokens := stats.CostData()
			bc.Result.CostUSD = costUSD
			bc.Result.InputTokens = inputTokens
			bc.Result.OutputTokens = outputTokens
		}

		// Check scope-too-large
		if isTooLarge, explanation := claude.IsScopeTooLarge(claudeResult); isTooLarge {
			r.handleScopeTooLarge(bc, claudeResult, explanation)
			return &escalation.InvocationResult{Result: claudeResult}, bc.Result.Error
		}

		// Check usage limits
		signals := usagelimit.Signals{
			ExitCode: claudeResult.ExitCode,
			Output:   claudeResult.Output,
		}
		if stats != nil {
			signals.RateLimitHits = stats.RateLimitHits
		}
		if usagelimit.Check(signals, usagelimit.ClaudePatterns()) {
			bc.Result.UsageLimited = true
			r.log("Warning: usage limit detected - stopping retry attempts")
			return nil, fmt.Errorf("usage limit detected: retries or escalation will not resolve this failure (exit code: %d, rate limit events: %d)", claudeResult.ExitCode, signals.RateLimitHits)
		}

		return &escalation.InvocationResult{
			Result:     claudeResult,
			StallFired: false,
		}, nil
	}
}

// makeValidationExecuteFn creates a validation.ExecuteFn callback that wraps
// the escalation handler's ExecuteWithRetry for Claude-based validation fix attempts.
func (r *Runner) makeValidationExecuteFn() validation.ExecuteFn {
	return func(ctx context.Context, bc *runtypes.BeadContext) bool {
		// Render a build prompt for the validation fix
		bc.PromptCtx.IsRetry = true
		bc.PromptCtx.PrevFailure = bc.Result.Output
		bc.PromptCtx.FailureContext = "Validation (tests/lint) failed after your build succeeded. Fix the validation errors."

		var renderErr error
		bc.BuildPrompt, renderErr = r.renderer.RenderBuild(bc.PromptCtx)
		if renderErr != nil {
			r.log("Warning: rendering validation fix prompt: %v", renderErr)
			return false
		}

		// Save retry state and enforce single attempt
		savedMaxRetries := bc.MaxRetries
		savedRetriesThisModel := bc.RetriesThisModel
		savedTotalRetries := bc.TotalRetriesThisBead
		bc.MaxRetries = 0
		bc.RetriesThisModel = 0

		success := r.escalationHandler.ExecuteWithRetry(ctx, bc, r.makeInvokeFn())

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

// makeMethodologyExec creates a methodology.Executor wired with callbacks
// that route through the Runner's provider, escalation, and validation infrastructure.
func (r *Runner) makeMethodologyExec() *methodology.Executor {
	// RenderFn wraps the renderer's acceptance tests prompt rendering
	renderFn := func(ctx *prompt.Context) (string, error) {
		if r.renderer == nil {
			return "", fmt.Errorf("renderer not configured")
		}
		return r.renderer.RenderAcceptanceTests(ctx)
	}

	// InvokeFn wraps the provider chain for acceptance test invocations
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error {
		if r.router == nil {
			return fmt.Errorf("router not configured")
		}
		p, modelName := r.router.Select("build", bc.Tier)
		if p == nil {
			return fmt.Errorf("no providers available for phase=build tier=%s", bc.Tier)
		}
		bc.Model = modelName
		if bc.Result.Escalated && bc.Result.EscalatedTo != "" {
			bc.Result.EscalatedTo = modelName
		}
		result, err := p.Run(ctx, promptText, bc.Tier)
		if err != nil {
			if p.IsUsageLimitError(result, err) {
				r.router.MarkUnavailable(p.Name())
				p2, modelName2 := r.router.Select("build", bc.Tier)
				if p2 != nil {
					bc.Model = modelName2
					result, err = p2.Run(ctx, promptText, bc.Tier)
				}
			}
			if err != nil {
				return err
			}
		}
		if result == nil || !result.Success {
			return fmt.Errorf("acceptance tests failed")
		}
		return nil
	}

	// ValidateDirectFn wraps the validation runner's direct validation
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return r.runDirectValidationCheck(ctx, commands, workDir)
	}

	// EscalateTierFn wraps escalation.Handler.EscalateTier
	escalateTierFn := func(bc *runtypes.BeadContext, nextTier string) {
		if r.escalationHandler != nil {
			r.escalationHandler.EscalateTier(bc, nextTier)
		}
	}

	// AnalyzeFn wraps the failure analyzer, extracting the suggestion string
	var analyzeFn methodology.AnalyzeFn
	if r.analyzer != nil {
		analyzeFn = func(ctx context.Context, b *bead.Bead, failureOutput string) (string, error) {
			analysis, err := r.analyzer.Analyze(ctx, b, failureOutput)
			if err != nil {
				return "", err
			}
			if analysis == nil {
				return "", fmt.Errorf("analysis returned nil")
			}
			return analysis.Suggestion, nil
		}
	}

	// GetDiffFn wraps the runner's git diff
	getDiffFn := func(startCommit string) (string, error) {
		return r.getDiff(startCommit)
	}

	// Create the base executor with ATDD callbacks
	methExec := methodology.NewExecutorWithEscalation(r.cfg, r.output, renderFn, invokeFn, validateFn, escalateTierFn)

	// Wire analysis support for VerifyTestsFailWithRetry
	methExec.SetAnalyzeFn(analyzeFn)
	methExec.SetGetDiffFn(getDiffFn)

	// Wire refactor deps
	methExec.SetRefactorDeps(methodology.NewRefactorDeps(
		getDiffFn,
		func(ctx *prompt.Context) (string, error) {
			if r.renderer == nil {
				return "", fmt.Errorf("renderer not configured")
			}
			return r.renderer.RenderRefactor(ctx)
		},
		r.runRefactorWithRouter,
		validateFn,
		func(commit string) error {
			resetCmd := exec.Command("git", "reset", "--hard", commit)
			return resetCmd.Run()
		},
		getGitHead,
	))

	return methExec
}

var _ provider.Provider = nil
var _ execution.Provider = nil
