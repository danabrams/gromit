package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func setupInvokeFnWithProvider(t *testing.T, streamRunFn func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)) (escalation.InvokeFn, *runtypes.BeadContext) {
	t.Helper()

	var buf bytes.Buffer
	mockProvider := &mockProviderWithRouterTracking{
		streamRunFn: streamRunFn,
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg:     &config.Config{},
		router:  mockRouter,
		invoker: newInvokerForTest(mockRouter, &buf, nil),
		output:  &buf,
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-2", Title: "Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "prompt",
		ParentCtx:   context.Background(),
	}

	return r.makeInvokeFn(), bc
}

// Expected failure: escalation.InvocationResult.ProviderResult does not exist yet
func TestMakeInvokeFn_PropagatesProviderResult(t *testing.T) {
	expected := &provider.Result{
		Success:      true,
		Output:       "provider output",
		ExitCode:     0,
		Model:        "test-model",
		CostUSD:      1.5,
		InputTokens:  12,
		OutputTokens: 34,
	}

	invokeFn, bc := setupInvokeFnWithProvider(t, func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return expected, nil
	})

	result, err := invokeFn(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil invocation result")
	}
	if result.ProviderResult == nil {
		t.Fatal("expected ProviderResult to be set")
	}
	if result.ProviderResult != expected {
		t.Fatalf("ProviderResult = %+v, want %+v", result.ProviderResult, expected)
	}
}

// Expected failure: escalation.InvocationResult.ProviderResult does not exist yet
func TestMakeInvokeFn_PropagatesProviderResult_OnInvocationError(t *testing.T) {
	expected := &provider.Result{
		Success:      false,
		Output:       "provider output",
		ExitCode:     2,
		Model:        "test-model",
		CostUSD:      0.1,
		InputTokens:  5,
		OutputTokens: 7,
	}

	invokeFn, bc := setupInvokeFnWithProvider(t, func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return expected, errors.New("boom")
	})

	result, err := invokeFn(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error for invocation failure")
	}
	if result == nil {
		t.Fatal("expected non-nil invocation result")
	}
	if result.TimeoutType != "invocation" {
		t.Fatalf("TimeoutType = %q, want %q", result.TimeoutType, "invocation")
	}
	if result.ProviderResult == nil {
		t.Fatal("expected ProviderResult to be set on failure")
	}
	if result.ProviderResult != expected {
		t.Fatalf("ProviderResult = %+v, want %+v", result.ProviderResult, expected)
	}
}

func TestMakeInvokeFn_DeadlineExceededSetsInvocationTimeout(t *testing.T) {
	invokeFn, bc := setupInvokeFnWithProvider(t, func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return &provider.Result{Success: false, Model: "test-model"}, context.DeadlineExceeded
	})

	result, err := invokeFn(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error for invocation deadline")
	}
	if result == nil {
		t.Fatal("expected non-nil invocation result")
	}
	if result.TimeoutType != "invocation" {
		t.Fatalf("TimeoutType = %q, want %q", result.TimeoutType, "invocation")
	}
	if bc.Result.TimeoutType != "invocation" {
		t.Fatalf("bc.Result.TimeoutType = %q, want %q", bc.Result.TimeoutType, "invocation")
	}
}
