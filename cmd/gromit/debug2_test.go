package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	debugpkg "github.com/danabrams/gromit/internal/v2/debug"
	"github.com/spf13/cobra"
)

func TestDebug2_InvokesLLMInWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	wtPath := filepath.Join(tmpDir, "spec-worktrees", specName)

	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(eventsDir, "events.jsonl"),
		[]byte(`{"type":"stage.completed","decision":"Fail"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var capturedDir string
	orig := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = orig })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		capturedDir = dir
		return `{"code_patch":"","learnings_entry":"","systemic_recommendation":""}`, nil
	}

	if err := debug2Impl(context.Background(), specName, tmpDir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedDir != wtPath {
		t.Errorf("llm invoked in %q, want %q", capturedDir, wtPath)
	}
}

func TestBuildDebug2Prompt_IncludesSpecNameAndEvents(t *testing.T) {
	specName := "test-spec"
	wtPath := "/tmp/worktrees/test-spec"
	events := []map[string]interface{}{
		{"type": "stage.completed", "stage_name": "validate", "decision": "Fail"},
	}
	commits := [][2]string{
		{"abc12345", "[bead:b1/validate/iter:1] Fail"},
	}

	prompt := buildDebug2Prompt(specName, wtPath, events, commits, "", nil, debugpkg.Diagnosis{})

	if !strings.Contains(prompt, "test-spec") {
		t.Error("prompt missing spec name")
	}
	if !strings.Contains(prompt, "validate") {
		t.Error("prompt missing failure stage")
	}
	if !strings.Contains(prompt, "abc12345") {
		t.Error("prompt missing commit hash")
	}
}

func TestFindFailureEvent_FindsFailDecision(t *testing.T) {
	events := []map[string]interface{}{
		{"type": "stage.completed", "stage_name": "build", "decision": "Proceed"},
		{"type": "stage.completed", "stage_name": "validate", "decision": "Fail"},
	}
	result := findFailureEvent(events)
	if result == nil {
		t.Fatal("expected failure event, got nil")
	}
	if result["stage_name"] != "validate" {
		t.Errorf("stage_name = %q, want %q", result["stage_name"], "validate")
	}
}

func TestReadDebug2EventLog_ParsesJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eventLine := `{"type":"stage.completed","stage_name":"build","bead_id":"b1","decision":"Fail"}` + "\n"
	eventsPath := filepath.Join(eventsDir, "events.jsonl")
	if err := os.WriteFile(eventsPath, []byte(eventLine), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := readDebug2EventLog(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0]["stage_name"] != "build" {
		t.Errorf("stage_name = %q, want %q", events[0]["stage_name"], "build")
	}
}

func TestDebug2Impl_DisplaysEventLogEntries(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "display-events"
	gromitDir := filepath.Join(tmpDir, ".gromit")
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)
	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")

	if err := os.MkdirAll(filepath.Dir(eventsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	eventLines := strings.Join([]string{
		`{"type":"stage.completed","stage_name":"build","decision":"Proceed"}`,
		`{"type":"stage.failed","stage_name":"validate","error":"provider failed"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(eventsPath, []byte(eventLines), 0o644); err != nil {
		t.Fatal(err)
	}

	origInvoke := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = origInvoke })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		return `{"code_patch":"","learnings_entry":"","systemic_recommendation":""}`, nil
	}

	origRunValidation := debug2RunValidationFn
	t.Cleanup(func() { debug2RunValidationFn = origRunValidation })
	debug2RunValidationFn = func(ctx context.Context, dir, command string) error {
		return nil
	}

	var stdout bytes.Buffer
	origStdout := debug2Stdout
	t.Cleanup(func() { debug2Stdout = origStdout })
	debug2Stdout = &stdout

	if err := debug2Impl(context.Background(), specName, gromitDir, nil); err != nil {
		t.Fatalf("debug2Impl() error = %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Event log (.gromit/v2/events.jsonl)") {
		t.Fatalf("stdout missing event log header, got:\n%s", got)
	}
	if !strings.Contains(got, `{"type":"stage.failed","stage_name":"validate","error":"provider failed"}`) {
		t.Fatalf("stdout missing event log entry, got:\n%s", got)
	}
}

func TestDebug2Impl_DisplaysGitHistoryCommands(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "history-spec"
	gromitDir := filepath.Join(tmpDir, ".gromit")
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)
	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")

	if err := os.MkdirAll(filepath.Dir(eventsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eventsPath, []byte(`{"type":"stage.failed","stage_name":"build","error":"boom"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origInvoke := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = origInvoke })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		return `{"code_patch":"","learnings_entry":"","systemic_recommendation":""}`, nil
	}

	origRunValidation := debug2RunValidationFn
	t.Cleanup(func() { debug2RunValidationFn = origRunValidation })
	debug2RunValidationFn = func(ctx context.Context, dir, command string) error {
		return nil
	}

	var stdout bytes.Buffer
	origStdout := debug2Stdout
	t.Cleanup(func() { debug2Stdout = origStdout })
	debug2Stdout = &stdout

	if err := debug2Impl(context.Background(), specName, gromitDir, nil); err != nil {
		t.Fatalf("debug2Impl() error = %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Git history commands (worktree branch gromit/spec/history-spec):") {
		t.Fatalf("stdout missing git history header, got:\n%s", got)
	}
	if !strings.Contains(got, fmt.Sprintf("git -C %s log --oneline", wtPath)) {
		t.Fatalf("stdout missing git log command, got:\n%s", got)
	}
	if !strings.Contains(got, fmt.Sprintf("git -C %s show <commit-hash>", wtPath)) {
		t.Fatalf("stdout missing git show command, got:\n%s", got)
	}
}

func TestResolveDebug2Worktree_ReturnsErrorWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "nonexistent-spec"

	// Mock branch finder to return an error when both worktree and branch are missing
	orig := debug2BranchWorktreeFn
	t.Cleanup(func() { debug2BranchWorktreeFn = orig })
	debug2BranchWorktreeFn = func(gromitDir, specName string) (string, error) {
		return "", fmt.Errorf("no preserved worktree or branch found for spec %q", specName)
	}

	_, err := resolveDebug2Worktree(tmpDir, specName)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no preserved worktree or branch found") {
		t.Errorf("error = %q, want to contain 'no preserved worktree or branch found'", err.Error())
	}
}

func TestResolveDebug2Worktree_WrapsFinderNotFoundError(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "missing-spec"

	orig := debug2BranchWorktreeFn
	t.Cleanup(func() { debug2BranchWorktreeFn = orig })
	debug2BranchWorktreeFn = func(gromitDir, spec string) (string, error) {
		return "", fmt.Errorf("%w: %s", debugpkg.ErrPreservedWorktreeBranchNotFound, spec)
	}

	_, err := resolveDebug2Worktree(tmpDir, specName)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), `no preserved worktree or branch found for spec "missing-spec"`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestResolveDebug2Worktree_FindsExistingWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "my-spec"
	wtPath := filepath.Join(tmpDir, "spec-worktrees", specName)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := resolveDebug2Worktree(tmpDir, specName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wtPath {
		t.Errorf("got %q, want %q", got, wtPath)
	}
}

func TestResolveDebug2Worktree_FallsBackToBranchWhenWorktreeMissing(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "fallback-spec"
	wtPath := filepath.Join(tmpDir, "spec-worktrees", specName)
	// Don't create the worktree directory

	// Mock the branch finder to return a valid path when worktree is missing
	orig := debug2BranchWorktreeFn
	t.Cleanup(func() { debug2BranchWorktreeFn = orig })
	debug2BranchWorktreeFn = func(gromitDir, specName string) (string, error) {
		// Simulate finding branch and returning a valid path
		return wtPath, nil
	}

	got, err := resolveDebug2Worktree(tmpDir, specName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wtPath {
		t.Errorf("got %q, want %q", got, wtPath)
	}
}

func TestDebug2BranchWorktreeFn_FindsWorktreeForSpecBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := t.TempDir()
	gromitDir := filepath.Join(repoDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runGitDebug2(t, repoDir, "init")
	runGitDebug2(t, repoDir, "config", "user.email", "tester@example.com")
	runGitDebug2(t, repoDir, "config", "user.name", "Test User")
	runGitDebug2(t, repoDir, "commit", "--allow-empty", "-m", "init")

	altWorktree := filepath.Join(repoDir, "other-worktree")
	runGitDebug2(t, repoDir, "worktree", "add", "-B", "gromit/spec/spec-x", altWorktree, "HEAD")

	got, err := debug2BranchWorktreeFn(gromitDir, "spec-x")
	if err != nil {
		t.Fatalf("debug2BranchWorktreeFn() error = %v", err)
	}
	if got != altWorktree {
		t.Fatalf("worktree path = %q, want %q", got, altWorktree)
	}
}

func TestDebug2BranchWorktreeFn_RejectsInvalidSpecName(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := debug2BranchWorktreeFn(gromitDir, "../bad-spec")
	if err == nil {
		t.Fatal("expected error for invalid spec name, got nil")
	}
	if !strings.Contains(err.Error(), "invalid spec name") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "invalid spec name")
	}
}

func TestSelectDebug2FailureCommit_PicksMostRecentStructuredFail(t *testing.T) {
	entries := []adapter.LogEntry{
		{Hash: "hash1", Message: "not structured"},
		{Hash: "hash2", Message: "[bead:b1/build/iter:2] Proceed"},
		{Hash: "hash3", Message: "[bead:b1/validate/iter:2] Fail"},
		{Hash: "hash4", Message: "[bead:b1/validate/iter:1] Fail"},
	}

	failure, info, ok := selectDebug2FailureCommit(entries)
	if !ok {
		t.Fatal("expected failure commit, got none")
	}
	if failure.Hash != "hash3" {
		t.Fatalf("failure hash = %q, want %q", failure.Hash, "hash3")
	}
	if info.StageName != "validate" {
		t.Fatalf("stage = %q, want %q", info.StageName, "validate")
	}
	if info.Decision != "Fail" {
		t.Fatalf("decision = %q, want %q", info.Decision, "Fail")
	}
}

func TestDebug2Diagnose_UsesCommitStageWhenFailureEventStageMissing(t *testing.T) {
	diagnosis := debugpkg.Diagnose(debugpkg.Input{
		Events: []map[string]interface{}{
			{
				"type":  "stage.failed",
				"error": "provider reported unsuccessful result: no detail available",
			},
		},
		LogEntries: []adapter.LogEntry{
			{Hash: "abc12345", Message: "[bead:b1/validate/iter:2] Fail"},
		},
	})

	if diagnosis.Stage != "validate" {
		t.Fatalf("diagnosis.Stage = %q, want %q", diagnosis.Stage, "validate")
	}
	if diagnosis.RootCause != debugpkg.RootCauseFlakyTest {
		t.Fatalf("diagnosis.RootCause = %q, want %q", diagnosis.RootCause, debugpkg.RootCauseFlakyTest)
	}
}

func runGitDebug2(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestDebug2Impl_PromptIncludesEventTailAndFailureDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	rootDir := t.TempDir()
	gromitDir := filepath.Join(rootDir, ".gromit")
	specName := "spec-a"
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	runGitDebug2(t, wtPath, "init")
	runGitDebug2(t, wtPath, "config", "user.email", "tester@example.com")
	runGitDebug2(t, wtPath, "config", "user.name", "Test User")
	runGitDebug2(t, wtPath, "commit", "--allow-empty", "-m", "init")

	if err := os.WriteFile(filepath.Join(wtPath, "failing.txt"), []byte("boom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitDebug2(t, wtPath, "add", "-A")
	runGitDebug2(t, wtPath, "commit", "-m", "[bead:b1/validate/iter:1] Fail")

	if err := os.WriteFile(filepath.Join(wtPath, "good.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitDebug2(t, wtPath, "add", "-A")
	runGitDebug2(t, wtPath, "commit", "-m", "[bead:b1/build/iter:2] Proceed")

	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(eventsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	events := strings.Join([]string{
		`{"event":"old","decision":"Proceed"}`,
		`{"event":"mid","decision":"Proceed"}`,
		`{"event":"new","decision":"Fail"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(eventsPath, []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}

	var promptText string
	origInvoke := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = origInvoke })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		promptText = prompt
		return `{"code_patch":"","learnings_entry":"","systemic_recommendation":""}`, nil
	}

	if err := debug2Impl(context.Background(), specName, gromitDir, nil); err != nil {
		t.Fatalf("debug2Impl() error = %v", err)
	}

	if !strings.Contains(promptText, "### Failure Diff") {
		t.Fatalf("prompt missing failure diff section: %q", promptText)
	}
	if !strings.Contains(promptText, "failing.txt") {
		t.Fatalf("prompt missing failure diff contents: %q", promptText)
	}
	if strings.Contains(promptText, `{"event":"old","decision":"Proceed"}`) {
		t.Fatalf("prompt should include event tail only, but contained oldest event: %q", promptText)
	}
}

func TestDebug2Impl_AppliesPatchAndRunsValidation(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	gromitDir := filepath.Join(tmpDir, ".gromit")
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)

	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(eventsDir, "events.jsonl"),
		[]byte(`{"type":"stage.completed","decision":"Fail"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wantPatch := "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -0,0 +1 @@\n+fixed\n"
	llmOutput := fmt.Sprintf(`{"code_patch": %q, "learnings_entry": "", "systemic_recommendation": ""}`, wantPatch)

	origInvoke := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = origInvoke })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		return llmOutput, nil
	}

	var gotPatch string
	origApplyPatch := debug2ApplyPatchFn
	t.Cleanup(func() { debug2ApplyPatchFn = origApplyPatch })
	debug2ApplyPatchFn = func(ctx context.Context, dir, patch string) error {
		gotPatch = patch
		return nil
	}

	var ranCommands []string
	origRunValidation := debug2RunValidationFn
	t.Cleanup(func() { debug2RunValidationFn = origRunValidation })
	debug2RunValidationFn = func(ctx context.Context, dir, command string) error {
		ranCommands = append(ranCommands, command)
		return nil
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Validation.Commands = []string{"go test ./cmd/gromit", "go vet ./cmd/gromit"}

	if err := debug2Impl(context.Background(), specName, gromitDir, cfg); err != nil {
		t.Fatalf("debug2Impl() error = %v", err)
	}

	if gotPatch != wantPatch {
		t.Fatalf("applied patch = %q, want %q", gotPatch, wantPatch)
	}
	wantCommands := []string{"go test ./cmd/gromit", "go vet ./cmd/gromit"}
	if !reflect.DeepEqual(ranCommands, wantCommands) {
		t.Fatalf("validation commands = %#v, want %#v", ranCommands, wantCommands)
	}
}

func TestDebug2Impl_ChecksOutDiagnosedFailureCommitBeforePatch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	specName := "test-spec"
	gromitDir := filepath.Join(tmpDir, ".gromit")
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)
	if err := os.MkdirAll(filepath.Join(wtPath, ".gromit", "v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, ".gromit", "v2", "events.jsonl"),
		[]byte(`{"type":"stage.failed","stage_name":"build","error":"provider reported unsuccessful result: no detail available"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runGitDebug2(t, wtPath, "init")
	runGitDebug2(t, wtPath, "config", "user.email", "tester@example.com")
	runGitDebug2(t, wtPath, "config", "user.name", "Test User")
	runGitDebug2(t, wtPath, "commit", "--allow-empty", "-m", "init")

	if err := os.WriteFile(filepath.Join(wtPath, "bad.txt"), []byte("bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitDebug2(t, wtPath, "add", "-A")
	runGitDebug2(t, wtPath, "commit", "-m", "[bead:b1/build/iter:1] Fail")

	failHashCmd := exec.Command("git", "rev-parse", "HEAD")
	failHashCmd.Dir = wtPath
	out, err := failHashCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse failure hash: %v\n%s", err, out)
	}
	wantFailureHash := strings.TrimSpace(string(out))

	runGitDebug2(t, wtPath, "commit", "--allow-empty", "-m", "[bead:b1/build/iter:2] Proceed")

	llmOutput := `{"code_patch":"diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -0,0 +1 @@\n+fixed\n","learnings_entry":"","systemic_recommendation":""}`
	origInvoke := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = origInvoke })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		return llmOutput, nil
	}

	var checkedOut string
	origCheckout := debug2CheckoutFailureFn
	t.Cleanup(func() { debug2CheckoutFailureFn = origCheckout })
	debug2CheckoutFailureFn = func(ctx context.Context, dir, commit string) error {
		checkedOut = commit
		return nil
	}

	origApplyPatch := debug2ApplyPatchFn
	t.Cleanup(func() { debug2ApplyPatchFn = origApplyPatch })
	debug2ApplyPatchFn = func(ctx context.Context, dir, patch string) error {
		return nil
	}

	origRunValidation := debug2RunValidationFn
	t.Cleanup(func() { debug2RunValidationFn = origRunValidation })
	debug2RunValidationFn = func(ctx context.Context, dir, command string) error {
		return nil
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	if err := debug2Impl(context.Background(), specName, gromitDir, cfg); err != nil {
		t.Fatalf("debug2Impl() error = %v", err)
	}
	if checkedOut != wantFailureHash {
		t.Fatalf("checked out commit = %q, want %q", checkedOut, wantFailureHash)
	}
}

func TestDebug2Impl_AppendsLearningsEntry(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	gromitDir := filepath.Join(tmpDir, ".gromit")
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)

	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(eventsDir, "events.jsonl"),
		[]byte(`{"type":"stage.completed","decision":"Fail"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	learningsPath := filepath.Join(wtPath, "LEARNINGS.md")
	initialLearnings := "# LEARNINGS\n\n## Confirmed Learnings\n\n## Provisional Learnings\n\n*No provisional learnings at this time.*\n"
	if err := os.WriteFile(learningsPath, []byte(initialLearnings), 0o644); err != nil {
		t.Fatal(err)
	}

	learningEntry := "### 2026-03-08 | debug2_learning | CODE_PATTERN\n\nAlways preserve failing context before retries.\n"
	llmOutput := fmt.Sprintf(`{"code_patch":"","learnings_entry":%q,"systemic_recommendation":""}`, learningEntry)

	origInvoke := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = origInvoke })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		return llmOutput, nil
	}

	origLearningValidation := debug2RunValidationFn
	t.Cleanup(func() { debug2RunValidationFn = origLearningValidation })
	debug2RunValidationFn = func(ctx context.Context, dir, command string) error {
		return nil
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	if err := debug2Impl(context.Background(), specName, gromitDir, cfg); err != nil {
		t.Fatalf("debug2Impl() error = %v", err)
	}

	updated, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("reading LEARNINGS.md: %v", err)
	}
	if !strings.Contains(string(updated), learningEntry) {
		t.Fatalf("LEARNINGS.md missing appended entry, got:\n%s", string(updated))
	}
}

func TestDebug2Impl_AppendsAutonomousLearningFromRootCausePattern(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	gromitDir := filepath.Join(tmpDir, ".gromit")
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)

	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(eventsDir, "events.jsonl"),
		[]byte(`{"type":"stage.failed","stage_name":"build","error":"provider reported unsuccessful result: no detail available"}`+"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	learningsPath := filepath.Join(wtPath, "LEARNINGS.md")
	initialLearnings := "# LEARNINGS\n\n## Confirmed Learnings\n\n## Provisional Learnings\n\n*No provisional learnings at this time.*\n"
	if err := os.WriteFile(learningsPath, []byte(initialLearnings), 0o644); err != nil {
		t.Fatal(err)
	}

	origInvoke := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = origInvoke })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		return `{"code_patch":"","learnings_entry":"","systemic_recommendation":""}`, nil
	}

	origRunValidation := debug2RunValidationFn
	t.Cleanup(func() { debug2RunValidationFn = origRunValidation })
	debug2RunValidationFn = func(ctx context.Context, dir, command string) error {
		return nil
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	if err := debug2Impl(context.Background(), specName, gromitDir, cfg); err != nil {
		t.Fatalf("debug2Impl() error = %v", err)
	}

	updated, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("reading LEARNINGS.md: %v", err)
	}
	updatedStr := string(updated)
	if !strings.Contains(updatedStr, "Capture build failure diagnostics before retries") {
		t.Fatalf("LEARNINGS.md missing root-cause learning pattern, got:\n%s", updatedStr)
	}
	if !strings.Contains(updatedStr, "*Autonomous: true*") {
		t.Fatalf("LEARNINGS.md missing autonomous marker, got:\n%s", updatedStr)
	}
}

func TestDebug2Impl_WritesSystemicRecommendationToStderr(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	gromitDir := filepath.Join(tmpDir, ".gromit")
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)

	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(eventsDir, "events.jsonl"),
		[]byte(`{"type":"stage.completed","decision":"Fail"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	systemic := "Add a pipeline guard so duplicate schema writers are blocked in CI."
	llmOutput := fmt.Sprintf(`{"code_patch":"","learnings_entry":"","systemic_recommendation":%q}`, systemic)

	origInvoke := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = origInvoke })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		return llmOutput, nil
	}

	origRunValidation := debug2RunValidationFn
	t.Cleanup(func() { debug2RunValidationFn = origRunValidation })
	debug2RunValidationFn = func(ctx context.Context, dir, command string) error {
		return nil
	}

	var stderr bytes.Buffer
	origStderr := debug2Stderr
	t.Cleanup(func() { debug2Stderr = origStderr })
	debug2Stderr = &stderr

	cfg := &config.Config{}
	cfg.SetDefaults()

	if err := debug2Impl(context.Background(), specName, gromitDir, cfg); err != nil {
		t.Fatalf("debug2Impl() error = %v", err)
	}

	if !strings.Contains(stderr.String(), systemic) {
		t.Fatalf("stderr missing systemic recommendation, got: %q", stderr.String())
	}
}

func TestDebug2Impl_SystemicRecommendationDoesNotAppendAutonomousLearning(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	gromitDir := filepath.Join(tmpDir, ".gromit")
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)

	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(eventsDir, "events.jsonl"),
		[]byte(`{"type":"stage.completed","decision":"Fail"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	learningsPath := filepath.Join(wtPath, "LEARNINGS.md")
	initialLearnings := "# LEARNINGS\n\n## Confirmed Learnings\n\n## Provisional Learnings\n\n*No provisional learnings at this time.*\n"
	if err := os.WriteFile(learningsPath, []byte(initialLearnings), 0o644); err != nil {
		t.Fatal(err)
	}

	learningsEntry := "### 2026-03-08 | debug2_systemic | ARCHITECTURE\n\nAdd a process guard for prompt fragment defaults.\n"
	systemic := "Add a pipeline guard for prompt fragment defaults and review in architecture meeting."
	llmOutput := fmt.Sprintf(`{"code_patch":"","learnings_entry":%q,"systemic_recommendation":%q}`, learningsEntry, systemic)

	origInvoke := debug2InvokeLLMFn
	t.Cleanup(func() { debug2InvokeLLMFn = origInvoke })
	debug2InvokeLLMFn = func(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
		return llmOutput, nil
	}

	origRunValidation := debug2RunValidationFn
	t.Cleanup(func() { debug2RunValidationFn = origRunValidation })
	debug2RunValidationFn = func(ctx context.Context, dir, command string) error {
		return nil
	}

	var stderr bytes.Buffer
	origStderr := debug2Stderr
	t.Cleanup(func() { debug2Stderr = origStderr })
	debug2Stderr = &stderr

	cfg := &config.Config{}
	cfg.SetDefaults()

	if err := debug2Impl(context.Background(), specName, gromitDir, cfg); err != nil {
		t.Fatalf("debug2Impl() error = %v", err)
	}

	updated, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("reading LEARNINGS.md: %v", err)
	}
	if strings.Contains(string(updated), learningsEntry) {
		t.Fatalf("LEARNINGS.md should not contain systemic learnings entry, got:\n%s", string(updated))
	}
	if !strings.Contains(stderr.String(), systemic) {
		t.Fatalf("stderr missing systemic recommendation, got: %q", stderr.String())
	}
}

func TestDebug2RunE_ThreadsCommandContext(t *testing.T) {
	repoRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot() error = %v", err)
	}
	origConfigPath := configPath
	t.Cleanup(func() { configPath = origConfigPath })
	configPath = filepath.Join(repoRoot, "gromit.yaml")

	var captured context.Context
	origImpl := debug2ImplFn
	t.Cleanup(func() { debug2ImplFn = origImpl })
	debug2ImplFn = func(ctx context.Context, specName, gromitDir string, cfg *config.Config) error {
		captured = ctx
		return nil
	}

	type ctxKey struct{}
	cmd := &cobra.Command{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "threaded")
	cmd.SetContext(ctx)

	if err := debug2RunE(cmd, []string{"spec-a"}); err != nil {
		t.Fatalf("debug2RunE() error = %v", err)
	}
	if got := captured.Value(ctxKey{}); got != "threaded" {
		t.Fatalf("captured context value = %v, want %q", got, "threaded")
	}
}
