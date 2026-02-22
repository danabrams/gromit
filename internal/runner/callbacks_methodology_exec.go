package runner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

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
		r.capturePromptDiagnostics(bc.Result)
		applyCostData := func(stats *logger.StreamStats) {
			if stats == nil {
				return
			}
			costUSD, inputTokens, outputTokens := stats.CostData()
			bc.Result.CostUSD = costUSD
			bc.Result.InputTokens = inputTokens
			bc.Result.OutputTokens = outputTokens
			reconcilePromptDiagnostics(bc.Result.PromptDiagnostics, inputTokens)
		}
		streamInvoke := func(p provider.Provider, modelName string, attempt string) (result *provider.Result, stats *logger.StreamStats, err error) {
			startedAt := time.Now()
			r.logLifecycleStartMarker(atddStartMarkerFormat, p.Name(), modelName, bc.Tier)
			defer func() {
				failureCategory := ""
				success := false
				if result != nil {
					failureCategory = result.FailureCategory
					success = result.Success
				}
				if err != nil {
					success = false
				}
				r.logLifecycleEndMarker(atddEndMarkerFormat, p.Name(), modelName, bc.Tier, success, time.Since(startedAt), failureCategory)
			}()

			var eventCount int64
			var lastEventUnixNano = startedAt.UnixNano()
			heartbeatStop := make(chan struct{})
			var statsErr error
			stats, statsErr = logger.NewStreamStats()
			if statsErr != nil {
				r.log("Warning: could not create stream stats for ATDD: %v", statsErr)
			}
			go func() {
				ticker := time.NewTicker(atddProgressLogInterval)
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
				logger.ParseAndLogEvent(r.streamLogger, stats, line)
			})

			r.log(
				"ATDD stream started (%s provider=%s model=%s tier=%s prompt_chars=%d)",
				attempt,
				p.Name(),
				modelName,
				bc.Tier,
				len(promptText),
			)
			result, err = p.StreamRun(ctx, promptText, bc.Tier, r.output, streamHandler, nil)
			elapsed := time.Since(startedAt).Round(time.Millisecond)
			if stats != nil && result != nil {
				stats.MergeCostData(result.CostUSD, result.InputTokens, result.OutputTokens)
			}
			failureCategory := ""
			if result != nil {
				failureCategory = result.FailureCategory
			}
			r.router.RecordOutcome(p.Name(), failureCategory)
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
				return result, stats, err
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
				return nil, stats, nil
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
			return result, stats, nil
		}

		p, modelName := r.router.Select(atddBuildPhase, bc.Tier)
		if p == nil {
			return fmt.Errorf("no providers available for phase=%s tier=%s", atddBuildPhase, bc.Tier)
		}
		setSelectedModel := func(selectedModel string) {
			bc.Model = selectedModel
			if bc.Result.Escalated && bc.Result.EscalatedTo != "" {
				bc.Result.EscalatedTo = selectedModel
			}
		}
		setSelectedModel(modelName)
		result, stats, err := streamInvoke(p, modelName, "primary")
		applyCostData(stats)
		runTransientFallback := func(failureClass, reason string) (provider.Provider, string, *provider.Result, error, bool) {
			r.logATDDFallbackDecision(failureClass, p.Name(), modelName, reason)
			r.router.MarkUnavailable(p.Name())
			p2, modelName2 := r.router.SelectCross(p.Name(), bc.Tier)
			if p2 == nil || p2.Name() == p.Name() {
				return nil, "", nil, nil, false
			}

			r.logATDDFallbackAttempt(failureClass, p.Name(), modelName, p2.Name(), modelName2)
			recordFallbackAttempt(bc.Result)
			fallbackResult, fallbackStats, fallbackErr := streamInvoke(p2, modelName2, "transient-fallback")
			applyCostData(fallbackStats)
			r.logATDDFallbackOutcome(failureClass, p.Name(), modelName, p2.Name(), modelName2, fallbackResult, fallbackErr)
			recordFallbackOutcome(bc.Result, fallbackErr == nil && fallbackResult != nil && fallbackResult.Success)
			return p2, modelName2, fallbackResult, fallbackErr, true
		}
		handleTransientFallback := func(failureClass, primaryInfo string, p2 provider.Provider, modelName2 string, fallbackResult *provider.Result, fallbackErr error) error {
			fallbackInfo := formatATDDFallbackInfo(p2, modelName2, fallbackResult, fallbackErr)
			if fallbackFailureErr := fallbackErrorResult(failureClass, primaryInfo, fallbackInfo, fallbackResult, fallbackErr); fallbackFailureErr != nil {
				return fallbackFailureErr
			}
			setSelectedModel(modelName2)
			return nil
		}
		if err != nil {
			if p.IsUsageLimitError(result, err) {
				r.router.MarkUnavailable(p.Name())
				p2, modelName2 := r.router.Select(atddBuildPhase, bc.Tier)
				if p2 != nil {
					recordFallbackAttempt(bc.Result)
					setSelectedModel(modelName2)
					result, stats, err = streamInvoke(p2, modelName2, "usage-limit-fallback")
					applyCostData(stats)
					recordFallbackOutcome(bc.Result, err == nil && result != nil && result.Success)
					p = p2
					modelName = modelName2
				}
			}
			if err != nil {
				failureClass := classifyATDDFailure(result, err)
				if isATDDFallbackEligible(failureClass) {
					p2, modelName2, fallbackResult, fallbackErr, attempted := runTransientFallback(failureClass, atddFallbackReasonError)
					if attempted {
						primaryInfo := fmt.Sprintf("primary_err=%v", err)
						if fallbackHandledErr := handleTransientFallback(failureClass, primaryInfo, p2, modelName2, fallbackResult, fallbackErr); fallbackHandledErr != nil {
							return fallbackHandledErr
						}
						if fallbackResult != nil && fallbackResult.Success {
							return nil
						}
					}
				}
			}
			if err != nil {
				return fmt.Errorf("acceptance tests provider invocation failed (provider=%s model=%s): %w", p.Name(), modelName, err)
			}
		}
		if result == nil {
			return fmt.Errorf("acceptance tests failed (provider=%s model=%s): nil result", p.Name(), modelName)
		}
		if !result.Success {
			failureClass := classifyATDDFailure(result, nil)
			if isATDDFallbackEligible(failureClass) {
				p2, modelName2, fallbackResult, fallbackErr, attempted := runTransientFallback(failureClass, atddFallbackReasonResult)
				if attempted {
					primaryInfo := fmt.Sprintf("primary={%s}", formatATDDProviderFailure(p, modelName, result))
					if fallbackHandledErr := handleTransientFallback(failureClass, primaryInfo, p2, modelName2, fallbackResult, fallbackErr); fallbackHandledErr != nil {
						return fallbackHandledErr
					}
					if fallbackResult != nil && fallbackResult.Success {
						return nil
					}
				}
			}
		}
		if !result.Success {
			return fmt.Errorf("acceptance tests failed: %s", formatATDDProviderFailure(p, modelName, result))
		}
		return nil
	}

	// ValidateDirectFn wraps the validation runner's direct validation
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		result, err := r.runDirectValidationCheck(ctx, commands, workDir)
		if err != nil || result == nil {
			return nil, err
		}
		return &claude.Result{
			Success:  result.Success,
			Output:   result.Output,
			ExitCode: result.ExitCode,
			Duration: result.Duration,
			Model:    result.Model,
		}, nil
	}

	// EscalateTierFn wraps escalation.Handler.EscalateTier
	escalateTierFn := func(bc *runtypes.BeadContext, nextTier string) {
		if r.escalationHandler != nil {
			r.escalationHandler.EscalateTier(bc, nextTier)
		}
	}

	// GetDiffFn wraps the runner's git diff
	getDiffFn := func(startCommit string) (string, error) {
		return r.getDiff(startCommit)
	}

	diagnosticInvokeFn := func(ctx context.Context, promptText string, tier string) (*claude.Result, error) {
		if r.router == nil {
			return nil, fmt.Errorf("router not configured")
		}
		p, modelName := r.router.Select("build", tier)
		if p == nil {
			return nil, fmt.Errorf("no providers available for phase=%s tier=%s", "build", tier)
		}
		result, err := p.Run(ctx, promptText, tier)
		if err != nil && p.IsUsageLimitError(result, err) {
			r.router.MarkUnavailable(p.Name())
			p2, modelName2 := r.router.Select("build", tier)
			if p2 != nil {
				result, err = p2.Run(ctx, promptText, tier)
				p = p2
				modelName = modelName2
			}
		}
		if err != nil {
			return nil, fmt.Errorf("atdd diagnostic provider invocation failed (provider=%s model=%s): %w", p.Name(), modelName, err)
		}
		if result == nil {
			return nil, fmt.Errorf("atdd diagnostic returned nil result")
		}
		return &claude.Result{
			Success:  result.Success,
			Output:   result.Output,
			ExitCode: result.ExitCode,
			Duration: result.Duration,
			Model:    modelName,
		}, nil
	}

	renderDiagnosticFn := func(ctx *prompt.DiagnosticContext) (string, error) {
		if r.renderer == nil {
			return "", fmt.Errorf("renderer not configured")
		}
		return r.renderer.RenderATDDDiagnostic(ctx)
	}

	// Create the base executor with ATDD callbacks
	methExec := methodology.NewExecutorWithEscalation(r.cfg, r.output, renderFn, invokeFn, validateFn, escalateTierFn)

	methExec.SetGetDiffFn(getDiffFn)
	methExec.SetCoverageValidateFn(r.makeCoverageValidateFn())
	methExec.SetDiagnosticDeps(diagnosticInvokeFn, renderDiagnosticFn)

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
		r.resetHard,
		r.getHead,
	))

	return methExec
}
