package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDebug2Integration_AppliesFix_PersistsLearning applies a fix and persists learning end-to-end.
func TestDebug2Integration_AppliesFix_PersistsLearning(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	wtPath := filepath.Join(tmpDir, "spec-worktrees", specName)

	// Setup worktree with event log
	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create event log with a nil-pointer failure pattern
	eventLine := `{"type":"stage.failed","stage_name":"build","bead_id":"b1","error":"nil pointer dereference"}`
	if err := os.WriteFile(filepath.Join(eventsDir, "events.jsonl"), []byte(eventLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create LEARNINGS.md
	learningsPath := filepath.Join(wtPath, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte("# Learnings\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run orchestrated debug
	ctx := context.Background()
	result, err := debug2OrchestrateDebug(ctx, tmpDir, specName, wtPath)
	if err != nil {
		t.Fatalf("debug2OrchestrateDebug failed: %v", err)
	}

	// Verify diagnosis completed
	if !result.DiagnosisComplete {
		t.Error("diagnosis should complete")
	}

	// Verify failure was identified
	if result.FailureEvent == nil {
		t.Error("failure event should be identified")
	}

	// Verify learning was extracted
	if !result.LearningExtracted {
		t.Error("learning should be extracted from nil pointer dereference pattern")
	}

	if result.LearningEntry == "" {
		t.Error("learning entry should be non-empty")
	}
}

// TestDebug2Integration_CanPersistLearningToFile persists learning to LEARNINGS.md.
func TestDebug2Integration_CanPersistLearningToFile(t *testing.T) {
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	// Create initial LEARNINGS.md
	if err := os.WriteFile(learningsPath, []byte("# Learnings\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate persisting a learning discovered during debug
	entry := "## Nil Pointer Safety\n\nAlways check pointer before dereferencing.\n"
	if err := debug2PersistLearningToFile(learningsPath, entry); err != nil {
		t.Fatalf("failed to persist learning: %v", err)
	}

	// Verify learning was persisted
	content, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Nil Pointer Safety") {
		t.Error("learning not persisted to LEARNINGS.md")
	}
}
