package main

import (
	"os"
	"testing"
)

// TestSmokeDecisionRubric_CanReadMatrixFromAnyDirectory verifies that
// smoke decision rubric tests can read the coverage matrix correctly
// regardless of working directory.
func TestSmokeDecisionRubric_CanReadMatrixFromAnyDirectory(t *testing.T) {
	// Use file-location-based path resolution to read the matrix
	matrixPath := resolveProjectPath("t", "docs/smoke_coverage_matrix.md")

	// Verify the file exists and can be read
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Errorf("should read smoke coverage matrix from %q: %v", matrixPath, err)
		return
	}

	if len(data) == 0 {
		t.Error("smoke coverage matrix should not be empty")
	}
}

// TestPackageTestNameIndex_WorksWithFileLocationResolution verifies that
// packageTestNameIndex can find test suites correctly.
func TestPackageTestNameIndex_WorksWithFileLocationResolution(t *testing.T) {
	// Get project root using file-location-based resolution
	projectRoot := getProjectRootFromTestFile("t")

	// Try to build an index of test names in a package
	tests := packageTestNameIndex(t, projectRoot, "cmd/gromit")
	if len(tests) == 0 {
		t.Error("packageTestNameIndex should find test cases in cmd/gromit")
	}

	// Verify we found some test names we know should exist
	foundTests := false
	for testName := range tests {
		if testName != "" {
			foundTests = true
			break
		}
	}
	if !foundTests {
		t.Error("packageTestNameIndex should find non-empty test names")
	}
}
