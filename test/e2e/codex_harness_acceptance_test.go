//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestE2EHarness_CodexUsesSharedFixtureAndCallLogHelpers(t *testing.T) {
	// Expected failure: ApplyCodexFixtureEnvE2E and FilterE2EToolCalls do not exist in e2e_test.go yet,
	// so fake Codex is not exposed through the same shared helper surface as other fakes.
	env := setupE2E(t)

	fixture := filepath.Join(fixturesDir, "codex_success.txt")
	env.Env = ApplyCodexFixtureEnvE2E(env.Env, fixture)

	// Run a simple command through the public command surface to ensure harness env wiring is exercised.
	_, _, _, _ = runGromit(env, "status")

	callLog := "bd ready --json --limit 10\n" +
		"codex run --model sonnet\n" +
		"claude -p --model sonnet\n" +
		"codex run --jsonl --model sonnet\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	codexCalls, err := FilterE2EToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(codex) returned error: %v", err)
	}
	if len(codexCalls) != 2 {
		t.Fatalf("expected 2 codex calls, got %d (%v)", len(codexCalls), codexCalls)
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(claude) returned error: %v", err)
	}
	if len(claudeCalls) != 1 {
		t.Fatalf("expected 1 claude call, got %d (%v)", len(claudeCalls), claudeCalls)
	}
}


func TestE2EHarness_FilterToolCallsIgnoresLeadingWhitespace(t *testing.T) {
	env := setupE2E(t)

	callLog := "   codex run --model sonnet\n" +
		"\tclaude -p --model sonnet\n" +
		"bd ready --json --limit 10\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	codexCalls, err := FilterE2EToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(codex) returned error: %v", err)
	}
	if len(codexCalls) != 1 {
		t.Fatalf("expected 1 codex call despite leading whitespace, got %d (%v)", len(codexCalls), codexCalls)
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(claude) returned error: %v", err)
	}
	if len(claudeCalls) != 1 {
		t.Fatalf("expected 1 claude call despite leading whitespace, got %d (%v)", len(claudeCalls), claudeCalls)
	}
}
