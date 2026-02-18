//go:build contract

package contracts

import (
	"os"
	"testing"
)

func TestFilterToolCalls_Codex(t *testing.T) {
	env := setupTestEnv(t)

	callLog := "claude -p --model sonnet\n" +
		"codex run --model sonnet\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	calls, err := FilterToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls returned error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 codex call, got %d (%v)", len(calls), calls)
	}
}
