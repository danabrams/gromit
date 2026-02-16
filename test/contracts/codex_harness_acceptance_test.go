//go:build contract

package contracts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContractHarness_CodexUsesSharedFixtureAndCallLogHelpers(t *testing.T) {
	// Expected failure: ApplyCodexFixtureEnv and FilterToolCalls do not exist in the shared contract harness yet,
	// so fake Codex cannot be configured/filterable the same way as fake Claude/Git/BD.
	env := setupTestEnv(t)

	fixture := filepath.Join(fixturesDir, "codex_success.txt")
	env.Env = ApplyCodexFixtureEnv(env.Env, fixture)

	callLog := "claude -p --model sonnet\n" +
		"codex run --model sonnet\n" +
		"bd ready --json --limit 10\n" +
		"codex run --jsonl --model sonnet\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	codexCalls, err := FilterToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls returned error: %v", err)
	}
	if len(codexCalls) != 2 {
		t.Fatalf("expected 2 codex calls, got %d (%v)", len(codexCalls), codexCalls)
	}
}

func TestContractHarness_SharedFilterKeepsNonCodexBehavior(t *testing.T) {
	// Expected failure: FilterToolCalls and ToolCallClaude/ToolCallBD do not exist yet,
	// so non-Codex tests are not yet backed by the new shared filtering API.
	env := setupTestEnv(t)

	callLog := "claude -p --model sonnet\n" +
		"bd ready --json --limit 10\n" +
		"git status\n" +
		"bd close test-bead-1\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	claudeCalls, err := FilterToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterToolCalls(claude) returned error: %v", err)
	}
	if len(claudeCalls) != 1 {
		t.Fatalf("expected 1 claude call, got %d (%v)", len(claudeCalls), claudeCalls)
	}

	bdCalls, err := FilterToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterToolCalls(bd) returned error: %v", err)
	}
	if len(bdCalls) != 2 {
		t.Fatalf("expected 2 bd calls, got %d (%v)", len(bdCalls), bdCalls)
	}
}
