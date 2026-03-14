//go:build llmcontract

package llmadapter

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/provider"
)

// ContractInvoker creates an Invoker backed by a real Claude provider for contract tests.
// It uses the cheapest tier (haiku) with a 2-minute timeout.
func ContractInvoker(t *testing.T) Invoker {
	t.Helper()
	client, err := claude.NewClient("claude", []string{"--no-input"}, 120)
	if err != nil {
		t.Fatalf("failed to create claude client: %v", err)
	}
	// NOTE: These model version strings need periodic updates as new model
	// versions are released by Anthropic.
	prov := provider.NewClaudeProvider(client, map[string]string{
		"low":    "claude-haiku-4-5-20251001",
		"medium": "claude-sonnet-4-5-20250514",
		"high":   "claude-sonnet-4-5-20250514",
	})
	return New(prov, Config{
		Tier:    "low",
		Timeout: 2 * time.Minute,
	})
}
