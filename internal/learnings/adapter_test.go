package learnings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/claude"
)

// TestClaudeRunnerAdapter_Success tests that the adapter properly converts a successful result
func TestClaudeRunnerAdapter_Success(t *testing.T) {
	mockClient := &mockClaudeClientRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success:  true,
				Output:   "test output",
				ExitCode: 0,
				Duration: time.Second,
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
}

// TestClaudeRunnerAdapter_Error tests that the adapter propagates errors
func TestClaudeRunnerAdapter_Error(t *testing.T) {
	mockClient := &mockClaudeClientRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
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
}

// TestClaudeRunnerAdapter_NilResult tests that adapter handles nil claude.Result
func TestClaudeRunnerAdapter_NilResult(t *testing.T) {
	mockClient := &mockClaudeClientRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return nil, nil
		},
	}

	adapter := NewClaudeRunnerAdapter(mockClient)
	result, err := adapter.Run(context.Background(), "test prompt", "haiku")

	if err == nil {
		t.Fatal("expected error for nil claude.Result")
	}
	if result != nil {
		t.Error("expected nil result when claude returns nil")
	}
}

// mockClaudeClientRunner implements a mock for claudeRunnerAdapter's ClaudeClientRunner dependency
type mockClaudeClientRunner struct {
	FnRun func(ctx context.Context, prompt string, model string) (*claude.Result, error)
}

func (m *mockClaudeClientRunner) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	if m.FnRun != nil {
		return m.FnRun(ctx, prompt, model)
	}
	return &claude.Result{Success: true}, nil
}
