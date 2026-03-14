//go:build integration

package specloop

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/provider"
)

func TestIntegration_HappyPath(t *testing.T) {
	t.Skip("TODO: implement happy path scenario")
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
func (m *mockIntegrationProvider) StreamRun(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
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
		runFn: func(_ context.Context, _ string, _ string) (*provider.Result, error) {
			return &provider.Result{Output: "limit-hit", Success: false}, fmt.Errorf("usage limit exceeded")
		},
	}
	fallbackProvider := &mockIntegrationProvider{
		name: "codex",
		runFn: func(_ context.Context, _ string, _ string) (*provider.Result, error) {
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
