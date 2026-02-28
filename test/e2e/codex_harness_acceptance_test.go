//go:build acceptance

package e2e

import (
	"os"
	"testing"

	"github.com/danabrams/gromit/test/toolcalls"
)

func TestE2EHarness_CodexUsesSharedFixtureAndCallLogHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	callLogPath := tmpDir + "/call.log"

	callLog := "bd ready --json --limit 10\n" +
		"codex run --model sonnet\n" +
		"claude -p --model sonnet\n" +
		"codex run --jsonl --model sonnet\n"
	if err := os.WriteFile(callLogPath, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	codexCalls, err := toolcalls.FilterToolCalls(callLogPath, toolcalls.ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls(codex) returned error: %v", err)
	}
	if len(codexCalls) != 2 {
		t.Fatalf("expected 2 codex calls, got %d (%v)", len(codexCalls), codexCalls)
	}

	claudeCalls, err := toolcalls.FilterToolCalls(callLogPath, toolcalls.ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterToolCalls(claude) returned error: %v", err)
	}
	if len(claudeCalls) != 1 {
		t.Fatalf("expected 1 claude call, got %d (%v)", len(claudeCalls), claudeCalls)
	}
}

func TestE2EHarness_FilterToolCallsIgnoresLeadingWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	callLogPath := tmpDir + "/call.log"

	callLog := "   codex run --model sonnet\n" +
		"\tclaude -p --model sonnet\n" +
		"bd ready --json --limit 10\n"
	if err := os.WriteFile(callLogPath, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	codexCalls, err := toolcalls.FilterToolCalls(callLogPath, toolcalls.ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls(codex) returned error: %v", err)
	}
	if len(codexCalls) != 1 {
		t.Fatalf("expected 1 codex call despite leading whitespace, got %d (%v)", len(codexCalls), codexCalls)
	}

	claudeCalls, err := toolcalls.FilterToolCalls(callLogPath, toolcalls.ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterToolCalls(claude) returned error: %v", err)
	}
	if len(claudeCalls) != 1 {
		t.Fatalf("expected 1 claude call despite leading whitespace, got %d (%v)", len(claudeCalls), claudeCalls)
	}
}
