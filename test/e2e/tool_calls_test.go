//go:build e2e

package e2e

import (
	"os"
	"testing"
)

func TestFilterE2EToolCalls_Codex(t *testing.T) {
	env := setupE2E(t)

	callLog := "bd ready --json --limit 10\n" +
		"codex run --model sonnet\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	calls, err := FilterE2EToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls returned error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 codex call, got %d (%v)", len(calls), calls)
	}
}
