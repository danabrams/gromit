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
	// versions are released by OpenAI.
	prov := provider.NewCodexProvider(codexPath, []string{}, map[string]string{
		"low":    "o4-mini",
		"medium": "o3",
		"high":   "o3",
	})
	return New(prov, Config{
		Tier:    "low",
		Timeout: 2 * time.Minute,
	})
}
