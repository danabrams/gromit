//go:build llmcontract

package llmadapter

import (
	"os/exec"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/provider"
)

// ContractClaudeInvoker creates an Invoker backed by a real Claude provider for contract tests.
// It uses the cheapest tier (haiku) with a 2-minute timeout.
func ContractClaudeInvoker(t *testing.T) Invoker {
	t.Helper()
	client, err := claude.NewClient("claude", []string{"--dangerously-skip-permissions"}, 120)
	if err != nil {
		t.Fatalf("failed to create claude client: %v", err)
	}
	prov := provider.NewClaudeProvider(client, map[string]string{
		"low":    "haiku",
		"medium": "sonnet",
		"high":   "sonnet",
	})
	return New(prov, Config{
		Tier:    "low",
		Timeout: 2 * time.Minute,
	})
}

// ContractInvoker is an alias for ContractClaudeInvoker for backwards compatibility.
func ContractInvoker(t *testing.T) Invoker {
	return ContractClaudeInvoker(t)
}

// ContractCodexInvoker creates an Invoker backed by a real Codex provider for contract tests.
// It skips the test if the codex binary is not found on $PATH.
func ContractCodexInvoker(t *testing.T) Invoker {
	t.Helper()
	codexPath, err := exec.LookPath("codex")
	if err != nil {
		t.Skip("codex binary not found on $PATH; skipping Codex contract test")
	}
	// NOTE: These model version strings need periodic updates as new model
	// versions are released by OpenAI. gpt-5.4 is the ChatGPT-account-compatible
	// model; o4-mini/o3 require API-key access that may not be available.
	prov := provider.NewCodexProvider(codexPath, []string{}, map[string]string{
		"low":    "gpt-5.1-codex-mini",
		"medium": "gpt-5.4",
		"high":   "gpt-5.4",
	})
	// Reasoning effort scales with tier. "xhigh" causes timeouts; avoid it.
	prov.SetReasoningEffort(map[string]string{
		"low":    "high",
		"medium": "medium",
		"high":   "high",
	})
	// Codex does not emit total_cost_usd for ChatGPT accounts; compute from tokens.
	// Pricing as of 2026-03:
	//   gpt-5.1-codex-mini = $0.25/1M input, $2.00/1M output
	//   gpt-5.4            = $2.50/1M input, $15.00/1M output
	prov.SetModelPricing(map[string]provider.ModelPricing{
		"gpt-5.1-codex-mini": {InputCostPerMillion: 0.25, OutputCostPerMillion: 2.00},
		"gpt-5.4":            {InputCostPerMillion: 2.50, OutputCostPerMillion: 15.00},
	})
	return New(prov, Config{
		Tier:    "low",
		Timeout: 2 * time.Minute,
	})
}
