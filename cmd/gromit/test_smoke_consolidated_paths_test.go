package main

import (
	"testing"
)

// TestLoadConsolidatedSmokeMatrix_UsesFileLocationBasedResolution verifies that
// loadConsolidatedSmokeMatrix uses file-location-based path resolution so tests
// work correctly from any directory.
func TestLoadConsolidatedSmokeMatrix_UsesFileLocationBasedResolution(t *testing.T) {
	// Get the project root using file-location-based resolution
	projectRoot := getProjectRootFromTestFile("t")

	// Try to load the consolidated smoke matrix
	entries := loadConsolidatedSmokeMatrix(t, projectRoot)
	if len(entries) == 0 {
		t.Error("loadConsolidatedSmokeMatrix should find entries in the smoke coverage matrix")
	}
}

// TestCollectAcceptanceTests_UsesProperPathResolution verifies that
// collectAcceptanceTests works correctly with file-location-based resolution.
func TestCollectAcceptanceTests_UsesProperPathResolution(t *testing.T) {
	projectRoot := getProjectRootFromTestFile("t")

	files := []string{
		"cmd/gromit/debug_agent_acceptance_test.go",
	}

	cases := collectAcceptanceTests(t, projectRoot, files)
	if len(cases) == 0 {
		t.Error("collectAcceptanceTests should find test cases")
	}
}

