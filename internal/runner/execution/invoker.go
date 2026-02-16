package execution

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// StallTimeoutFunc returns stall detection timeouts for a given model name.
// Returns (0, 0) to disable stall detection.
type StallTimeoutFunc func(model string) (stallTimeout, stallTimeoutActive time.Duration)

// InvocationResult captures the outcome of a single Claude invocation.
type InvocationResult struct {
	Result         *claude.Result
	Stats          *logger.StreamStats
	StallFired     bool
	ModelName      string
	ProviderName   string
	ProviderResult *provider.Result
}

// Invoker handles a single Claude invocation: provider selection, streaming,
// usage-limit fallback, and diagnostic capture.
type Invoker struct {
	router               Router
	output               io.Writer
	streamLogger         *logger.StreamLogger
	overwriteOut         OverwriteWriter
	stallTimeoutFn       StallTimeoutFunc
	invocationTimeoutFn  func(model string) time.Duration
	preserveNativeStream bool
}

// NewInvoker creates an Invoker with the given narrow dependencies.
func NewInvoker(router Router, output io.Writer, streamLogger *logger.StreamLogger) *Invoker {
	return &Invoker{
		router:       router,
		output:       output,
		streamLogger: streamLogger,
	}
}

// WithHeartbeat configures the invoker to display periodic heartbeat progress
// and optionally detect stalls. The OverwriteWriter is used for in-place
// terminal updates. The StallTimeoutFunc provides per-model stall timeouts;
// pass nil to disable stall detection while keeping progress display.
func (inv *Invoker) WithHeartbeat(out OverwriteWriter, stallTimeoutFn StallTimeoutFunc) *Invoker {
	inv.overwriteOut = out
	inv.stallTimeoutFn = stallTimeoutFn
	return inv
}

// WithInvocationTimeout configures per-model invocation timeouts for each provider call.
// Returning 0 or less disables the timeout and uses the parent context only.
func (inv *Invoker) WithInvocationTimeout(timeoutFn func(model string) time.Duration) *Invoker {
	inv.invocationTimeoutFn = timeoutFn
	return inv
}

// WithPreserveProviderTerminalStream controls whether the invoker should prefer
// provider-native terminal stream rendering over structured event parsing.
func (inv *Invoker) WithPreserveProviderTerminalStream(enabled bool) *Invoker {
	inv.preserveNativeStream = enabled
	return inv
}

// Execute runs a single Claude invocation, returning the result and diagnostics.
// It does not decide whether to retry or escalate — the caller handles that.
func (inv *Invoker) Execute(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error) {
	stats, err := logger.NewStreamStats()
	if err != nil {
		return nil, err
	}

	if inv.router == nil {
		return nil, fmt.Errorf("runner router is nil")
	}

	phase := "build"
	tier := bc.Tier

	inv.logLifecycleStart(tier)

	p, modelName := inv.router.Select(phase, tier)
	if p == nil {
		return nil, fmt.Errorf("no providers available for phase=%s tier=%s", phase, tier)
	}

	inv.logLifecycleSelection(p.Name(), modelName, tier)

	bc.Model = modelName
	bc.Result.Model = modelName
	bc.BuildProvider = p.Name()
	if bc.Result.Escalated && bc.Result.EscalatedTo != "" {
		bc.Result.EscalatedTo = modelName
	}

	var invocationCtx context.Context
	var invocationCancel context.CancelFunc
	if inv.invocationTimeoutFn != nil {
		if timeout := inv.invocationTimeoutFn(modelName); timeout > 0 {
			invocationCtx, invocationCancel = context.WithTimeout(ctx, timeout)
		} else {
			invocationCtx, invocationCancel = context.WithCancel(ctx)
		}
	} else {
		invocationCtx, invocationCancel = context.WithCancel(ctx)
	}
	defer invocationCancel()

	stallFired := false

	preserveProviderTerminalStream := shouldPreserveProviderTerminalStream(inv.preserveNativeStream)

	// Set up heartbeat for progress display and stall detection.
	// When preserving provider-native terminal streams, disable heartbeat so
	// progress lines don't interfere with provider-owned terminal rendering.
	var toolCallEvents chan claude.ToolEvent
	var stopHeartbeat func()
	if inv.overwriteOut != nil && !preserveProviderTerminalStream {
		toolCallEvents = make(chan claude.ToolEvent, 100)
		var stallTimeout, stallTimeoutActive time.Duration
		if inv.stallTimeoutFn != nil {
			stallTimeout, stallTimeoutActive = inv.stallTimeoutFn(modelName)
		}
		stopHeartbeat = StartHeartbeat(stats, stallTimeout, stallTimeoutActive, func() {
			stallFired = true
			invocationCancel()
		}, toolCallEvents, inv.overwriteOut)
	}

	// Default behavior installs a structured stream event handler so providers that
	// gate JSON mode on handler presence (Codex --json) still emit parseable events.
	// In preserve mode, we intentionally pass nil handlers to keep provider-native
	// terminal output (color + structure) untouched.
	sl := inv.streamLogger
	handler := func(line []byte) {
		logger.ParseAndLogEvent(sl, stats, line)
	}

	var providerHandler provider.EventHandler
	if !preserveProviderTerminalStream {
		providerHandler = provider.EventHandler(handler)
	}

	var providerToolHandler provider.ToolCallHandler
	if !preserveProviderTerminalStream {
		providerToolHandler = func(event provider.ToolEvent) {
			if toolCallEvents != nil {
				select {
				case toolCallEvents <- claude.ToolEvent{
					ToolName: event.ToolName,
					FilePath: event.FilePath,
				}:
				default:
				}
			}
		}
	}

	providerResult, err := p.StreamRun(invocationCtx, bc.BuildPrompt, tier, inv.output, providerHandler, providerToolHandler)

	if err != nil && p.IsUsageLimitError(providerResult, err) {
		inv.router.MarkUnavailable(p.Name())

		p2, modelName2 := inv.router.Select(phase, tier)
		if p2 != nil {
			inv.logLifecycleSelection(p2.Name(), modelName2, tier)

			bc.Model = modelName2
			bc.Result.Model = modelName2
			modelName = modelName2

			providerResult, err = p2.StreamRun(invocationCtx, bc.BuildPrompt, tier, inv.output, providerHandler, providerToolHandler)
			p = p2
		}
	}

	// Prefer stream-event cost/token data, but fall back to provider-level usage
	// (e.g., when the provider exposes usage only in terminal turn metadata).
	if stats != nil && providerResult != nil {
		stats.MergeCostData(providerResult.CostUSD, providerResult.InputTokens, providerResult.OutputTokens)
	}

	// Stop heartbeat before reading stallFired — stopHeartbeat waits for the
	// goroutine to finish, establishing a happens-before relationship.
	if stopHeartbeat != nil {
		stopHeartbeat()
	}

	var claudeResult *claude.Result
	if providerResult != nil {
		claudeResult = &claude.Result{
			Success:  providerResult.Success,
			Output:   providerResult.Output,
			ExitCode: providerResult.ExitCode,
			Duration: providerResult.Duration,
			Model:    providerResult.Model,
		}
	}

	inv.logLifecycleCompletion(p, modelName, tier, providerResult, err)

	stallCount, stallTier, ttfe, toolCalls, rateLimitHits, rateLimitRecoveryMs := stats.DiagnosticSnapshot()
	bc.Result.StallCount = stallCount
	bc.Result.StallTier = stallTier
	bc.Result.TimeToFirstEventMs = ttfe.Milliseconds()
	bc.Result.ToolCallCount = toolCalls
	bc.Result.RateLimitHits = rateLimitHits
	bc.Result.RateLimitRecoveryMs = rateLimitRecoveryMs

	return &InvocationResult{
		Result:         claudeResult,
		Stats:          stats,
		StallFired:     stallFired,
		ModelName:      modelName,
		ProviderName:   p.Name(),
		ProviderResult: providerResult,
	}, err
}

func shouldPreserveProviderTerminalStream(defaultValue bool) bool {
	raw, ok := os.LookupEnv("GROMIT_PRESERVE_PROVIDER_STREAM")
	if !ok {
		return defaultValue
	}
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "1", "true", "yes", "on", "raw", "passthrough", "native":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}

func (inv *Invoker) logLifecycleStart(tier string) {
	if inv == nil {
		return
	}
	inv.streamLogger.LogEvent("%s tier=%s", InvocationLifecycleMarkerStart, tier)
}

func (inv *Invoker) logLifecycleSelection(providerName, modelName, tier string) {
	if inv == nil {
		return
	}
	inv.streamLogger.LogEvent("%s provider=%s model=%s tier=%s", InvocationLifecycleMarkerSelection, providerName, modelName, tier)
}

func (inv *Invoker) logLifecycleCompletion(p Provider, modelName, tier string, result *provider.Result, err error) {
	if inv == nil {
		return
	}
	if err != nil {
		inv.streamLogger.LogEvent("%s provider=%s model=%s tier=%s error=%v", InvocationLifecycleMarkerFailure, safeProviderName(p), modelName, tier, err)
		return
	}
	success := false
	if result != nil {
		success = result.Success
	}
	inv.streamLogger.LogEvent("%s provider=%s model=%s tier=%s success=%t", InvocationLifecycleMarkerComplete, safeProviderName(p), modelName, tier, success)
}

func safeProviderName(p Provider) string {
	if p == nil {
		return ""
	}
	return p.Name()
}
