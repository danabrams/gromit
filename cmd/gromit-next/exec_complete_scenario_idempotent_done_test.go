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

func TestScenario_ExecComplete_AlreadyDoneSpecIsIdempotent(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	specsDir := filepath.Join(tmp, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}

	specContent := "DONE 2026-03-15\n\n# Spec 0003a — Infrastructure Failure Detection\n\nSome content here.\n"
	specPath := filepath.Join(specsDir, "0003a.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	rs := &runstore.RunState{
		RunID:     "run-xyz",
		SpecID:    "0003a",
		ProjectID: "gromit",
		Status:    runstore.StatusReadyForReview,
		StartedAt: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{{Status: "done"}},
	}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Invoke
	cmd := newExecCompleteCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"run-xyz", "--store-dir", tmp})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec complete: %v", err)
	}

	// Assert — run is marked completed
	output := buf.String()
	if !strings.Contains(output, "run-xyz") {
		t.Error("expected run ID in output")
	}
	if !strings.Contains(output, "completed") {
		t.Error("expected 'completed' in output")
	}

	// Assert — run status is now completed in the store
	updated, err := store.Get("run-xyz")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updated.Status != runstore.StatusCompleted {
		t.Errorf("expected status %q, got %q", runstore.StatusCompleted, updated.Status)
	}
	if updated.EndedAt.IsZero() {
		t.Error("expected EndedAt to be set")
	}

	// Assert — spec file is unchanged, no second DONE line added
	afterContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec file: %v", err)
	}
	if string(afterContent) != specContent {
		t.Errorf("spec file was modified.\nwant:\n%s\ngot:\n%s", specContent, string(afterContent))
	}
	if strings.Count(string(afterContent), "DONE") != 1 {
		t.Errorf("expected exactly one DONE line, found %d", strings.Count(string(afterContent), "DONE"))
	}
}