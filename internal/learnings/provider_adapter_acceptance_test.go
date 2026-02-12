package learnings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/provider"
)

// TestProviderRunnerAdapter_Success verifies that the adapter properly converts a successful provider.Result
// to learnings.Result when wrapping a Provider.
// Expected failure: ProviderRunnerAdapter type does not exist yet
func TestProviderRunnerAdapter_Success(t *testing.T) {
	mockProvider := &mockProviderRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
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
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

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
}

// TestProviderRunnerAdapter_Error verifies that the adapter propagates errors from Provider.Run
// Expected failure: ProviderRunnerAdapter type does not exist yet
func TestProviderRunnerAdapter_Error(t *testing.T) {
	mockProvider := &mockProviderRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return nil, errors.New("provider error")
		},
	}

	adapter := NewProviderRunnerAdapter(mockProvider)
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
	if err.Error() != "provider error" {
		t.Errorf("expected 'provider error', got %q", err.Error())
	}
}

// TestProviderRunnerAdapter_NilProvider verifies that adapter handles nil provider
// Expected failure: ProviderRunnerAdapter type does not exist yet
func TestProviderRunnerAdapter_NilProvider(t *testing.T) {
	adapter := NewProviderRunnerAdapter(nil)
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

	if err == nil {
		t.Fatal("expected error for nil provider")
	}
	if result != nil {
		t.Error("expected nil result on nil provider")
	}
}

// TestProviderRunnerAdapter_NilResult verifies that adapter handles nil provider.Result
// Expected failure: ProviderRunnerAdapter type does not exist yet
func TestProviderRunnerAdapter_NilResult(t *testing.T) {
	mockProvider := &mockProviderRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return nil, nil
		},
	}

	adapter := NewProviderRunnerAdapter(mockProvider)
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

	if err == nil {
		t.Fatal("expected error for nil provider.Result")
	}
	if result != nil {
		t.Error("expected nil result when provider returns nil")
	}
}

// TestNewProviderRunnerAdapter_ReturnsAdapter verifies that the constructor returns a non-nil adapter
// Expected failure: NewProviderRunnerAdapter function does not exist yet
func TestNewProviderRunnerAdapter_ReturnsAdapter(t *testing.T) {
	mockProvider := &mockProviderRunner{}
	adapter := NewProviderRunnerAdapter(mockProvider)

	if adapter == nil {
		t.Fatal("NewProviderRunnerAdapter returned nil")
	}
}

// TestProviderRunnerAdapter_ConvertsTierToModel verifies that the adapter passes the model/tier
// parameter through to the provider unchanged
// Expected failure: ProviderRunnerAdapter type does not exist yet
func TestProviderRunnerAdapter_ConvertsTierToModel(t *testing.T) {
	var capturedTier string
	mockProvider := &mockProviderRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			capturedTier = tier
			return &provider.Result{Success: true, Output: "ok"}, nil
		},
	}

	adapter := NewProviderRunnerAdapter(mockProvider)
	_, err := adapter.Run(context.Background(), "test prompt", "sonnet")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedTier != "sonnet" {
		t.Errorf("expected tier='sonnet' to be passed to provider, got %q", capturedTier)
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
