//go:build acceptance

package contracts

import (
	"os"
	"testing"

	"github.com/danabrams/gromit/test/toolcalls"
)

func TestContractHarness_CodexUsesSharedFixtureAndCallLogHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	callLogPath := tmpDir + "/call.log"

	callLog := "claude -p --model sonnet\n" +
		"codex run --model sonnet\n" +
		"bd ready --json --limit 10\n" +
		"codex run --jsonl --model sonnet\n"
	if err := os.WriteFile(callLogPath, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	codexCalls, err := toolcalls.FilterToolCalls(callLogPath, toolcalls.ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls returned error: %v", err)
	}
	if len(codexCalls) != 2 {
		t.Fatalf("expected 2 codex calls, got %d (%v)", len(codexCalls), codexCalls)
	}
}

func TestContractHarness_SharedFilterKeepsNonCodexBehavior(t *testing.T) {
	tmpDir := t.TempDir()
	callLogPath := tmpDir + "/call.log"

	callLog := "claude -p --model sonnet\n" +
		"bd ready --json --limit 10\n" +
		"git status\n" +
		"bd close test-bead-1\n"
	if err := os.WriteFile(callLogPath, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	claudeCalls, err := toolcalls.FilterToolCalls(callLogPath, toolcalls.ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterToolCalls(claude) returned error: %v", err)
	}
	if len(claudeCalls) != 1 {
		t.Fatalf("expected 1 claude call, got %d (%v)", len(claudeCalls), claudeCalls)
	}

	bdCalls, err := toolcalls.FilterToolCalls(callLogPath, toolcalls.ToolCallBD)
	if err != nil {
		t.Fatalf("FilterToolCalls(bd) returned error: %v", err)
	}
	if len(bdCalls) != 2 {
		t.Fatalf("expected 2 bd calls, got %d (%v)", len(bdCalls), bdCalls)
	}
}
