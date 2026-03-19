package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestScenario_SpecPickerExcludesDoneSpecs verifies that pickSpec filters out
// specs with "DONE" prefix, regardless of their run history.
//
// Scenario: spec picker with DONE prefix filtering
// Given: Two spec files — one with DONE prefix, one ready
// When: pickSpec is called with input "1\n"
// Then: Only the ready spec appears in the picker output; the DONE spec
//
//	is excluded based on its content prefix.
func TestScenario_SpecPickerExcludesDoneSpecs(t *testing.T) {
	// --- Seed ---

	// Create a temp specs directory with two .md files.
	specsDir := t.TempDir()

	// Create a DONE spec — starts with "DONE" prefix
	doneContent := "DONE 2026-03-15\n\n# Completed spec\n\nThis spec is marked as done.\n"
	if err := os.WriteFile(filepath.Join(specsDir, "completed-feature.md"), []byte(doneContent), 0o644); err != nil {
		t.Fatalf("write completed-feature.md: %v", err)
	}

	// Create a ready spec — no prefix, available to run
	readyContent := "# Next Feature\n\nThis spec is ready to implement.\n"
	if err := os.WriteFile(filepath.Join(specsDir, "next-feature.md"), []byte(readyContent), 0o644); err != nil {
		t.Fatalf("write next-feature.md: %v", err)
	}

	// Create a store with no runs (both specs will derive status from content alone)
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Stub branchResolver
	branchResolver := func(worktreePath string) string {
		return "(unknown)"
	}

	// --- Invoke ---

	in := strings.NewReader("1\n")
	var out bytes.Buffer

	specPath, err := pickSpec("gromit", specsDir, store, branchResolver, in, &out)
	if err != nil {
		t.Fatalf("pickSpec: %v", err)
	}

	// --- Assert ---

	output := out.String()

	// The DONE spec must NOT appear in the picker output
	if strings.Contains(output, "completed-feature") {
		t.Errorf("DONE spec 'completed-feature' should be excluded from picker, got:\n%s", output)
	}

	// The ready spec must appear in the output
	if !strings.Contains(output, "next-feature") {
		t.Errorf("expected 'next-feature' in output, got:\n%s", output)
	}

	// Must show exactly one numbered entry
	if !strings.Contains(output, "1. next-feature") {
		t.Errorf("expected '1. next-feature' in output, got:\n%s", output)
	}

	// Return value must be the full path to next-feature.md
	wantPath := filepath.Join(specsDir, "next-feature.md")
	if specPath != wantPath {
		t.Errorf("pickSpec returned %q, want %q", specPath, wantPath)
	}
}
