package toolcalls

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilterToolCalls_Codex(t *testing.T) {
	// Create a temporary call log file
	tmpDir := t.TempDir()
	callLogPath := filepath.Join(tmpDir, "call_log.txt")

	callLog := "claude -p --model sonnet\n" +
		"codex run --model sonnet\n"
	if err := os.WriteFile(callLogPath, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	calls, err := FilterToolCalls(callLogPath, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls returned error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 codex call, got %d (%v)", len(calls), calls)
	}
	if calls[0] != "codex run --model sonnet" {
		t.Fatalf("expected call 'codex run --model sonnet', got %q", calls[0])
	}
}

func TestFilterToolCalls_UnknownKind(t *testing.T) {
	tmpDir := t.TempDir()
	callLogPath := filepath.Join(tmpDir, "call_log.txt")
	if err := os.WriteFile(callLogPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	_, err := FilterToolCalls(callLogPath, ToolCallKind("unknown"))
	if err == nil {
		t.Fatal("expected an error for unknown tool kind")
	}
}

func TestFilterToolCalls_MultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	callLogPath := filepath.Join(tmpDir, "call_log.txt")

	callLog := "bd ready --json --limit 10\n" +
		"codex run --model sonnet\n" +
		"claude -p --model sonnet\n" +
		"codex run --jsonl --model sonnet\n"
	if err := os.WriteFile(callLogPath, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	calls, err := FilterToolCalls(callLogPath, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls returned error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 codex calls, got %d (%v)", len(calls), calls)
	}
}
