//go:build integration

package specloop

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
)

func TestIntegration_AdapterLayer_HappyPath(t *testing.T) {
	// 1. Create a mockIntegrationProvider that returns Output: "plan-result" with CostUSD: 0.05.
	// LLMAdapter.Invoke uses StreamRun, so we wire the response there.
	mock := &mockIntegrationProvider{
		name: "claude",
		streamRunFn: func(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Output: "plan-result", Success: true, CostUSD: 0.05}, nil
		},
	}

	// 2. Create a Config with Phase, Tier, and the two callbacks.
	var gotCost float64
	var invocations []runstore.InvocationRecord

	cfg := llmadapter.Config{
		Phase:   "plan",
		Tier:    "high",
		Timeout: 30 * time.Second,
		OnCost: func(cost float64) {
			gotCost += cost
		},
		OnInvocation: func(record runstore.InvocationRecord) {
			invocations = append(invocations, record)
		},
	}

	// 3. Create the LLMAdapter.
	adapter := llmadapter.New(mock, cfg)

	// 4. Invoke.
	result, err := adapter.Invoke(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 5. Assertions.
	if result.Output != "plan-result" {
		t.Errorf("expected output %q, got %q", "plan-result", result.Output)
	}
	if math.Abs(gotCost-0.05) > 0.001 {
		t.Errorf("expected OnCost called with 0.05, got %f", gotCost)
	}
	if len(invocations) != 1 {
		t.Fatalf("expected 1 invocation record, got %d", len(invocations))
	}
	rec := invocations[0]
	if rec.Phase != "plan" {
		t.Errorf("expected Phase %q, got %q", "plan", rec.Phase)
	}
	if rec.Provider != "claude" {
		t.Errorf("expected Provider %q, got %q", "claude", rec.Provider)
	}
	if math.Abs(rec.CostUSD-0.05) > 0.001 {
		t.Errorf("expected record CostUSD 0.05, got %f", rec.CostUSD)
	}
}

func TestIntegration_FullPipeline_WithFallbackAdapters(t *testing.T) {
	// 1. A single mock provider returning CostUSD: 0.02 per invocation via StreamRun.
	mock := &mockIntegrationProvider{
		name: "claude",
		streamRunFn: func(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Output: "ok", Success: true, CostUSD: 0.02}, nil
		},
	}

	// 2. Build a router with only claude, ratio 100.
	providers := map[string]provider.Provider{"claude": mock}
	preferences := map[string]string{}
	ratio := map[string]int{"claude": 100}
	router := provider.NewRouter(providers, preferences, ratio, 5*time.Minute, nil, nil)

	// 3. Shared accumulators.
	var mu sync.Mutex
	var totalCost float64
	var records []runstore.InvocationRecord

	onCost := func(cost float64) {
		mu.Lock()
		totalCost += cost
		mu.Unlock()
	}
	onInvocation := func(rec runstore.InvocationRecord) {
		mu.Lock()
		records = append(records, rec)
		mu.Unlock()
	}

	// 4. Create 4 FallbackAdapters — one per phase.
	phases := []string{"plan", "execute", "review", "accept"}
	adapters := make([]*llmadapter.FallbackAdapter, len(phases))
	for i, phase := range phases {
		cfg := llmadapter.Config{
			Timeout:      30 * time.Second,
			OnCost:       onCost,
			OnInvocation: onInvocation,
		}
		adapters[i] = llmadapter.NewFallbackAdapter(router, phase, cfg, "high")
	}

	// 5. Invoke each adapter once.
	for i, adapter := range adapters {
		_, err := adapter.Invoke(context.Background(), "dummy prompt")
		if err != nil {
			t.Fatalf("phase %q: unexpected error: %v", phases[i], err)
		}
	}

	// 6. Assertions.
	if math.Abs(totalCost-0.08) > 0.001 {
		t.Errorf("expected totalCost ≈ 0.08 (4 × 0.02), got %f", totalCost)
	}
	if len(records) != 4 {
		t.Fatalf("expected 4 invocation records, got %d", len(records))
	}

	// Verify each record has the correct phase and provider.
	phasesSeen := make(map[string]bool)
	for _, rec := range records {
		phasesSeen[rec.Phase] = true
		if rec.Provider != "claude" {
			t.Errorf("expected Provider %q, got %q", "claude", rec.Provider)
		}
	}
	for _, phase := range phases {
		if !phasesSeen[phase] {
			t.Errorf("expected invocation record for phase %q, not found", phase)
		}
	}
}

func TestIntegration_ValidationFailureTriggersRepair(t *testing.T) {
	t.Skip("TODO: implement validation failure scenario")
}

func TestIntegration_ReviewTriggersReplan(t *testing.T) {
	t.Skip("TODO: implement review replan scenario")
}

func TestIntegration_BudgetExhaustion(t *testing.T) {
	t.Skip("TODO: implement budget exhaustion scenario")
}

func TestIntegration_AcceptanceFailure(t *testing.T) {
	t.Skip("TODO: implement acceptance failure triggers replan scenario")
}

// --- mock provider for integration tests ---

type mockIntegrationProvider struct {
	name         string
	runFn        func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	streamRunFn  func(ctx context.Context, prompt string, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	isUsageLimit bool
}

func (m *mockIntegrationProvider) Name() string                    { return m.name }
func (m *mockIntegrationProvider) ModelForTier(tier string) string { return "mock-" + tier }
func (m *mockIntegrationProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Output: "ok", Success: true}, nil
}
func (m *mockIntegrationProvider) StreamRun(ctx context.Context, prompt string, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, w, handler, onToolCall)
	}
	return &provider.Result{Output: "ok", Success: true}, nil
}
func (m *mockIntegrationProvider) RunValidation(_ context.Context, _ []string, _ string, _ string) (*provider.Result, error) {
	return &provider.Result{Output: "ok", Success: true}, nil
}
func (m *mockIntegrationProvider) IsUsageLimitError(_ *provider.Result, _ error) bool {
	return m.isUsageLimit
}
func (m *mockIntegrationProvider) IsValidationPassed(_ *provider.Result) bool { return true }
func (m *mockIntegrationProvider) IsScopeTooLarge(_ *provider.Result) (bool, string) {
	return false, ""
}

func TestIntegration_FallbackAdapter_UsageLimitFallback_ThroughRouter(t *testing.T) {
	// Primary provider returns usage-limit error; router routes to fallback codex provider.
	primaryProvider := &mockIntegrationProvider{
		name:         "claude",
		isUsageLimit: true,
		streamRunFn: func(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Output: "limit-hit", Success: false}, fmt.Errorf("usage limit exceeded")
		},
	}
	fallbackProvider := &mockIntegrationProvider{
		name: "codex",
		streamRunFn: func(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Output: "codex-ok", Success: true}, nil
		},
	}

	providers := map[string]provider.Provider{
		"claude": primaryProvider,
		"codex":  fallbackProvider,
	}
	preferences := map[string]string{
		"plan": "claude",
	}
	ratio := map[string]int{"claude": 50, "codex": 50}

	router := provider.NewRouter(providers, preferences, ratio, 5*time.Minute, nil, nil)

	cfg := llmadapter.Config{
		Tier:    "high",
		Timeout: 30 * time.Second,
	}
	adapter := llmadapter.NewFallbackAdapter(router, "plan", cfg, "high")

	result, err := adapter.Invoke(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if result.Output != "codex-ok" {
		t.Errorf("expected output %q, got %q", "codex-ok", result.Output)
	}
}

func TestIntegration_ProviderFallbackOnUsageLimit(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: requires simulated usage-limit — manual test")
}

func TestIntegration_RouterPhasePreferences(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: verify correct provider selected per phase preference")
}
