package runner

import (
	"context"
	"io"

	"github.com/danabrams/gromit/internal/provider"
)

// mockProviderWithRouterTracking is a mock provider for router tracking tests
// Used in both unit tests and acceptance tests
type mockProviderWithRouterTracking struct {
	name            string
	onSelect        func(phase, tier string)
	runFn           func(ctx context.Context, prompt, tier string) (*provider.Result, error)
	streamRunResult *provider.Result
}

func (m *mockProviderWithRouterTracking) Name() string {
	if m.name != "" {
		return m.name
	}
	return "test-provider"
}

func (m *mockProviderWithRouterTracking) ModelForTier(tier string) string {
	switch tier {
	case provider.TierHigh:
		return "test-opus"
	case provider.TierMedium:
		return "test-sonnet"
	case provider.TierLow:
		return "test-haiku"
	default:
		return "test-model"
	}
}

func (m *mockProviderWithRouterTracking) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.onSelect != nil {
		// Track the Select() call indirectly through Run()
		m.onSelect("", tier)
	}
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProviderWithRouterTracking) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunResult != nil {
		return m.streamRunResult, nil
	}
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProviderWithRouterTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "test-model", Output: "VALIDATION_PASSED"}, nil
}

func (m *mockProviderWithRouterTracking) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

// mockRouterWithTracking wraps a provider.Router and tracks Select() calls
type mockRouterWithTracking struct {
	inner    *provider.Router
	onSelect func(phase, tier string)
}

func newMockRouterWithTracking(inner *provider.Router, onSelect func(phase, tier string)) *provider.Router {
	// We can't actually wrap the router because it's a struct, not an interface.
	// Instead, we need to intercept at the provider level.
	// Return the inner router and let the provider handle tracking.
	return inner
}
