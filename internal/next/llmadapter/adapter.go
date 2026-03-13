package llmadapter

import (
	"context"
	"io"
	"time"

	"github.com/danabrams/gromit/internal/provider"
)

// Config configures an LLMAdapter instance.
type Config struct {
	Tier    string
	Timeout time.Duration
	OnCost  func(cost float64)
}

// LLMAdapter wraps a provider.Provider with timeout enforcement and cost tracking.
// It is the shared base for all per-domain adapters.
type LLMAdapter struct {
	provider provider.Provider
	cfg      Config
}

// New creates an LLMAdapter.
func New(p provider.Provider, cfg Config) *LLMAdapter {
	return &LLMAdapter{provider: p, cfg: cfg}
}

// Invoke calls provider.Run with the configured tier.
// Returns the result even on error for 0002d FallbackAdapter compatibility.
func (a *LLMAdapter) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	if a.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.Timeout)
		defer cancel()
	}

	result, err := a.provider.Run(ctx, prompt, a.cfg.Tier)
	if err != nil {
		return result, err
	}

	if a.cfg.OnCost != nil && result.CostUSD > 0 {
		a.cfg.OnCost(result.CostUSD)
	}

	return result, nil
}

// InvokeStream calls provider.StreamRun with the configured tier.
func (a *LLMAdapter) InvokeStream(ctx context.Context, prompt string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if a.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.Timeout)
		defer cancel()
	}

	result, err := a.provider.StreamRun(ctx, prompt, a.cfg.Tier, w, handler, onToolCall)
	if err != nil {
		return result, err
	}

	if a.cfg.OnCost != nil && result.CostUSD > 0 {
		a.cfg.OnCost(result.CostUSD)
	}

	return result, nil
}

// ProviderName returns the name of the underlying provider.
func (a *LLMAdapter) ProviderName() string {
	return a.provider.Name()
}

// Tier returns the configured tier.
func (a *LLMAdapter) Tier() string {
	return a.cfg.Tier
}
