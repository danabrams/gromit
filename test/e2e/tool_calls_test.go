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

	if err := writeE2ECallLog(env,
		"bd ready --json --limit 10",
		"codex run --model sonnet",
		"claude -p --model sonnet",
		"codex run --jsonl --model sonnet",
	); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

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

func TestE2E_WriteAndFilterCallLog_Integration(t *testing.T) {
	env := setupE2E(t)

	initialCalls := []string{
		"bd ready --json --limit 10",
		"claude -p --model haiku",
		"codex run --model sonnet",
	}

	if err := writeE2ECallLog(env, initialCalls...); err != nil {
		t.Fatalf("writeE2ECallLog failed: %v", err)
	}

	bdCalls, err := FilterE2EToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(BD) failed: %v", err)
	}
	if len(bdCalls) != 1 {
		t.Errorf("expected 1 BD call, got %d: %v", len(bdCalls), bdCalls)
	}
	if len(bdCalls) > 0 && bdCalls[0] != "bd ready --json --limit 10" {
		t.Errorf("expected 'bd ready --json --limit 10', got %q", bdCalls[0])
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Claude) failed: %v", err)
	}
	if len(claudeCalls) != 1 {
		t.Errorf("expected 1 Claude call, got %d: %v", len(claudeCalls), claudeCalls)
	}
	if len(claudeCalls) > 0 && claudeCalls[0] != "claude -p --model haiku" {
		t.Errorf("expected 'claude -p --model haiku', got %q", claudeCalls[0])
	}

	codexCalls, err := FilterE2EToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Codex) failed: %v", err)
	}
	if len(codexCalls) != 1 {
		t.Errorf("expected 1 Codex call, got %d: %v", len(codexCalls), codexCalls)
	}
	if len(codexCalls) > 0 && codexCalls[0] != "codex run --model sonnet" {
		t.Errorf("expected 'codex run --model sonnet', got %q", codexCalls[0])
	}
}

func TestE2E_SequentialWriteAndFilter_Interleaved(t *testing.T) {
	env := setupE2E(t)

	firstBatch := []string{
		"bd ready --json --limit 10",
		"claude -p --model haiku",
	}
	if err := writeE2ECallLog(env, firstBatch...); err != nil {
		t.Fatalf("first writeE2ECallLog failed: %v", err)
	}

	secondBatch := []string{
		"codex run --model sonnet",
		"bd close test-id",
		"claude -p --model sonnet",
	}
	if err := writeE2ECallLog(env, secondBatch...); err != nil {
		t.Fatalf("second writeE2ECallLog failed: %v", err)
	}

	bdCalls, err := FilterE2EToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(BD) failed: %v", err)
	}
	if len(bdCalls) != 1 {
		t.Errorf("expected 1 BD call after second write (overwrites first), got %d: %v", len(bdCalls), bdCalls)
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Claude) failed: %v", err)
	}
	if len(claudeCalls) != 1 {
		t.Errorf("expected 1 Claude call after second write (overwrites first), got %d: %v", len(claudeCalls), claudeCalls)
	}

	codexCalls, err := FilterE2EToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Codex) failed: %v", err)
	}
	if len(codexCalls) != 1 {
		t.Errorf("expected 1 Codex call from second batch, got %d: %v", len(codexCalls), codexCalls)
	}
}

func TestE2E_FilterEmptyCallLog_ReturnsEmptySlice(t *testing.T) {
	env := setupE2E(t)

	if err := writeE2ECallLog(env); err != nil {
		t.Fatalf("writeE2ECallLog (empty) failed: %v", err)
	}

	bdCalls, err := FilterE2EToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(BD) on empty log failed: %v", err)
	}
	if len(bdCalls) != 0 {
		t.Errorf("expected empty slice for BD calls, got %d calls: %v", len(bdCalls), bdCalls)
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Claude) on empty log failed: %v", err)
	}
	if len(claudeCalls) != 0 {
		t.Errorf("expected empty slice for Claude calls, got %d calls: %v", len(claudeCalls), claudeCalls)
	}

	codexCalls, err := FilterE2EToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Codex) on empty log failed: %v", err)
	}
	if len(codexCalls) != 0 {
		t.Errorf("expected empty slice for Codex calls, got %d calls: %v", len(codexCalls), codexCalls)
	}
}

func TestE2E_FilterLargeCallLog_Correctness(t *testing.T) {
	env := setupE2E(t)

	calls := make([]string, 100)
	expectedBD := 0
	expectedClaude := 0
	expectedCodex := 0

	for i := 0; i < 100; i++ {
		switch i % 3 {
		case 0:
			calls[i] = "bd ready --json --limit 10"
			expectedBD++
		case 1:
			calls[i] = "claude -p --model haiku"
			expectedClaude++
		case 2:
			calls[i] = "codex run --model sonnet"
			expectedCodex++
		}
	}

	if err := writeE2ECallLog(env, calls...); err != nil {
		t.Fatalf("writeE2ECallLog failed: %v", err)
	}

	bdCalls, err := FilterE2EToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(BD) failed: %v", err)
	}
	if len(bdCalls) != expectedBD {
		t.Errorf("expected %d BD calls, got %d", expectedBD, len(bdCalls))
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Claude) failed: %v", err)
	}
	if len(claudeCalls) != expectedClaude {
		t.Errorf("expected %d Claude calls, got %d", expectedClaude, len(claudeCalls))
	}

	codexCalls, err := FilterE2EToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Codex) failed: %v", err)
	}
	if len(codexCalls) != expectedCodex {
		t.Errorf("expected %d Codex calls, got %d", expectedCodex, len(codexCalls))
	}
}

func TestE2E_FilterNonexistentCallLog_ReturnsEmptySlice(t *testing.T) {
	env := setupE2E(t)

	bdCalls, err := FilterE2EToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(BD) on nonexistent log failed: %v", err)
	}
	if len(bdCalls) != 0 {
		t.Errorf("expected empty slice when call log doesn't exist, got %d calls: %v", len(bdCalls), bdCalls)
	}
}
