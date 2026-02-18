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

func TestExecuteClaudeInvocation_ReturnsProviderResult(t *testing.T) {
	var buf bytes.Buffer
	expected := &provider.Result{
		Success:      true,
		Output:       "provider output",
		ExitCode:     3,
		Model:        "test-model",
		CostUSD:      9.99,
		InputTokens:  44,
		OutputTokens: 55,
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
	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-1", Title: "Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "prompt",
		ParentCtx:   context.Background(),
	}

	invResult, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invResult == nil {
		t.Fatal("expected non-nil InvocationResult")
	}
	if invResult.Result == nil {
		t.Fatal("expected non-nil invResult.Result")
	}
	if invResult.Stats == nil {
		t.Fatal("expected non-nil invResult.Stats")
	}
	if invResult.StallFired {
		t.Fatal("expected StallFired=false")
	}
	if invResult.ProviderResult == nil {
		t.Fatal("expected non-nil provider result")
	}
	if invResult.ProviderResult != expected {
		t.Fatalf("provider result = %+v, want %+v", invResult.ProviderResult, expected)
	}
}
