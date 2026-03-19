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

// TestScenario_SpecPickerWithMixedStatuses verifies that pickSpec filters specs
// to only those with derived status "ready" or "ready_for_review", displays them
// as a numbered list with worktree/branch details for ready_for_review specs,
// and returns the selected spec path.
func TestScenario_SpecPickerWithMixedStatuses(t *testing.T) {
	// --- Seed ---

	// Create a temp specs directory with four .md files.
	specsDir := t.TempDir()
	for _, name := range []string{"alpha.md", "beta.md", "gamma.md", "delta.md"} {
		if err := os.WriteFile(filepath.Join(specsDir, name), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Create a store and populate with run states.
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// alpha: no runs → derived status "ready"
	// beta: one run with StatusReadyForReview
	betaRun := &runstore.RunState{
		RunID:        "run-beta-001",
		SpecID:       "beta",
		ProjectID:    "gromit",
		Status:       runstore.StatusReadyForReview,
		WorktreePath: "/tmp/wt",
		StartedAt:    time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC),
		Tasks:        []runstore.Task{},
	}
	if err := store.Save(betaRun); err != nil {
		t.Fatalf("save beta run: %v", err)
	}

	// gamma: one run with StatusCompleted → should be excluded
	gammaRun := &runstore.RunState{
		RunID:     "run-gamma-001",
		SpecID:    "gamma",
		ProjectID: "gromit",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	if err := store.Save(gammaRun); err != nil {
		t.Fatalf("save gamma run: %v", err)
	}

	// delta: one run with StatusRunning → should be excluded
	deltaRun := &runstore.RunState{
		RunID:     "run-delta-001",
		SpecID:    "delta",
		ProjectID: "gromit",
		Status:    runstore.StatusRunning,
		StartedAt: time.Date(2026, 3, 18, 8, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	if err := store.Save(deltaRun); err != nil {
		t.Fatalf("save delta run: %v", err)
	}

	// Stub branchResolver: returns "feature/foo" for /tmp/wt.
	branchResolver := func(worktreePath string) string {
		if worktreePath == "/tmp/wt" {
			return "feature/foo"
		}
		return "(unknown)"
	}

	// --- Invoke ---

	in := strings.NewReader("2\n")
	var out bytes.Buffer

	specPath, err := pickSpec("gromit", specsDir, store, branchResolver, in, &out)
	if err != nil {
		t.Fatalf("pickSpec: %v", err)
	}

	// --- Assert ---

	output := out.String()

	// Must contain exactly two numbered entries.
	if !strings.Contains(output, "1. alpha") {
		t.Errorf("expected '1. alpha' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2. beta") {
		t.Errorf("expected '2. beta' in output, got:\n%s", output)
	}

	// beta must be marked as ready_for_review with asterisk.
	if !strings.Contains(output, "* (ready_for_review)") {
		t.Errorf("expected '* (ready_for_review)' marker for beta, got:\n%s", output)
	}

	// beta must show worktree and branch on indented lines below the spec entry.
	wantWorktreeLine := "     worktree: /tmp/wt"
	wantBranchLine := "     branch:   feature/foo"
	if !strings.Contains(output, wantWorktreeLine) {
		t.Errorf("expected indented worktree line %q in output, got:\n%s", wantWorktreeLine, output)
	}
	if !strings.Contains(output, wantBranchLine) {
		t.Errorf("expected indented branch line %q in output, got:\n%s", wantBranchLine, output)
	}

	// gamma (completed) and delta (running) must NOT appear.
	if strings.Contains(output, "gamma") {
		t.Errorf("gamma (completed) should not appear in picker, got:\n%s", output)
	}
	if strings.Contains(output, "delta") {
		t.Errorf("delta (running) should not appear in picker, got:\n%s", output)
	}

	// Return value must be the full path to beta.md.
	wantPath := filepath.Join(specsDir, "beta.md")
	if specPath != wantPath {
		t.Errorf("pickSpec returned %q, want %q", specPath, wantPath)
	}
}
