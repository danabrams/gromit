package runner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/specgate"
	"github.com/danabrams/gromit/internal/usagelimit"
)

const atddProgressLogInterval = 15 * time.Second

func newSpecGate(r *Runner) (*specgate.Gate, error) {
	if r == nil {
		return nil, fmt.Errorf("runner is nil")
	}
	return r.buildSpecGate()
}

type redPhasePromptShaper interface {
	ShapeRedPhaseContext(ctx *prompt.Context) *prompt.Context
}

type greenPhasePromptShaper interface {
	ShapeGreenPhaseContext(ctx *prompt.Context) *prompt.Context
}

type refactorPhasePromptShaper interface {
	ShapeRefactorPhaseContext(ctx *prompt.Context) *prompt.Context
}

func setFailurePhaseIfResult(bc *runtypes.BeadContext, phase string) {
	if bc != nil && bc.Result != nil {
		bc.Result.FailurePhase = phase
	}
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
				bc.TouchedPackages = methodology.DetectTouchedPackages(diff)
			}
		}
		injectScopedTestCommand(bc)

		// Re-render build prompt when retry context changes or when the scoped
		// self-check command changes from git diff detection.
		if bc.PromptCtx != nil && r.renderer != nil && (bc.PromptCtx.IsRetry || bc.PromptCtx.ScopedTestCommand != prevScopedTestCmd) {
			rendered, renderErr := r.renderer.RenderBuild(r.shapeMethodologyPromptContext("green", bc.PromptCtx))
			if renderErr == nil {
				bc.BuildPrompt = rendered
				r.capturePromptDiagnostics(bc.Result)
			}
		}

		r.log("Running Claude with model: %s", bc.Model)

		startedAt := time.Now()
		r.logInvocationStartMarker(bc)
		invResult, err := r.executeClaudeInvocation(ctx, bc)
		r.logInvocationEndMarker(bc, invResult, err, time.Since(startedAt))

		if err != nil {
			return r.handleInvokeError(ctx, bc, invResult, err)
		}

		if invResult == nil || invResult.Result == nil {
			setFailurePhaseIfResult(bc, failurephase.Build)
			return nil, fmt.Errorf("claude returned nil result")
		}

		bc.Result.Provider = invResult.ProviderName
		if invResult.ProviderResult != nil {
			bc.Result.FailureCategory = invResult.ProviderResult.FailureCategory
		}
		r.router.RecordOutcome(bc.Result.Provider, bc.Result.FailureCategory)

		// Populate cost/token data
		if invResult.Stats != nil {
			costUSD, inputTokens, outputTokens := invResult.Stats.CostData()
			bc.Result.CostUSD = r.estimatedCostUSD(invResult.ProviderName, bc.Result.Model, costUSD, inputTokens, outputTokens)
			bc.Result.InputTokens = inputTokens
			bc.Result.OutputTokens = outputTokens
			reconcilePromptDiagnostics(bc.Result.PromptDiagnostics, inputTokens)
		}

		// Check scope-too-large
		if isTooLarge, explanation := claude.IsScopeTooLarge(invResult.Result); isTooLarge {
			r.handleScopeTooLarge(bc, invResult.Result, explanation)
			setFailurePhaseIfResult(bc, failurephase.Build)
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
			setFailurePhaseIfResult(bc, failurephase.Build)
			r.log("Warning: usage limit detected - stopping retry attempts")
			return nil, fmt.Errorf("usage limit detected: retries or escalation will not resolve this failure (exit code: %d, rate limit events: %d)", invResult.Result.ExitCode, signals.RateLimitHits)
		}

		return invResult, nil
	}
}

func (r *Runner) logInvocationStartMarker(bc *runtypes.BeadContext) {
	if bc == nil {
		return
	}
	r.streamLogger.LogEvent("INVOCATION_START provider=%s model=%s tier=%s", bc.BuildProvider, bc.Model, bc.Tier)
}

func (r *Runner) logInvocationEndMarker(bc *runtypes.BeadContext, invResult *runtypes.InvocationResult, err error, elapsed time.Duration) {
	tier := ""
	providerName := ""
	modelName := ""
	success := false
	failureCategory := ""

	if bc != nil {
		tier = bc.Tier
		providerName = bc.BuildProvider
		modelName = bc.Model
	}
	if invResult != nil {
		if invResult.ProviderName != "" {
			providerName = invResult.ProviderName
		}
		if invResult.ModelName != "" {
			modelName = invResult.ModelName
		}
		if invResult.Result != nil {
			success = invResult.Result.Success
		}
		if invResult.ProviderResult != nil {
			failureCategory = invResult.ProviderResult.FailureCategory
		}
	}
	if err != nil {
		success = false
	}

	r.streamLogger.LogEvent(
		"INVOCATION_END provider=%s model=%s tier=%s success=%t duration=%s failure_category=%s",
		providerName,
		modelName,
		tier,
		success,
		elapsed.Round(time.Millisecond),
		failureCategory,
	)
}

func (r *Runner) estimatedCostUSD(providerName, model string, reportedCostUSD float64, inputTokens, outputTokens int) float64 {
	if reportedCostUSD != 0 || (inputTokens == 0 && outputTokens == 0) {
		return reportedCostUSD
	}

	for _, candidate := range costProviderCandidates(providerName) {
		provDef, ok := r.providerCostDefs[candidate]
		if ok {
			return provDef.EstimateCostForModel(model, inputTokens, outputTokens)
		}
	}
	return reportedCostUSD
}

func costProviderCandidates(providerName string) []string {
	switch providerName {
	case "codex":
		return []string{"codex", "openai"}
	case "openai":
		return []string{"openai", "codex"}
	default:
		return []string{providerName}
	}
}

func (r *Runner) handleInvokeError(ctx context.Context, bc *runtypes.BeadContext, invResult *runtypes.InvocationResult, err error) (*runtypes.InvocationResult, error) {
	r.ensureEscalationPolicy()
	classification := r.escalationPolicy.ClassifyTimeout(ctx.Err(), bc.ParentCtx.Err(), invResult != nil && invResult.StallFired)

	switch classification.TimeoutType {
	case "stall":
		if invResult != nil {
			invResult.TimeoutType = "stall"
			return invResult, err
		}
		return stampTimeoutType(invResult, "stall"), err
	case "bead":
		bc.Result.TimeoutType = "bead"
		if r.renderer != nil {
			escalation.ExtractTimeoutLearning(bc, r.renderer.GetLearningsFile())
		}
		return stampTimeoutType(invResult, "bead"), fmt.Errorf("bead timeout: exceeded %v total processing time", bc.BeadTimeout)
	case "invocation":
		bc.Result.TimeoutType = "invocation"
		return stampTimeoutType(invResult, "invocation"), fmt.Errorf("claude invocation: %w", err)
	}

	if classification.ParentCanceled {
		if bc != nil && bc.Result != nil {
			setFailurePhaseIfResult(bc, failurephase.Timeout)
			bc.Result.TimeoutPhase = "build"
		}
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
			r.streamLogger.LogEvent("ATDD_INVOCATION_START provider=%s model=%s tier=%s", p.Name(), modelName, bc.Tier)
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
				r.streamLogger.LogEvent(
					"ATDD_INVOCATION_END provider=%s model=%s tier=%s success=%t duration=%s failure_category=%s",
					p.Name(),
					modelName,
					bc.Tier,
					success,
					time.Since(startedAt).Round(time.Millisecond),
					failureCategory,
				)
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

		p, modelName := r.router.Select("build", bc.Tier)
		if p == nil {
			return fmt.Errorf("no providers available for phase=build tier=%s", bc.Tier)
		}
		bc.Model = modelName
		if bc.Result.Escalated && bc.Result.EscalatedTo != "" {
			bc.Result.EscalatedTo = modelName
		}
		result, stats, err := streamInvoke(p, modelName, "primary")
		applyCostData(stats)
		if err != nil {
			if p.IsUsageLimitError(result, err) {
				r.router.MarkUnavailable(p.Name())
				p2, modelName2 := r.router.Select("build", bc.Tier)
				if p2 != nil {
					bc.Model = modelName2
					result, stats, err = streamInvoke(p2, modelName2, "usage-limit-fallback")
					applyCostData(stats)
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
				fallbackResult, fallbackStats, fallbackErr := streamInvoke(p2, modelName2, "codex-transport-fallback")
				applyCostData(fallbackStats)
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
	methExec.SetCoverageValidateFn(r.makeCoverageValidateFn())

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
