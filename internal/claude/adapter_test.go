package claude

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestClaudeRunnerAdapter_NilReceiver tests that adapter checks for nil receiver
func TestClaudeRunnerAdapter_NilReceiver(t *testing.T) {
	var adapter *ClaudeRunnerAdapter
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

	if err == nil {
		t.Fatal("expected error for nil receiver")
	}
	if result != nil {
		t.Error("expected nil result on nil receiver")
	}
	if err.Error() != "adapter is nil" {
		t.Errorf("expected 'adapter is nil', got %q", err.Error())
	}
}

// TestClaudeRunnerAdapter_NilClient tests that adapter handles nil client
func TestClaudeRunnerAdapter_NilClient(t *testing.T) {
	adapter := NewClaudeRunnerAdapter(nil)
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if result != nil {
		t.Error("expected nil result on nil client")
	}
	if err.Error() != "claude client is nil" {
		t.Errorf("expected 'claude client is nil', got %q", err.Error())
	}
}

// TestClaudeRunnerAdapter_NilResult tests that adapter handles nil Result from client
func TestClaudeRunnerAdapter_NilResult(t *testing.T) {
	mockClient := &mockClient{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return nil, nil // Return nil result with no error
		},
	}

	adapter := NewClaudeRunnerAdapter(mockClient)
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

	if err == nil {
		t.Fatal("expected error for nil result")
	}
	if result != nil {
		t.Error("expected nil result when client returns nil")
	}
	if err.Error() != "claude returned nil result" {
		t.Errorf("expected 'claude returned nil result', got %q", err.Error())
	}
}

// TestClaudeRunnerAdapter_Error tests that adapter propagates errors
func TestClaudeRunnerAdapter_Error(t *testing.T) {
	mockClient := &mockClient{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return nil, errors.New("test error")
		},
	}

	adapter := NewClaudeRunnerAdapter(mockClient)
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %q", err.Error())
	}
}

// TestClaudeRunnerAdapter_Success tests successful result conversion
func TestClaudeRunnerAdapter_Success(t *testing.T) {
	mockClient := &mockClient{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return &Result{
				Success:  true,
				Output:   "test output",
				ExitCode: 0,
				Duration: time.Second,
				Model:    "haiku",
			}, nil
		},
	}

	adapter := NewClaudeRunnerAdapter(mockClient)
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
	if result.Output != "test output" {
		t.Errorf("expected output='test output', got %q", result.Output)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Duration != time.Second {
		t.Errorf("expected duration 1s, got %v", result.Duration)
	}
	if result.Model != "haiku" {
		t.Errorf("expected model 'haiku', got %q", result.Model)
	}
}

// TestClaudeRunnerAdapter_FailedResult tests failed result conversion
func TestClaudeRunnerAdapter_FailedResult(t *testing.T) {
	mockClient := &mockClient{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return &Result{
				Success:  false,
				Output:   "error message",
				ExitCode: 1,
				Duration: 2 * time.Second,
				Model:    "haiku",
			}, nil
		},
	}

	adapter := NewClaudeRunnerAdapter(mockClient)
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Success {
		t.Error("expected success=false")
	}
	if result.Output != "error message" {
		t.Errorf("expected output='error message', got %q", result.Output)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

// mockClient implements a mock Client for testing
type mockClient struct {
	FnRun func(ctx context.Context, prompt string, model string) (*Result, error)
}

func (m *mockClient) Run(ctx context.Context, prompt string, model string) (*Result, error) {
	if m.FnRun != nil {
		return m.FnRun(ctx, prompt, model)
	}
	return &Result{Success: true}, nil
}
