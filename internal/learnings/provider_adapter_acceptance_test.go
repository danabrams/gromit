//go:build acceptance

package learnings

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/provider"
)

// TestProviderRunnerAdapter verifies that the adapter properly converts provider.Result
// to learnings.Result and passes tier parameter correctly.
func TestProviderRunnerAdapter(t *testing.T) {
	var capturedTier string
	mockProvider := &mockProviderRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			capturedTier = tier
			return &provider.Result{
				Success:  true,
				Output:   "test output from provider",
				ExitCode: 0,
				Duration: time.Second,
				Model:    "opus",
			}, nil
		},
	}

	adapter := NewProviderRunnerAdapter(mockProvider)
	if adapter == nil {
		t.Fatal("NewProviderRunnerAdapter returned nil")
	}

	result, err := adapter.Run(context.Background(), "test prompt", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !result.Success {
		t.Error("expected success=true")
	}
	if result.Output != "test output from provider" {
		t.Errorf("expected output='test output from provider', got %q", result.Output)
	}
	if capturedTier != "low" {
		t.Errorf("expected tier='low' to be passed to provider, got %q", capturedTier)
	}
}

// mockProviderRunner implements a mock for testing ProviderRunnerAdapter
type mockProviderRunner struct {
	FnRun func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
}

func (m *mockProviderRunner) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.FnRun != nil {
		return m.FnRun(ctx, prompt, tier)
	}
	return &provider.Result{Success: true}, nil
}
