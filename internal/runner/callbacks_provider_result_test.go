package runner

import (
	"bytes"
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// Expected failure: escalation.InvocationResult.ProviderResult does not exist yet
func TestMakeInvokeFn_PropagatesProviderResult(t *testing.T) {
	var buf bytes.Buffer
	expected := &provider.Result{
		Success:      true,
		Output:       "provider output",
		ExitCode:     0,
		Model:        "test-model",
		CostUSD:      1.5,
		InputTokens:  12,
		OutputTokens: 34,
	}
	mockProvider := &mockProviderWithRouterTracking{
		streamRunResult: expected,
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg:     &config.Config{},
		router:  mockRouter,
		invoker: newInvokerForTest(mockRouter, &buf, nil),
		output:  &buf,
	}
	invokeFn := r.makeInvokeFn()

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-2", Title: "Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "prompt",
		ParentCtx:   context.Background(),
	}

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
