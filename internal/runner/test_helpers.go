package runner

import (
	"context"
	"io"

	"github.com/danabrams/gromit/internal/provider"
)

// createMockRouter creates a mock router for tests that don't need specific behavior
func createMockRouter() *provider.Router {
	// Create a simple mock provider
	mockProv := &simpleMockProvider{}

	// Create a single-provider router wrapping the mock
	return provider.NewSingleProviderRouter(mockProv)
}

// simpleMockProvider is a minimal provider implementation for tests
type simpleMockProvider struct{}

func (m *simpleMockProvider) Name() string {
	return "mock"
}

func (m *simpleMockProvider) ModelForTier(tier string) string {
	return tier
}

func (m *simpleMockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Output: "mock output"}, nil
}

func (m *simpleMockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}

func (m *simpleMockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}

func (m *simpleMockProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
