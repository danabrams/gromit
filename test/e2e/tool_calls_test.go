//go:build e2e

package e2e

import (
	"os"
	"strings"
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

func TestE2E_FilteredCallOrder_PreservesSequence(t *testing.T) {
	env := setupE2E(t)

	calls := []string{
		"claude -p --model haiku",
		"bd ready --json --limit 10",
		"claude -p --model sonnet",
		"codex run --model sonnet",
		"claude -p --model opus",
		"bd close test-1",
	}

	if err := writeE2ECallLog(env, calls...); err != nil {
		t.Fatalf("writeE2ECallLog failed: %v", err)
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Claude) failed: %v", err)
	}

	expectedClaude := []string{
		"claude -p --model haiku",
		"claude -p --model sonnet",
		"claude -p --model opus",
	}

	if len(claudeCalls) != len(expectedClaude) {
		t.Fatalf("expected %d Claude calls, got %d: %v", len(expectedClaude), len(claudeCalls), claudeCalls)
	}

	for i, expected := range expectedClaude {
		if claudeCalls[i] != expected {
			t.Errorf("Claude call %d: expected %q, got %q", i, expected, claudeCalls[i])
		}
	}
}

func TestE2E_WriteAndFilterContent_ExactMatching(t *testing.T) {
	env := setupE2E(t)

	calls := []string{
		"bd ready --json --limit 10",
		"bd close test-bead-1",
		"bd sync",
		"bd ready --json --limit 10",
		"bd close test-bead-2",
	}

	if err := writeE2ECallLog(env, calls...); err != nil {
		t.Fatalf("writeE2ECallLog failed: %v", err)
	}

	bdCalls, err := FilterE2EToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(BD) failed: %v", err)
	}

	if len(bdCalls) != len(calls) {
		t.Fatalf("expected %d BD calls, got %d: %v", len(calls), len(bdCalls), bdCalls)
	}

	for i, expected := range calls {
		if bdCalls[i] != expected {
			t.Errorf("BD call %d: expected %q, got %q", i, expected, bdCalls[i])
		}
	}
}

func TestE2E_FilterComplexCallArguments(t *testing.T) {
	env := setupE2E(t)

	calls := []string{
		`claude -p "task description" --model haiku --stream --temperature 0.7`,
		`bd ready --json --limit 10 --filter "status:open"`,
		`codex run --model sonnet --input-tokens 4000 --output-tokens 2000`,
	}

	if err := writeE2ECallLog(env, calls...); err != nil {
		t.Fatalf("writeE2ECallLog failed: %v", err)
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Claude) failed: %v", err)
	}

	if len(claudeCalls) != 1 {
		t.Fatalf("expected 1 Claude call, got %d: %v", len(claudeCalls), claudeCalls)
	}

	expectedCall := `claude -p "task description" --model haiku --stream --temperature 0.7`
	if claudeCalls[0] != expectedCall {
		t.Errorf("expected %q, got %q", expectedCall, claudeCalls[0])
	}

	bdCalls, err := FilterE2EToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(BD) failed: %v", err)
	}

	if len(bdCalls) != 1 {
		t.Fatalf("expected 1 BD call, got %d: %v", len(bdCalls), bdCalls)
	}

	expectedBD := `bd ready --json --limit 10 --filter "status:open"`
	if bdCalls[0] != expectedBD {
		t.Errorf("expected %q, got %q", expectedBD, bdCalls[0])
	}

	codexCalls, err := FilterE2EToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Codex) failed: %v", err)
	}

	if len(codexCalls) != 1 {
		t.Fatalf("expected 1 Codex call, got %d: %v", len(codexCalls), codexCalls)
	}

	expectedCodex := `codex run --model sonnet --input-tokens 4000 --output-tokens 2000`
	if codexCalls[0] != expectedCodex {
		t.Errorf("expected %q, got %q", expectedCodex, codexCalls[0])
	}
}

func TestE2E_WriteCallLog_WithOnlyWhitespace(t *testing.T) {
	env := setupE2E(t)

	calls := []string{
		"",
		"   ",
		"\t",
		"",
	}

	if err := writeE2ECallLog(env, calls...); err != nil {
		t.Fatalf("writeE2ECallLog failed: %v", err)
	}

	bdCalls, err := FilterE2EToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(BD) failed: %v", err)
	}
	if len(bdCalls) != 0 {
		t.Errorf("expected 0 BD calls from whitespace-only log, got %d: %v", len(bdCalls), bdCalls)
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Claude) failed: %v", err)
	}
	if len(claudeCalls) != 0 {
		t.Errorf("expected 0 Claude calls from whitespace-only log, got %d: %v", len(claudeCalls), claudeCalls)
	}
}

func TestE2E_WriteAndReadCallLog_RoundTrip(t *testing.T) {
	env := setupE2E(t)

	originalCalls := []string{
		"bd ready --json --limit 10",
		"claude -p --model haiku",
		"codex run --model sonnet",
		"bd close test-id",
	}

	if err := writeE2ECallLog(env, originalCalls...); err != nil {
		t.Fatalf("writeE2ECallLog failed: %v", err)
	}

	readCalls, err := readE2ECallLog(env)
	if err != nil {
		t.Fatalf("readE2ECallLog failed: %v", err)
	}

	if len(readCalls) != len(originalCalls) {
		t.Fatalf("expected %d calls, got %d", len(originalCalls), len(readCalls))
	}

	for i, expected := range originalCalls {
		if readCalls[i] != expected {
			t.Errorf("call %d: expected %q, got %q", i, expected, readCalls[i])
		}
	}

	allBD, _ := FilterE2EToolCalls(env, ToolCallBD)
	allClaude, _ := FilterE2EToolCalls(env, ToolCallClaude)
	allCodex, _ := FilterE2EToolCalls(env, ToolCallCodex)

	if len(allBD) + len(allClaude) + len(allCodex) != len(originalCalls) {
		t.Errorf("expected filtered calls to sum to %d, got %d", len(originalCalls), len(allBD) + len(allClaude) + len(allCodex))
	}
}

func TestE2E_MultipleWriteOperations_LastWinsSemanticsPreserved(t *testing.T) {
	env := setupE2E(t)

	firstWrite := []string{"bd ready --json --limit 10"}
	secondWrite := []string{"claude -p --model haiku", "codex run --model sonnet"}
	thirdWrite := []string{"bd close test-id"}

	if err := writeE2ECallLog(env, firstWrite...); err != nil {
		t.Fatalf("first writeE2ECallLog failed: %v", err)
	}

	if err := writeE2ECallLog(env, secondWrite...); err != nil {
		t.Fatalf("second writeE2ECallLog failed: %v", err)
	}

	if err := writeE2ECallLog(env, thirdWrite...); err != nil {
		t.Fatalf("third writeE2ECallLog failed: %v", err)
	}

	readCalls, err := readE2ECallLog(env)
	if err != nil {
		t.Fatalf("readE2ECallLog failed: %v", err)
	}

	if len(readCalls) != len(thirdWrite) {
		t.Fatalf("expected %d calls (last write), got %d: %v", len(thirdWrite), len(readCalls), readCalls)
	}

	for i, expected := range thirdWrite {
		if readCalls[i] != expected {
			t.Errorf("call %d: expected %q, got %q", i, expected, readCalls[i])
		}
	}

	bdCalls, _ := FilterE2EToolCalls(env, ToolCallBD)
	claudeCalls, _ := FilterE2EToolCalls(env, ToolCallClaude)
	codexCalls, _ := FilterE2EToolCalls(env, ToolCallCodex)

	if len(bdCalls) != 1 || bdCalls[0] != "bd close test-id" {
		t.Errorf("expected 1 BD call 'bd close test-id', got %d: %v", len(bdCalls), bdCalls)
	}

	if len(claudeCalls) != 0 {
		t.Errorf("expected 0 Claude calls from third write, got %d: %v", len(claudeCalls), claudeCalls)
	}

	if len(codexCalls) != 0 {
		t.Errorf("expected 0 Codex calls from third write, got %d: %v", len(codexCalls), codexCalls)
	}
}

func TestE2E_CallLogWithLeadingTrailingWhitespace_FilterStillWorks(t *testing.T) {
	env := setupE2E(t)

	data := strings.Join([]string{
		"  bd ready --json --limit 10",
		"\tclaude -p --model haiku",
		"   codex run --model sonnet   ",
		"bd close test-id\t",
	}, "\n") + "\n"

	if err := os.WriteFile(env.CallLog, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	bdCalls, err := FilterE2EToolCalls(env, ToolCallBD)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(BD) failed: %v", err)
	}

	if len(bdCalls) != 2 {
		t.Errorf("expected 2 BD calls despite whitespace, got %d: %v", len(bdCalls), bdCalls)
	}

	claudeCalls, err := FilterE2EToolCalls(env, ToolCallClaude)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Claude) failed: %v", err)
	}

	if len(claudeCalls) != 1 {
		t.Errorf("expected 1 Claude call despite whitespace, got %d: %v", len(claudeCalls), claudeCalls)
	}

	codexCalls, err := FilterE2EToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterE2EToolCalls(Codex) failed: %v", err)
	}

	if len(codexCalls) != 1 {
		t.Errorf("expected 1 Codex call despite whitespace, got %d: %v", len(codexCalls), codexCalls)
	}
}
