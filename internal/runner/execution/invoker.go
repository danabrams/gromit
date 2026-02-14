package execution

import (
	"context"
	"fmt"
	"io"
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
	Result       *claude.Result
	Stats        *logger.StreamStats
	StallFired   bool
	ModelName    string
	ProviderName string
}

// Invoker handles a single Claude invocation: provider selection, streaming,
// usage-limit fallback, and diagnostic capture.
type Invoker struct {
	router         Router
	output         io.Writer
	streamLogger   *logger.StreamLogger
	overwriteOut   OverwriteWriter
	stallTimeoutFn StallTimeoutFunc
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

	p, modelName := inv.router.Select(phase, tier)
	if p == nil {
		return nil, fmt.Errorf("no providers available for phase=%s tier=%s", phase, tier)
	}

	bc.Model = modelName
	bc.Result.Model = modelName
	bc.BuildProvider = p.Name()
	if bc.Result.Escalated && bc.Result.EscalatedTo != "" {
		bc.Result.EscalatedTo = modelName
	}

	childCtx, childCancel := context.WithCancel(ctx)
	defer childCancel()

	stallFired := false

	// Set up heartbeat for progress display and stall detection
	var toolCallEvents chan claude.ToolEvent
	var stopHeartbeat func()
	if inv.overwriteOut != nil {
		toolCallEvents = make(chan claude.ToolEvent, 100)
		var stallTimeout, stallTimeoutActive time.Duration
		if inv.stallTimeoutFn != nil {
			stallTimeout, stallTimeoutActive = inv.stallTimeoutFn(modelName)
		}
		stopHeartbeat = StartHeartbeat(stats, stallTimeout, stallTimeoutActive, func() {
			stallFired = true
			childCancel()
		}, toolCallEvents, inv.overwriteOut)
	}

	// Always install an event handler so providers that gate structured streaming
	// on handler presence (Codex --json mode) still stream live output/events even
	// when stream logging to file is disabled.
	sl := inv.streamLogger
	handler := func(line []byte) {
		logger.ParseAndLogEvent(sl, stats, line)
	}

	var providerHandler provider.EventHandler
	if handler != nil {
		providerHandler = provider.EventHandler(handler)
	}

	providerToolHandler := func(event provider.ToolEvent) {
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

	providerResult, err := p.StreamRun(childCtx, bc.BuildPrompt, tier, inv.output, providerHandler, providerToolHandler)

	if err != nil && p.IsUsageLimitError(providerResult, err) {
		inv.router.MarkUnavailable(p.Name())

		p2, modelName2 := inv.router.Select(phase, tier)
		if p2 != nil {
			bc.Model = modelName2
			bc.Result.Model = modelName2
			modelName = modelName2

			providerResult, err = p2.StreamRun(childCtx, bc.BuildPrompt, tier, inv.output, providerHandler, providerToolHandler)
			p = p2
		}
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

	stallCount, stallTier, ttfe, toolCalls, rateLimitHits, rateLimitRecoveryMs := stats.DiagnosticSnapshot()
	bc.Result.StallCount = stallCount
	bc.Result.StallTier = stallTier
	bc.Result.TimeToFirstEventMs = ttfe.Milliseconds()
	bc.Result.ToolCallCount = toolCalls
	bc.Result.RateLimitHits = rateLimitHits
	bc.Result.RateLimitRecoveryMs = rateLimitRecoveryMs

	return &InvocationResult{
		Result:       claudeResult,
		Stats:        stats,
		StallFired:   stallFired,
		ModelName:    modelName,
		ProviderName: p.Name(),
	}, err
}
