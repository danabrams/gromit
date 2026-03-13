package llmadapter

import (
	"context"

	"github.com/danabrams/gromit/internal/provider"
)

// Invoker is the interface domain adapters depend on.
// LLMAdapter satisfies this, and tests can substitute mocks.
type Invoker interface {
	Invoke(ctx context.Context, prompt string) (*provider.Result, error)
}

// ProviderAwareInvoker extends Invoker with access to the underlying provider.
// Needed by 0002d's FallbackAdapter to inspect/route by provider.
type ProviderAwareInvoker interface {
	Invoker
	Provider() provider.Provider
}

// ProviderAware wraps an Invoker and a Provider to satisfy ProviderAwareInvoker.
type ProviderAware struct {
	Invoker
	prov provider.Provider
}

// NewProviderAware creates a ProviderAware wrapper.
func NewProviderAware(inv Invoker, prov provider.Provider) *ProviderAware {
	return &ProviderAware{Invoker: inv, prov: prov}
}

// Provider returns the underlying provider.
func (pa *ProviderAware) Provider() provider.Provider {
	return pa.prov
}
