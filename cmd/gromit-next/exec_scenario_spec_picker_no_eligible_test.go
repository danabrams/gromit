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

// TestScenario_SpecPicker_NoEligibleSpecs verifies that pickSpec prints
// "no specs available to run\n" and returns ("", nil) when every spec in
// the directory has a derived status of "completed" or "draft".
func TestScenario_SpecPicker_NoEligibleSpecs(t *testing.T) {
	// --- Seed ---

	// Create a specs directory with two .md files:
	// - completed-spec.md: has a completed run → derived status "completed"
	// - draft-spec.md: content starts with "DRAFT" → derived status "draft"
	specsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(specsDir, "completed-spec.md"), []byte("# Completed Spec\n"), 0o644); err != nil {
		t.Fatalf("write completed-spec.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "draft-spec.md"), []byte("DRAFT\n# Draft Spec\n"), 0o644); err != nil {
		t.Fatalf("write draft-spec.md: %v", err)
	}

	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// completed-spec: one run with StatusCompleted → derived status "completed"
	if err := store.Save(&runstore.RunState{
		RunID:     "run-completed-001",
		SpecID:    "completed-spec",
		ProjectID: "test-proj",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}); err != nil {
		t.Fatalf("save completed run: %v", err)
	}

	// draft-spec: no runs needed — "DRAFT" prefix in content triggers "draft" status

	// --- Invoke ---

	// No input needed — pickSpec should return before reading stdin.
	in := strings.NewReader("")
	var out bytes.Buffer

	specPath, err := pickSpec("test-proj", specsDir, store, nil, in, &out)

	// --- Assert ---

	// Must return ("", nil) — empty string signals caller to return nil without executing.
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if specPath != "" {
		t.Errorf("expected empty spec path, got %q", specPath)
	}

	// Output must contain the "no specs available" message.
	if out.String() != "no specs available to run\n" {
		t.Errorf("expected %q, got %q", "no specs available to run\n", out.String())
	}
}