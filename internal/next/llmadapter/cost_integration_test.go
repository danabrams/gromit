//go:build integration

package llmadapter

import (
	"context"
	"math"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
	"io"
)

// costMockProvider is a minimal mock provider that returns a configurable CostUSD.
type costMockProvider struct {
	name    string
	costUSD float64
}

func (m *costMockProvider) Name() string                    { return m.name }
func (m *costMockProvider) ModelForTier(tier string) string { return "mock-" + tier }
func (m *costMockProvider) Run(_ context.Context, _ string, _ string) (*provider.Result, error) {
	return &provider.Result{Output: "ok", CostUSD: m.costUSD, Success: true}, nil
}
func (m *costMockProvider) StreamRun(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Output: "ok", CostUSD: m.costUSD, Success: true}, nil
}
func (m *costMockProvider) RunValidation(_ context.Context, _ []string, _ string, _ string) (*provider.Result, error) {
	return &provider.Result{Output: "ok", Success: true}, nil
}
func (m *costMockProvider) IsUsageLimitError(_ *provider.Result, _ error) bool { return false }
func (m *costMockProvider) IsValidationPassed(_ *provider.Result) bool         { return true }
func (m *costMockProvider) IsScopeTooLarge(_ *provider.Result) (bool, string)  { return false, "" }

// TestIntegration_CostTracking_AcrossAdapters verifies that the OnCost callback on
// LLMAdapter.Config correctly fires and accumulates costs when 3 adapters are
// invoked sequentially, simulating plan, execute, and review phases.
func TestIntegration_CostTracking_AcrossAdapters(t *testing.T) {
	var totalCost float64
	var invocations []runstore.InvocationRecord

	onCost := func(cost float64) {
		totalCost += cost
	}
	onInvocation := func(record runstore.InvocationRecord) {
		invocations = append(invocations, record)
	}

	planProvider := &costMockProvider{name: "claude", costUSD: 0.04}
	executeProvider := &costMockProvider{name: "claude", costUSD: 0.15}
	reviewProvider := &costMockProvider{name: "claude", costUSD: 0.05}

	planAdapter := New(planProvider, Config{
		Phase:        "plan",
		Tier:         "high",
		OnCost:       onCost,
		OnInvocation: onInvocation,
	})
	executeAdapter := New(executeProvider, Config{
		Phase:        "execute",
		Tier:         "high",
		OnCost:       onCost,
		OnInvocation: onInvocation,
	})
	reviewAdapter := New(reviewProvider, Config{
		Phase:        "review",
		Tier:         "medium",
		OnCost:       onCost,
		OnInvocation: onInvocation,
	})

	ctx := context.Background()

	if _, err := planAdapter.Invoke(ctx, "plan prompt"); err != nil {
		t.Fatalf("plan adapter: unexpected error: %v", err)
	}
	if _, err := executeAdapter.Invoke(ctx, "execute prompt"); err != nil {
		t.Fatalf("execute adapter: unexpected error: %v", err)
	}
	if _, err := reviewAdapter.Invoke(ctx, "review prompt"); err != nil {
		t.Fatalf("review adapter: unexpected error: %v", err)
	}

	const wantTotal = 0.24
	const epsilon = 0.001
	if math.Abs(totalCost-wantTotal) >= epsilon {
		t.Errorf("totalCost = %f, want %f (epsilon %f)", totalCost, wantTotal, epsilon)
	}

	if len(invocations) != 3 {
		t.Fatalf("expected 3 invocation records, got %d", len(invocations))
	}

	cases := []struct {
		phase    string
		provider string
		costUSD  float64
	}{
		{"plan", "claude", 0.04},
		{"execute", "claude", 0.15},
		{"review", "claude", 0.05},
	}
	for i, tc := range cases {
		rec := invocations[i]
		if rec.Phase != tc.phase {
			t.Errorf("invocations[%d].Phase = %q, want %q", i, rec.Phase, tc.phase)
		}
		if rec.Provider != tc.provider {
			t.Errorf("invocations[%d].Provider = %q, want %q", i, rec.Provider, tc.provider)
		}
		if math.Abs(rec.CostUSD-tc.costUSD) >= epsilon {
			t.Errorf("invocations[%d].CostUSD = %f, want %f", i, rec.CostUSD, tc.costUSD)
		}
	}
}
