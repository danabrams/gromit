package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebug2_InvokesAgentInWorktree(t *testing.T) {
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
	orig := debug2AgentLaunchFn
	t.Cleanup(func() { debug2AgentLaunchFn = orig })
	debug2AgentLaunchFn = func(promptPath, dir string) error {
		capturedDir = dir
		return nil
	}

	if err := debug2Impl(specName, tmpDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedDir != wtPath {
		t.Errorf("agent launched in %q, want %q", capturedDir, wtPath)
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

	prompt := buildDebug2Prompt(specName, wtPath, events, commits)

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
