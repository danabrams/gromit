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

func TestFilterE2EToolCalls_ByKind(t *testing.T) {
	env := setupE2E(t)

	writeE2ECallLog(t, env,
		"bd ready --json --limit 10",
		"codex run --model sonnet",
		"claude -p --model sonnet",
		"codex run --jsonl --model sonnet",
	)

	cases := []struct {
		tool ToolCallKind
		want int
	}{
		{tool: ToolCallBD, want: 1},
		{tool: ToolCallCodex, want: 2},
		{tool: ToolCallClaude, want: 1},
	}

	for _, tc := range cases {
		calls, err := FilterE2EToolCalls(env, tc.tool)
		if err != nil {
			t.Fatalf("FilterE2EToolCalls(%s) returned error: %v", tc.tool, err)
		}
		if len(calls) != tc.want {
			t.Fatalf("expected %d %s calls, got %d (%v)", tc.want, tc.tool, len(calls), calls)
		}
	}
}
