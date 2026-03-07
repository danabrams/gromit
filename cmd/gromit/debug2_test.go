package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	_, err := resolveDebug2Worktree(tmpDir, "nonexistent-spec")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no preserved worktree found") {
		t.Errorf("error = %q, want to contain 'no preserved worktree found'", err.Error())
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
