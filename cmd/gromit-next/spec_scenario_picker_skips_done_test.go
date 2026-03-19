package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_SpecPickerSkipsDoneSpecs(t *testing.T) {
	// Seed: create a specs directory with two spec files
	specsDir := t.TempDir()

	// 0003a.md starts with "DONE 2026-03-19"
	err := os.WriteFile(
		filepath.Join(specsDir, "0003a.md"),
		[]byte("DONE 2026-03-19\n# Spec 0003a — Infrastructure Failure Detection\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("write 0003a.md: %v", err)
	}

	// 0003h.md has no prefix
	err = os.WriteFile(
		filepath.Join(specsDir, "0003h.md"),
		[]byte("# Spec 0003h — Contract File Self-Correction\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("write 0003h.md: %v", err)
	}

	// Create a store with no runs (both specs would be "ready" if not filtered)
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Invoke: call pickSpec with input selecting option 1
	var out bytes.Buffer
	in := strings.NewReader("1\n")
	branchResolver := func(path string) string { return "main" }

	result, err := pickSpec("test-project", specsDir, store, branchResolver, in, &out)
	if err != nil {
		t.Fatalf("pickSpec: %v", err)
	}

	// Assert: only 0003h should appear in the picker list
	output := out.String()

	if strings.Contains(output, "0003a") {
		t.Errorf("picker should not show done spec 0003a, got:\n%s", output)
	}

	if !strings.Contains(output, "0003h") {
		t.Errorf("picker should show 0003h, got:\n%s", output)
	}

	// The selected spec (option 1) should be 0003h
	expectedPath := filepath.Join(specsDir, "0003h.md")
	if result != expectedPath {
		t.Errorf("expected selected path %q, got %q", expectedPath, result)
	}
}
