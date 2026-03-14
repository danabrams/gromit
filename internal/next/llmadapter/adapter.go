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
// Panics if Tier is empty (construction-time fail-fast, consistent with
// ShellValidator's nil-runner panic). Clamps negative Timeout to 0 (no timeout).
func New(p provider.Provider, cfg Config) *LLMAdapter {
	if cfg.Tier == "" {
		panic("llmadapter: Tier must be non-empty")
	}
	if cfg.Timeout < 0 {
		cfg.Timeout = 0
	}
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

	// Track cost even on error — partial results may still incur charges.
	if a.cfg.OnCost != nil && result != nil && result.CostUSD > 0 {
		a.cfg.OnCost(result.CostUSD)
	}

	return result, err
}

// InvokeInDir calls the provider with a working directory. If the provider
// implements provider.DirStreamRunner, it uses StreamRunInDir; otherwise it
// falls back to provider.Run (ignoring the directory).
func (a *LLMAdapter) InvokeInDir(ctx context.Context, prompt string, dir string) (*provider.Result, error) {
	if a.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.Timeout)
		defer cancel()
	}

	var result *provider.Result
	var err error
	if dsr, ok := a.provider.(provider.DirStreamRunner); ok {
		result, err = dsr.StreamRunInDir(ctx, prompt, a.cfg.Tier, dir, io.Discard, nil, nil)
	} else {
		result, err = a.provider.Run(ctx, prompt, a.cfg.Tier)
	}

	// Track cost even on error — partial results may still incur charges.
	if a.cfg.OnCost != nil && result != nil && result.CostUSD > 0 {
		a.cfg.OnCost(result.CostUSD)
	}

	return result, err
}

// InvokeStream calls provider.StreamRun with the configured tier.
func (a *LLMAdapter) InvokeStream(ctx context.Context, prompt string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if a.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.Timeout)
		defer cancel()
	}

	result, err := a.provider.StreamRun(ctx, prompt, a.cfg.Tier, w, handler, onToolCall)

	// Track cost even on error — partial results may still incur charges.
	if a.cfg.OnCost != nil && result != nil && result.CostUSD > 0 {
		a.cfg.OnCost(result.CostUSD)
	}

	return result, err
}

// ProviderName returns the name of the underlying provider.
func (a *LLMAdapter) ProviderName() string {
	return a.provider.Name()
}

// Tier returns the configured tier.
func (a *LLMAdapter) Tier() string {
	return a.cfg.Tier
}

// Provider returns the underlying provider. This allows LLMAdapter to
// satisfy ProviderAwareInvoker directly.
func (a *LLMAdapter) Provider() provider.Provider {
	return a.provider
}

var _ Invoker = (*LLMAdapter)(nil)
var _ ProviderAwareInvoker = (*LLMAdapter)(nil)
