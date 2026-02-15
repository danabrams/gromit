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

// Expected failure: executeClaudeInvocation does not return provider.Result yet
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

	claudeResult, stats, providerResult, stallFired, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claudeResult == nil {
		t.Fatal("expected non-nil claude result")
	}
	if stats == nil {
		t.Fatal("expected non-nil stream stats")
	}
	if stallFired {
		t.Fatal("expected stallFired=false")
	}
	if providerResult == nil {
		t.Fatal("expected non-nil provider result")
	}
	if providerResult != expected {
		t.Fatalf("provider result = %+v, want %+v", providerResult, expected)
	}
}
