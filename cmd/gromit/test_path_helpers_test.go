package main

import (
	"os"
	"testing"
)

// TestResolveProjectPath_ResolvesPaths_Independently_Of_WorkingDirectory verifies that
// paths are resolved correctly regardless of the working directory when the test runs.
func TestResolveProjectPath_ResolvesPaths_Independently_Of_WorkingDirectory(t *testing.T) {
	// This test verifies that resolveProjectPath correctly resolves paths relative to the
	// project root, regardless of the current working directory when tests are run.

	// Try to read RULES.md using the resolver
	rulesPath := resolveProjectPath("t", ".gromit/RULES.md")
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("resolveProjectPath should find RULES.md, but got error: %v", err)
	}

	// Verify it actually contains RULES content
	if len(data) == 0 {
		t.Fatal("RULES.md should not be empty")
	}
}

// TestResolveProjectPath_ReturnsSame_Path_ForMultipleCalls verifies that
// the resolver returns consistent results across multiple calls.
func TestResolveProjectPath_ReturnsSame_Path_ForMultipleCalls(t *testing.T) {
	path1 := resolveProjectPath("t", ".gromit/RULES.md")
	path2 := resolveProjectPath("t", ".gromit/RULES.md")

	if path1 != path2 {
		t.Fatalf("resolveProjectPath should return consistent paths: %q vs %q", path1, path2)
	}
}

// TestResolveProjectPath_Resolves_Multiple_Files verifies that the resolver
// works with different file paths.
func TestResolveProjectPath_Resolves_Multiple_Files(t *testing.T) {
	files := []string{
		".gromit/RULES.md",
		"CLAUDE.md",
		"skills/gromit-decompose/SKILL.md",
		"docs/smoke_coverage_matrix.md",
	}

	for _, file := range files {
		path := resolveProjectPath("t", file)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("resolveProjectPath should find %q at %q, but got error: %v", file, path, err)
		}
	}
}
