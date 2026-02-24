package runner

import (
	"math"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

func TestWorktreeMergerAdapterPendingBranches_NilManagerReturnsError(t *testing.T) {
	adapter := &worktreeMergerAdapter{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PendingBranches() panicked with nil manager: %v", r)
		}
	}()

	_, err := adapter.PendingBranches()
	if err == nil {
		t.Fatal("expected error for nil worktree manager")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "worktree manager") {
		t.Fatalf("error = %v, want message mentioning worktree manager", err)
	}
}

func TestApplyCostFallback_EstimatesFromProviderPricing(t *testing.T) {
	result := &provider.Result{
		Model:        "gpt-5.3-codex",
		CostUSD:      0,
		InputTokens:  1000,
		OutputTokens: 500,
	}
	defs := map[string]config.ProviderDef{
		"codex": {
			CostPer1kInput:  0.001,
			CostPer1kOutput: 0.002,
			ModelCosts: map[string]*config.ModelCost{
				"gpt-5.3-codex": {
					CostPer1kInput:  0.003,
					CostPer1kOutput: 0.012,
				},
			},
		},
	}

	applyCostFallback(result, "codex", defs)

	const want = 0.009
	if math.Abs(result.CostUSD-want) > 1e-12 {
		t.Fatalf("CostUSD = %v, want %v", result.CostUSD, want)
	}
}

func TestApplyCostFallback_DoesNotOverrideProviderReportedCost(t *testing.T) {
	result := &provider.Result{
		Model:        "gpt-5.3-codex",
		CostUSD:      0.25,
		InputTokens:  1000,
		OutputTokens: 500,
	}
	defs := map[string]config.ProviderDef{
		"codex": {
			CostPer1kInput:  1,
			CostPer1kOutput: 1,
		},
	}

	applyCostFallback(result, "codex", defs)

	if result.CostUSD != 0.25 {
		t.Fatalf("CostUSD = %v, want unchanged 0.25", result.CostUSD)
	}
}

func TestApplyCostFallback_NoProviderDefLeavesZero(t *testing.T) {
	result := &provider.Result{
		Model:        "gpt-5.3-codex",
		CostUSD:      0,
		InputTokens:  1000,
		OutputTokens: 500,
	}

	applyCostFallback(result, "codex", map[string]config.ProviderDef{})

	if result.CostUSD != 0 {
		t.Fatalf("CostUSD = %v, want 0", result.CostUSD)
	}
}
