package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ExecComplete_MarksSpecFileAsDone(t *testing.T) {
	// Seed: create a specs directory with 0003h.md
	specsDir := filepath.Join(t.TempDir(), "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	originalContent := "# Spec 0003h — Contract File Self-Correction\n\n## spec_id\n0003h\n"
	specPath := filepath.Join(specsDir, "0003h.md")
	if err := os.WriteFile(specPath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Seed: create store with a ready_for_review run
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)
	rs := &runstore.RunState{
		RunID:     "run-abc123",
		SpecID:    "0003h",
		ProjectID: "gromit",
		Status:    runstore.StatusReadyForReview,
		StartedAt: time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Invoke: exec complete via cobra
	cmd := newExecCompleteCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"run-abc123",
		"--store-dir", storeDir,
		"--specs-dir", specsDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}
	output := buf.String()

	// Assert: run status is completed
	updated, err := store.Get("run-abc123")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updated.Status != runstore.StatusCompleted {
		t.Errorf("expected status %q, got %q", runstore.StatusCompleted, updated.Status)
	}

	// Assert: confirmation message printed
	if !strings.Contains(output, "run-abc123") {
		t.Errorf("expected run ID in output, got: %s", output)
	}
	if !strings.Contains(output, "completed") {
		t.Errorf("expected 'completed' in output, got: %s", output)
	}

	// Assert: spec file now starts with "DONE 2026-03-19\n" followed by original content
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec file: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "DONE 2026-03-19\n") {
		t.Errorf("expected spec file to start with 'DONE 2026-03-19\\n', got:\n%s", content)
	}

	// Assert: original content is preserved after the DONE line
	afterDone := strings.TrimPrefix(content, "DONE 2026-03-19\n")
	if afterDone != originalContent {
		t.Errorf("expected original content preserved after DONE line.\nWant:\n%s\nGot:\n%s", originalContent, afterDone)
	}
}
