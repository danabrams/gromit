package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
const (
	invocationStartMarkerFormat = "INVOCATION_START provider=%s model=%s tier=%s"
	invocationEndMarkerFormat   = "INVOCATION_END provider=%s model=%s tier=%s success=%t duration=%s failure_category=%s"
	atddStartMarkerFormat       = "ATDD_INVOCATION_START provider=%s model=%s tier=%s"
	atddEndMarkerFormat         = "ATDD_INVOCATION_END provider=%s model=%s tier=%s success=%t duration=%s failure_category=%s"
	atddFallbackDecisionFormat  = "ATDD_FALLBACK_DECISION class=%s primary_provider=%s primary_model=%s fallback_provider=auto fallback_model=auto reason=%s"
	atddFallbackAttemptFormat   = "ATDD_FALLBACK_ATTEMPT class=%s primary_provider=%s primary_model=%s fallback_provider=%s fallback_model=%s"
	atddFallbackOutcomeFormat   = "ATDD_FALLBACK_OUTCOME class=%s primary_provider=%s primary_model=%s fallback_provider=%s fallback_model=%s success=%t fallback_failure_class=%s"
	atddFallbackReasonError     = "primary_error"
	atddFallbackReasonResult    = "primary_result"
	atddFallbackClassNone       = "none"
)

const (
	atddFailureClassTransport = provider.FailureCategoryTransportDisconnect
	atddFailureClassStartup   = provider.FailureCategoryStartupError
	atddFailureClassOther     = provider.FailureCategoryOther
	atddBuildPhase            = "build"
)

var atddTransportFailureSignals = []string{
	"stream disconnected",
	"connection reset",
	"broken pipe",
}

var atddStartupFailureSignals = []string{
	"failed to start",
	"startup",
	"initializ",
	"timed out waiting for first event",
}

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
		invResult, providerResult, err := r.executeClaudeInvocation(ctx, bc)
		r.logInvocationEndMarker(bc, invResult, err, time.Since(startedAt))

		if err != nil {
			return r.handleInvokeError(ctx, bc, invResult, providerResult, err)
		}

		if invResult == nil || invResult.Result == nil {
			setFailurePhaseIfResult(bc, failurephase.Build)
			return nil, fmt.Errorf("claude returned nil result")
		}
		if providerResult == nil {
			providerResult = &provider.Result{
				Success:  invResult.Result.Success,
				Output:   invResult.Result.Output,
				ExitCode: invResult.Result.ExitCode,
				Duration: invResult.Result.Duration,
				Model:    invResult.Result.Model,
			}
		}

		bc.Result.Provider = invResult.ProviderName
		if invResult.ProviderResult != nil {
			bc.Result.FailureCategory = invResult.ProviderResult.FailureCategory
			bc.Result.ReasoningEffort = invResult.ProviderResult.ReasoningEffort
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
		if isTooLarge, explanation := provider.IsScopeTooLarge(providerResult); isTooLarge {
			r.handleScopeTooLarge(bc, providerResult, explanation)
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
	r.logLifecycleStartMarker(invocationStartMarkerFormat, bc.BuildProvider, bc.Model, bc.Tier)
}

func (r *Runner) logInvocationEndMarker(bc *runtypes.BeadContext, invResult *runtypes.InvocationResult, err error, elapsed time.Duration) {
	providerName, modelName, tier, success, failureCategory := invocationLifecycleFields(bc, invResult, err)
	r.logLifecycleEndMarker(invocationEndMarkerFormat, providerName, modelName, tier, success, elapsed, failureCategory)
}

func (r *Runner) logLifecycleStartMarker(format, providerName, modelName, tier string) {
	r.streamLogger.LogEvent(format, providerName, modelName, tier)
}

func (r *Runner) logLifecycleEndMarker(format, providerName, modelName, tier string, success bool, elapsed time.Duration, failureCategory string) {
	r.streamLogger.LogEvent(format, providerName, modelName, tier, success, elapsed.Round(time.Millisecond), failureCategory)
}

func invocationLifecycleFields(bc *runtypes.BeadContext, invResult *runtypes.InvocationResult, err error) (providerName, modelName, tier string, success bool, failureCategory string) {
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
	return providerName, modelName, tier, success, failureCategory
}

func classifyATDDFailure(result *provider.Result, err error) string {
	if result != nil {
		if result.FailureCategory == provider.FailureCategoryTransportDisconnect {
			return atddFailureClassTransport
		}
		if result.FailureCategory == provider.FailureCategoryStartupError {
			return atddFailureClassStartup
		}
	}

	var failureText strings.Builder
	if err != nil {
		failureText.WriteString(err.Error())
		failureText.WriteString("\n")
	}
	if result != nil {
		failureText.WriteString(result.Stderr)
		failureText.WriteString("\n")
		failureText.WriteString(result.Output)
		failureText.WriteString("\n")
		failureText.WriteString(result.Diagnostics)
	}
	content := strings.ToLower(failureText.String())

	if containsAny(content, atddTransportFailureSignals) {
		return atddFailureClassTransport
	}
	if containsAny(content, atddStartupFailureSignals) {
		return atddFailureClassStartup
	}
	return atddFailureClassOther
}

func containsAny(haystack string, signals []string) bool {
	for _, signal := range signals {
		if strings.Contains(haystack, signal) {
			return true
		}
	}
	return false
}

func isATDDFallbackEligible(failureClass string) bool {
	return failureClass == atddFailureClassTransport || failureClass == atddFailureClassStartup
}

func (r *Runner) logATDDFallbackDecision(failureClass, providerName, modelName, reason string) {
	r.log(atddFallbackDecisionFormat, failureClass, providerName, modelName, reason)
}

func (r *Runner) logATDDFallbackAttempt(failureClass, primaryProvider, primaryModel, fallbackProvider, fallbackModel string) {
	r.log(atddFallbackAttemptFormat, failureClass, primaryProvider, primaryModel, fallbackProvider, fallbackModel)
}

func (r *Runner) logATDDFallbackOutcome(failureClass, primaryProvider, primaryModel, fallbackProvider, fallbackModel string, fallbackResult *provider.Result, fallbackErr error) {
	success := fallbackErr == nil && fallbackResult != nil && fallbackResult.Success
	fallbackFailureClass := atddFallbackClassNone
	if !success {
		fallbackFailureClass = classifyATDDFailure(fallbackResult, fallbackErr)
	}
	r.log(
		atddFallbackOutcomeFormat,
		failureClass,
		primaryProvider,
		primaryModel,
		fallbackProvider,
		fallbackModel,
		success,
		fallbackFailureClass,
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

func (r *Runner) handleInvokeError(ctx context.Context, bc *runtypes.BeadContext, invResult *runtypes.InvocationResult, providerResult *provider.Result, err error) (*runtypes.InvocationResult, error) {
	r.ensureEscalationPolicy()
	classification := r.escalationPolicy.ClassifyTimeout(ctx.Err(), bc.ParentCtx.Err(), invResult != nil && invResult.StallFired)

	switch classification.TimeoutType {
	case "stall":
		return stampTimeoutType(invResult, providerResult, "stall"), err
	case "bead":
		bc.Result.TimeoutType = "bead"
		if r.renderer != nil {
			escalation.ExtractTimeoutLearning(bc, r.renderer.GetLearningsFile())
		}
		return stampTimeoutType(invResult, providerResult, "bead"), fmt.Errorf("bead timeout: exceeded %v total processing time", bc.BeadTimeout)
	case "invocation":
		bc.Result.TimeoutType = "invocation"
		return stampTimeoutType(invResult, providerResult, "invocation"), fmt.Errorf("claude invocation: %w", err)
	}

	if classification.ParentCanceled {
		if bc != nil && bc.Result != nil {
			setFailurePhaseIfResult(bc, failurephase.Timeout)
			bc.Result.TimeoutPhase = "build"
		}
		return nil, fmt.Errorf("context cancelled: %w", bc.ParentCtx.Err())
	}

	bc.Result.TimeoutType = "invocation"
	return stampTimeoutType(invResult, providerResult, "invocation"), fmt.Errorf("claude invocation: %w", err)
}

func stampTimeoutType(invResult *runtypes.InvocationResult, providerResult *provider.Result, timeoutType string) *runtypes.InvocationResult {
	if invResult != nil {
		if invResult.ProviderResult == nil {
			invResult.ProviderResult = providerResult
		}
		invResult.TimeoutType = timeoutType
		return invResult
	}
	return &runtypes.InvocationResult{TimeoutType: timeoutType, ProviderResult: providerResult}
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
						if fallbackErr != nil {
							return fmt.Errorf(
								"acceptance tests failed after transient fallback class=%s (provider=%s model=%s): primary_err=%v fallback_err=%v",
								failureClass,
								p2.Name(),
								modelName2,
								err,
								fallbackErr,
							)
						}
						if fallbackResult == nil {
							return fmt.Errorf(
								"acceptance tests failed after transient fallback class=%s (provider=%s model=%s): primary_err=%v fallback=nil result",
								failureClass,
								p2.Name(),
								modelName2,
								err,
							)
						}
						if fallbackResult.Success {
							setSelectedModel(modelName2)
							return nil
						}
						return fmt.Errorf(
							"acceptance tests failed after transient fallback class=%s: primary_err=%v fallback={%s}",
							failureClass,
							err,
							formatATDDProviderFailure(p2, modelName2, fallbackResult),
						)
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
					if fallbackErr != nil {
						return fmt.Errorf(
							"acceptance tests failed after transient fallback class=%s (provider=%s model=%s): primary={%s} fallback_err=%v",
							failureClass,
							p2.Name(),
							modelName2,
							formatATDDProviderFailure(p, modelName, result),
							fallbackErr,
						)
					}
					if fallbackResult == nil {
						return fmt.Errorf(
							"acceptance tests failed after transient fallback class=%s (provider=%s model=%s): primary={%s} fallback=nil result",
							failureClass,
							p2.Name(),
							modelName2,
							formatATDDProviderFailure(p, modelName, result),
						)
					}
					if fallbackResult.Success {
						setSelectedModel(modelName2)
						return nil
					}
					return fmt.Errorf(
						"acceptance tests failed after transient fallback class=%s: primary={%s} fallback={%s}",
						failureClass,
						formatATDDProviderFailure(p, modelName, result),
						formatATDDProviderFailure(p2, modelName2, fallbackResult),
					)
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
