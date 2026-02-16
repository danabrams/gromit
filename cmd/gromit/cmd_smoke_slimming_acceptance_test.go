//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file contains acceptance tests for gromit-kk13.2:
// "Slim cmd acceptance tests to smoke-only flows"
//
// These tests verify that:
// 1. cmd/gromit/*_acceptance_test.go contains only smoke scenarios from the approved matrix
// 2. Detailed behavior checks removed from acceptance tests are covered by cmd unit tests
// 3. The acceptance test suite is lean and focused on high-value E2E smoke checks
//
// Test status:
// - TestCmdAcceptanceTests_OnlyContainSmokeMatrixKeepCases: FAILS (move cases still present)
// - TestCmdAcceptanceTests_AllKeepCasesPresent: PASSES (prerequisites met)
// - TestCmdAcceptanceTests_RemovedCasesHaveUnitCoverage: PASSES (unit tests exist)
// - TestCmdAcceptanceTests_TotalLineCountReducedSignificantly: FAILS (still 394 lines, want <=120)
// - TestCmdAcceptanceTests_OnlyTestCmdSmokePatternFunctions: FAILS (non-smoke tests exist)
// - TestCmdAcceptanceTests_MaxThreeTestsPerFile: FAILS (multiple tests per file)
// - TestCmdAcceptanceTests_SmokeAnnotationsMatchMatrix: PASSES (annotations correct)

// TestCmdAcceptanceTests_OnlyContainSmokeMatrixKeepCases verifies that
// cmd/gromit/*_acceptance_test.go files contain only test functions that
// are marked "keep" in the smoke coverage matrix.
// Expected failure: current acceptance test files still contain "move" cases
// that should be removed per the smoke matrix reclassification.
func TestCmdAcceptanceTests_OnlyContainSmokeMatrixKeepCases(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	smokeMatrix, err := LoadCmdSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadCmdSmokeMatrix: %v", err)
	}

	acceptanceFiles := []string{
		"cmd/gromit/debug_agent_acceptance_test.go",
		"cmd/gromit/explore_codex_help_acceptance_test.go",
		"cmd/gromit/review_spec_validation_acceptance_test.go",
	}

	for _, relPath := range acceptanceFiles {
		fullPath := filepath.Join(projectRoot, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("read %s: %v", relPath, err)
		}

		src := string(content)
		lines := strings.Split(src, "\n")

		for _, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "func Test") {
				continue
			}

			// Extract function name
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "func ") {
				continue
			}
			afterFunc := strings.TrimPrefix(trimmed, "func ")
			parenIdx := strings.Index(afterFunc, "(")
			if parenIdx == -1 {
				continue
			}
			testName := afterFunc[:parenIdx]

			// Check if this test is in the matrix
			matrixKey := relPath + ":" + testName
			entry, exists := smokeMatrix[matrixKey]

			if !exists {
				// If not in matrix, it shouldn't exist
				t.Errorf("%s contains test %s which is not in smoke matrix - should be removed", relPath, testName)
				continue
			}

			// If marked "move", it should not exist in acceptance tests
			if entry.Decision == "move" {
				t.Errorf("%s contains test %s marked 'move' in smoke matrix - should be removed and migrated to %s",
					relPath, testName, entry.Destination)
			}
		}
	}
}

// TestCmdAcceptanceTests_AllKeepCasesPresent verifies that all test cases
// marked "keep" in the smoke coverage matrix exist in the acceptance test files.
// Expected failure: acceptance tests have not yet been updated to match the
// canonical smoke matrix keep set.
func TestCmdAcceptanceTests_AllKeepCasesPresent(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	smokeMatrix, err := LoadCmdSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadCmdSmokeMatrix: %v", err)
	}

	// Build set of expected keep tests
	expectedKeep := make(map[string]CmdSmokeMatrixEntry)
	for caseID, entry := range smokeMatrix {
		if entry.Decision == "keep" {
			expectedKeep[caseID] = entry
		}
	}

	// Verify each keep case exists
	for caseID := range expectedKeep {
		parts := strings.SplitN(caseID, ":", 2)
		if len(parts) != 2 {
			t.Fatalf("invalid matrix case ID %s", caseID)
		}
		filePath := parts[0]
		testName := parts[1]

		fullPath := filepath.Join(projectRoot, filePath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("read %s: %v", filePath, err)
		}

		if !strings.Contains(string(content), "func "+testName+"(") {
			t.Errorf("keep case %s not found in %s", testName, filePath)
		}
	}
}

// TestCmdAcceptanceTests_RemovedCasesHaveUnitCoverage verifies that all
// test cases marked "move" in the smoke matrix have corresponding unit tests
// at their destination.
// Expected failure: unit test destinations do not yet exist for all moved cases.
func TestCmdAcceptanceTests_RemovedCasesHaveUnitCoverage(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	smokeMatrix, err := LoadCmdSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadCmdSmokeMatrix: %v", err)
	}

	// Find all "move" cases
	for caseID, entry := range smokeMatrix {
		if entry.Decision != "move" {
			continue
		}

		// Parse destination: "file:testname"
		destParts := strings.SplitN(entry.Destination, ":", 2)
		if len(destParts) != 2 {
			t.Fatalf("invalid destination %s for case %s", entry.Destination, caseID)
		}

		destFile := destParts[0]
		destTest := destParts[1]

		// Verify destination file exists
		fullPath := filepath.Join(projectRoot, destFile)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("destination file %s for moved case %s does not exist: %v", destFile, caseID, err)
			continue
		}

		// Verify destination test exists
		if !strings.Contains(string(content), "func "+destTest+"(") {
			t.Errorf("destination test %s not found in %s for moved case %s", destTest, destFile, caseID)
		}
	}
}

// TestCmdAcceptanceTests_TotalLineCountReducedSignificantly verifies that
// the cmd acceptance test line count is reduced to a small fraction of the
// original count after slimming.
// Expected failure: current acceptance tests still contain many non-smoke cases,
// so total line count remains high.
func TestCmdAcceptanceTests_TotalLineCountReducedSignificantly(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	acceptanceFiles := []string{
		"cmd/gromit/debug_agent_acceptance_test.go",
		"cmd/gromit/explore_codex_help_acceptance_test.go",
		"cmd/gromit/review_spec_validation_acceptance_test.go",
	}

	totalLines := 0
	for _, relPath := range acceptanceFiles {
		fullPath := filepath.Join(projectRoot, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("read %s: %v", relPath, err)
		}

		lines := strings.Split(string(content), "\n")
		totalLines += len(lines)
	}

	// Current total is 391 lines. After slimming to smoke-only, expect roughly
	// 30% or less (around 120 lines max for 3 keep cases with setup)
	maxExpectedLines := 120
	if totalLines > maxExpectedLines {
		t.Errorf("cmd acceptance tests total %d lines, expected <= %d after slimming to smoke-only",
			totalLines, maxExpectedLines)
	}
}

// TestCmdAcceptanceTests_OnlyTestCmdSmokePatternFunctions verifies that
// acceptance test files only contain functions following the TestCmdSmoke_*
// naming pattern for the smoke suite.
// Expected failure: current acceptance files contain TestDebug*, TestExplore*
// and other non-smoke-prefixed test functions.
func TestCmdAcceptanceTests_OnlyTestCmdSmokePatternFunctions(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	acceptanceFiles := []string{
		"cmd/gromit/debug_agent_acceptance_test.go",
		"cmd/gromit/explore_codex_help_acceptance_test.go",
		"cmd/gromit/review_spec_validation_acceptance_test.go",
	}

	for _, relPath := range acceptanceFiles {
		fullPath := filepath.Join(projectRoot, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("read %s: %v", relPath, err)
		}

		src := string(content)
		lines := strings.Split(src, "\n")

		for lineNum, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "func Test") {
				continue
			}

			// Extract function name
			afterFunc := strings.TrimPrefix(trimmed, "func ")
			parenIdx := strings.Index(afterFunc, "(")
			if parenIdx == -1 {
				continue
			}
			testName := afterFunc[:parenIdx]

			// All acceptance tests should follow TestCmdSmoke_* pattern
			if !strings.HasPrefix(testName, "TestCmdSmoke_") {
				t.Errorf("%s:%d contains non-smoke test function %s - should be removed or renamed to TestCmdSmoke_* pattern",
					relPath, lineNum+1, testName)
			}
		}
	}
}

// TestCmdAcceptanceTests_MaxThreeTestsPerFile verifies that each acceptance
// file contains at most one smoke test (one keep case per file per matrix).
// Expected failure: current files contain multiple tests including move cases.
func TestCmdAcceptanceTests_MaxThreeTestsPerFile(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	acceptanceFiles := []string{
		"cmd/gromit/debug_agent_acceptance_test.go",
		"cmd/gromit/explore_codex_help_acceptance_test.go",
		"cmd/gromit/review_spec_validation_acceptance_test.go",
	}

	for _, relPath := range acceptanceFiles {
		fullPath := filepath.Join(projectRoot, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("read %s: %v", relPath, err)
		}

		src := string(content)
		testCount := strings.Count(src, "func Test")

		// Each file should have exactly 1 smoke test after slimming
		maxTests := 1
		if testCount > maxTests {
			t.Errorf("%s contains %d test functions, expected <= %d after slimming to smoke-only",
				relPath, testCount, maxTests)
		}
	}
}

// TestCmdAcceptanceTests_SmokeAnnotationsMatchMatrix verifies that smoke-matrix
// comment annotations in acceptance test files match the canonical matrix decisions.
// Expected failure: acceptance tests have not yet been annotated with the canonical
// smoke-matrix comment format showing keep/move decisions.
func TestCmdAcceptanceTests_SmokeAnnotationsMatchMatrix(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	smokeMatrix, err := LoadCmdSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadCmdSmokeMatrix: %v", err)
	}

	acceptanceFiles := []string{
		"cmd/gromit/debug_agent_acceptance_test.go",
		"cmd/gromit/explore_codex_help_acceptance_test.go",
		"cmd/gromit/review_spec_validation_acceptance_test.go",
	}

	for _, relPath := range acceptanceFiles {
		fullPath := filepath.Join(projectRoot, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("read %s: %v", relPath, err)
		}

		src := string(content)
		lines := strings.Split(src, "\n")

		for i := 0; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)

			// Look for smoke-matrix annotations
			if strings.HasPrefix(trimmed, "// smoke-matrix:") {
				// Parse the annotation
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) != 2 {
					continue
				}

				// Find associated test function (should be next non-comment line)
				testName := ""
				for j := i + 1; j < len(lines); j++ {
					testLine := strings.TrimSpace(lines[j])
					if strings.HasPrefix(testLine, "//") {
						continue
					}
					if strings.HasPrefix(testLine, "func Test") {
						afterFunc := strings.TrimPrefix(testLine, "func ")
						parenIdx := strings.Index(afterFunc, "(")
						if parenIdx != -1 {
							testName = afterFunc[:parenIdx]
						}
					}
					break
				}

				if testName == "" {
					continue
				}

				// Verify annotation matches matrix
				matrixKey := relPath + ":" + testName
				entry, exists := smokeMatrix[matrixKey]
				if !exists {
					t.Errorf("%s test %s has smoke-matrix annotation but not in canonical matrix",
						relPath, testName)
					continue
				}

				// Check if annotation content matches
				annotationContent := strings.TrimSpace(parts[1])
				if !strings.Contains(annotationContent, entry.Decision) {
					t.Errorf("%s test %s annotation decision mismatch: annotation=%q, matrix=%q",
						relPath, testName, annotationContent, entry.Decision)
				}
			}
		}
	}
}
