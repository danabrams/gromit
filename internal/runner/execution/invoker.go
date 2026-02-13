package execution

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

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
	router       Router
	output       io.Writer
	streamLogger *logger.StreamLogger
}

// NewInvoker creates an Invoker with the given narrow dependencies.
func NewInvoker(router Router, output io.Writer, streamLogger *logger.StreamLogger) *Invoker {
	return &Invoker{
		router:       router,
		output:       output,
		streamLogger: streamLogger,
	}
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

	var handler claude.EventHandler
	if inv.streamLogger != nil {
		sl := inv.streamLogger
		handler = func(line []byte) {
			logger.ParseAndLogEvent(sl, stats, line)
		}
	}

	var providerHandler provider.EventHandler
	if handler != nil {
		providerHandler = provider.EventHandler(handler)
	}

	providerToolHandler := func(event provider.ToolEvent) {
		// Tool call events can be captured for heartbeat display;
		// for now we record them in stats.
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
