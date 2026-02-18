package runner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

type redPhasePromptShaper interface {
	ShapeRedPhaseContext(ctx *prompt.Context) *prompt.Context
}

type greenPhasePromptShaper interface {
	ShapeGreenPhaseContext(ctx *prompt.Context) *prompt.Context
}

type refactorPhasePromptShaper interface {
	ShapeRefactorPhaseContext(ctx *prompt.Context) *prompt.Context
}

// makeInvokeFn creates an InvokeFn callback that wraps the Runner's Claude invocation,
// handling cost data, scope-too-large, usage limit detection, and timeout classification.
func (r *Runner) makeInvokeFn() escalation.InvokeFn {
	return func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		prevScopedTestCmd := ""
		if bc.PromptCtx != nil {
			prevScopedTestCmd = bc.PromptCtx.ScopedTestCommand
		}
		if bc.StartCommit != "" {
			if diff, err := r.getDiff(bc.StartCommit); err == nil && diff != "" {
				bc.TouchedPackages = detectTouchedPackages(diff)
			}
		}
		injectScopedTestCommand(bc)

		// Re-render build prompt when retry context changes or when the scoped
		// self-check command changes from git diff detection.
		if bc.PromptCtx != nil && r.renderer != nil && (bc.PromptCtx.IsRetry || bc.PromptCtx.ScopedTestCommand != prevScopedTestCmd) {
			rendered, renderErr := r.renderer.RenderBuild(r.shapeMethodologyPromptContext("green", bc.PromptCtx))
			if renderErr == nil {
				bc.BuildPrompt = rendered
			}
		}

		r.log("Running Claude with model: %s", bc.Model)

		invResult, err := r.executeClaudeInvocation(ctx, bc)

		if err != nil {
			return r.handleInvokeError(ctx, bc, invResult, err)
		}

		if invResult == nil || invResult.Result == nil {
			return nil, fmt.Errorf("claude returned nil result")
		}

		// Populate cost/token data
		if invResult.Stats != nil {
			costUSD, inputTokens, outputTokens := invResult.Stats.CostData()
			bc.Result.CostUSD = costUSD
			bc.Result.InputTokens = inputTokens
			bc.Result.OutputTokens = outputTokens
		}

		// Check scope-too-large
		if isTooLarge, explanation := claude.IsScopeTooLarge(invResult.Result); isTooLarge {
			r.handleScopeTooLarge(bc, invResult.Result, explanation)
			return invResult, bc.Result.Error
		}

		// Check usage limits
		signals := usagelimit.Signals{
			ExitCode: invResult.Result.ExitCode,
			Output:   invResult.Result.Output,
		}
		if invResult.Stats != nil {
			signals.RateLimitHits = invResult.Stats.RateLimitHits
		}
		if usagelimit.Check(signals, usagelimit.ClaudePatterns()) {
			bc.Result.UsageLimited = true
			r.log("Warning: usage limit detected - stopping retry attempts")
			return nil, fmt.Errorf("usage limit detected: retries or escalation will not resolve this failure (exit code: %d, rate limit events: %d)", invResult.Result.ExitCode, signals.RateLimitHits)
		}

		return invResult, nil
	}
}

func (r *Runner) handleInvokeError(ctx context.Context, bc *runtypes.BeadContext, invResult *runtypes.InvocationResult, err error) (*runtypes.InvocationResult, error) {
	if invResult != nil && invResult.StallFired && ctx.Err() == nil {
		invResult.TimeoutType = "stall"
		return invResult, err
	}

	// Classify timeout type.
	if ctx.Err() != nil && bc.ParentCtx.Err() == nil {
		bc.Result.TimeoutType = "bead"
		escalation.ExtractTimeoutLearning(bc, r.renderer.GetLearningsFile())
		return stampTimeoutType(invResult, "bead"), fmt.Errorf("bead timeout: exceeded %v total processing time", bc.BeadTimeout)
	}
	if bc.ParentCtx.Err() != nil {
		return nil, fmt.Errorf("context cancelled: %w", bc.ParentCtx.Err())
	}

	bc.Result.TimeoutType = "invocation"
	return stampTimeoutType(invResult, "invocation"), fmt.Errorf("claude invocation: %w", err)
}

func stampTimeoutType(invResult *runtypes.InvocationResult, timeoutType string) *runtypes.InvocationResult {
	if invResult != nil {
		invResult.TimeoutType = timeoutType
		return invResult
	}
	return &runtypes.InvocationResult{TimeoutType: timeoutType}
}

func (r *Runner) shapeMethodologyPromptContext(phase string, ctx *prompt.Context) *prompt.Context {
	if ctx == nil || r == nil || r.renderer == nil {
		return ctx
	}
	var shaped *prompt.Context
	switch phase {
	case "red":
		if shaper, ok := r.renderer.(redPhasePromptShaper); ok {
			shaped = shaper.ShapeRedPhaseContext(ctx)
		}
	case "green":
		if shaper, ok := r.renderer.(greenPhasePromptShaper); ok {
			shaped = shaper.ShapeGreenPhaseContext(ctx)
		}
	case "refactor":
		if shaper, ok := r.renderer.(refactorPhasePromptShaper); ok {
			shaped = shaper.ShapeRefactorPhaseContext(ctx)
		}
	default:
		return ctx
	}
	if shaped == nil {
		return ctx
	}
	return shaped
}

// makeValidationExecuteFn creates a validation.ExecuteFn callback that wraps
// the escalation handler's ExecuteWithRetry for Claude-based validation fix attempts.
func (r *Runner) makeValidationExecuteFn() validation.ExecuteFn {
	return func(ctx context.Context, bc *runtypes.BeadContext) bool {
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
		// Validation repair should run a quick first pass, then one escalated pass.
		savedMaxRetries := bc.MaxRetries
		savedRetriesThisModel := bc.RetriesThisModel
		savedTotalRetries := bc.TotalRetriesThisBead
		savedTier := bc.Tier
		savedEscalationEnabled := r.cfg.Escalation.Enabled
		bc.MaxRetries = 0
		bc.RetriesThisModel = 0

		runSingleAttempt := func() bool {
			// Disable escalation inside ExecuteWithRetry so each call is exactly one attempt.
			r.cfg.Escalation.Enabled = false
			defer func() {
				r.cfg.Escalation.Enabled = savedEscalationEnabled
			}()
			return r.escalationHandler.ExecuteWithRetry(ctx, bc, r.makeInvokeFn())
		}

		success := runSingleAttempt()
		if !success {
			// If quick recovery fails, run one escalated-tier attempt.
			if nextTier := r.cfg.NextEscalationTier(savedTier); nextTier != "" {
				r.escalationHandler.EscalateTier(bc, nextTier)
				success = runSingleAttempt()
			}
		}

		// Restore retry state
		bc.MaxRetries = savedMaxRetries
		bc.RetriesThisModel = savedRetriesThisModel
		bc.TotalRetriesThisBead = savedTotalRetries
		r.cfg.Escalation.Enabled = savedEscalationEnabled

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
		return r.renderer.RenderAcceptanceTests(r.shapeMethodologyPromptContext("red", ctx))
	}

	// InvokeFn wraps the provider chain for acceptance test invocations
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error {
		if r.router == nil {
			return fmt.Errorf("router not configured")
		}
		streamInvoke := func(p provider.Provider, modelName string, attempt string) (*provider.Result, error) {
			startedAt := time.Now()
			var eventCount int64
			var lastEventUnixNano = startedAt.UnixNano()
			heartbeatStop := make(chan struct{})
			go func() {
				ticker := time.NewTicker(15 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						lastEvent := time.Unix(0, atomic.LoadInt64(&lastEventUnixNano))
						r.log(
							"ATDD progress: elapsed=%s events=%d idle=%s (%s provider=%s model=%s tier=%s)",
							time.Since(startedAt).Round(time.Second),
							atomic.LoadInt64(&eventCount),
							time.Since(lastEvent).Round(time.Second),
							attempt,
							p.Name(),
							modelName,
							bc.Tier,
						)
					case <-heartbeatStop:
						return
					}
				}
			}()
			defer close(heartbeatStop)

			var firstEventOnce sync.Once
			streamHandler := provider.EventHandler(func(line []byte) {
				atomic.AddInt64(&eventCount, 1)
				atomic.StoreInt64(&lastEventUnixNano, time.Now().UnixNano())
				firstEventOnce.Do(func() {
					r.log(
						"ATDD stream connected after %s (%s provider=%s model=%s tier=%s)",
						time.Since(startedAt).Round(time.Millisecond),
						attempt,
						p.Name(),
						modelName,
						bc.Tier,
					)
				})
			})

			r.log(
				"ATDD stream started (%s provider=%s model=%s tier=%s prompt_chars=%d)",
				attempt,
				p.Name(),
				modelName,
				bc.Tier,
				len(promptText),
			)
			result, err := p.StreamRun(ctx, promptText, bc.Tier, r.output, streamHandler, nil)
			elapsed := time.Since(startedAt).Round(time.Millisecond)
			if err != nil {
				r.log(
					"ATDD stream error after %s (%s provider=%s model=%s tier=%s events=%d): %v",
					elapsed,
					attempt,
					p.Name(),
					modelName,
					bc.Tier,
					atomic.LoadInt64(&eventCount),
					err,
				)
				return result, err
			}
			if result == nil {
				r.log(
					"ATDD stream returned nil result after %s (%s provider=%s model=%s tier=%s events=%d)",
					elapsed,
					attempt,
					p.Name(),
					modelName,
					bc.Tier,
					atomic.LoadInt64(&eventCount),
				)
				return nil, nil
			}
			r.log(
				"ATDD stream completed after %s (%s provider=%s model=%s tier=%s events=%d success=%t exit=%d failure_category=%s)",
				elapsed,
				attempt,
				p.Name(),
				modelName,
				bc.Tier,
				atomic.LoadInt64(&eventCount),
				result.Success,
				result.ExitCode,
				result.FailureCategory,
			)
			return result, nil
		}

		p, modelName := r.router.Select("build", bc.Tier)
		if p == nil {
			return fmt.Errorf("no providers available for phase=build tier=%s", bc.Tier)
		}
		bc.Model = modelName
		if bc.Result.Escalated && bc.Result.EscalatedTo != "" {
			bc.Result.EscalatedTo = modelName
		}
		result, err := streamInvoke(p, modelName, "primary")
		if err != nil {
			if p.IsUsageLimitError(result, err) {
				r.router.MarkUnavailable(p.Name())
				p2, modelName2 := r.router.Select("build", bc.Tier)
				if p2 != nil {
					bc.Model = modelName2
					result, err = streamInvoke(p2, modelName2, "usage-limit-fallback")
				}
			}
			if err != nil {
				return fmt.Errorf("acceptance tests provider invocation failed (provider=%s model=%s): %w", p.Name(), modelName, err)
			}
		}
		if result == nil {
			return fmt.Errorf("acceptance tests failed (provider=%s model=%s): nil result", p.Name(), modelName)
		}
		if !result.Success && p.Name() == "codex" && result.FailureCategory == provider.FailureCategoryTransportDisconnect {
			r.log("ATDD fallback: codex transport failure, retrying with alternate provider")
			r.router.MarkUnavailable(p.Name())
			p2, modelName2 := r.router.SelectCross(p.Name(), bc.Tier)
			if p2 != nil && p2.Name() != p.Name() {
				fallbackResult, fallbackErr := streamInvoke(p2, modelName2, "codex-transport-fallback")
				if fallbackErr != nil {
					return fmt.Errorf(
						"acceptance tests failed after codex transport fallback (provider=%s model=%s): primary={%s} fallback_err=%v",
						p2.Name(),
						modelName2,
						formatATDDProviderFailure(p, modelName, result),
						fallbackErr,
					)
				}
				if fallbackResult == nil {
					return fmt.Errorf(
						"acceptance tests failed after codex transport fallback (provider=%s model=%s): primary={%s} fallback=nil result",
						p2.Name(),
						modelName2,
						formatATDDProviderFailure(p, modelName, result),
					)
				}
				if fallbackResult.Success {
					bc.Model = modelName2
					return nil
				}
				return fmt.Errorf(
					"acceptance tests failed after codex transport fallback: primary={%s} fallback={%s}",
					formatATDDProviderFailure(p, modelName, result),
					formatATDDProviderFailure(p2, modelName2, fallbackResult),
				)
			}
		}
		if !result.Success {
			return fmt.Errorf("acceptance tests failed: %s", formatATDDProviderFailure(p, modelName, result))
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
			return r.renderer.RenderRefactor(r.shapeMethodologyPromptContext("refactor", ctx))
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
