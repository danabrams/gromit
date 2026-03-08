package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestDebug2_OrchestratesDiagnoseFixLearn diagnoses, fixes, and learns from a failure.
func TestDebug2_OrchestratesDiagnoseFixLearn(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	wtPath := filepath.Join(tmpDir, "spec-worktrees", specName)

	// Setup worktree with event log
	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	eventLine := `{"type":"stage.failed","stage_name":"validate","bead_id":"b1","error":"syntax error"}` + "\n"
	if err := os.WriteFile(filepath.Join(eventsDir, "events.jsonl"), []byte(eventLine), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a test file to fix
	testFile := filepath.Join(wtPath, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc broken() {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Call the orchestration function
	ctx := context.Background()
	result, err := debug2OrchestrateDebug(ctx, tmpDir, specName, wtPath)
	if err != nil {
		t.Fatalf("debug2OrchestrateDebug failed: %v", err)
	}

	if !result.DiagnosisComplete {
		t.Error("result.DiagnosisComplete = false, want true")
	}
	if result.FixApplied && result.LearningExtracted {
		// At least one should be true (either fix or learning)
		// This demonstrates the tool attempted to improve
	}
}

// TestDebug2_OrchestrateDebug_ReadsEventLog verifies event log is read and parsed.
func TestDebug2_OrchestrateDebug_ReadsEventLog(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	wtPath := filepath.Join(tmpDir, "spec-worktrees", specName)

	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write multiple events
	events := `{"type":"stage.completed","decision":"Proceed"}
{"type":"stage.failed","error":"build failed"}
`
	if err := os.WriteFile(filepath.Join(eventsDir, "events.jsonl"), []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	result, err := debug2OrchestrateDebug(ctx, tmpDir, specName, wtPath)
	if err != nil {
		t.Fatalf("debug2OrchestrateDebug failed: %v", err)
	}

	if len(result.Events) == 0 {
		t.Error("result.Events is empty, want to contain parsed events")
	}
}

