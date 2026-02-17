package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const reviewSpecValidationReclassifiedFile = "cmd/gromit/review_spec_validation_reclassified_test.go"

func loadReviewSpecValidationReclassifiedSource(t *testing.T) string {
	t.Helper()

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	targetPath := filepath.Join(projectRoot, reviewSpecValidationReclassifiedFile)
	src, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("could not read %s: %v", reviewSpecValidationReclassifiedFile, err)
	}

	return string(src)
}

func lineCount(src string) int {
	if src == "" {
		return 0
	}
	return strings.Count(src, "\n") + 1
}

func TestReviewSpecValidationRefactor_UsesTableDrivenScenarioGroups(t *testing.T) {
	// Expected failure: TestReviewSpecValidationScenarios_TableDriven and
	// buildReviewSpecValidationCases do not exist yet in
	// cmd/gromit/review_spec_validation_reclassified_test.go.
	src := loadReviewSpecValidationReclassifiedSource(t)

	if !strings.Contains(src, "func TestReviewSpecValidationScenarios_TableDriven") {
		t.Fatalf("%s should define TestReviewSpecValidationScenarios_TableDriven", reviewSpecValidationReclassifiedFile)
	}

	if !strings.Contains(src, "buildReviewSpecValidationCases") {
		t.Fatalf("%s should use buildReviewSpecValidationCases to construct table scenarios", reviewSpecValidationReclassifiedFile)
	}

	if !strings.Contains(src, "for _, tc := range cases") {
		t.Fatalf("%s should iterate table cases with for _, tc := range cases", reviewSpecValidationReclassifiedFile)
	}
}

func TestReviewSpecValidationRefactorReclassified_UsesSharedHelpers(t *testing.T) {
	// Expected failure: setupReviewSpecValidationFixture and
	// assertSpecValidationError helper functions do not exist yet.
	src := loadReviewSpecValidationReclassifiedSource(t)

	requiredHelpers := []string{
		"func setupReviewSpecValidationFixture(",
		"func assertSpecValidationError(",
	}

	for _, helper := range requiredHelpers {
		if !strings.Contains(src, helper) {
			t.Fatalf("%s should contain helper %q", reviewSpecValidationReclassifiedFile, helper)
		}
	}
}

func TestReviewSpecValidationRefactor_ReducesLinesAndKeepsDeterministicCoverage(t *testing.T) {
	// Expected failure: maxReviewSpecValidationAcceptanceLines and
	// reviewSpecValidationCoverageSentinels are not enforced yet via consolidated structure.
	src := loadReviewSpecValidationReclassifiedSource(t)

	const maxReviewSpecValidationAcceptanceLines = 420
	if got := lineCount(src); got > maxReviewSpecValidationAcceptanceLines {
		t.Fatalf("%s should be <= %d lines after consolidation, got %d", reviewSpecValidationReclassifiedFile, maxReviewSpecValidationAcceptanceLines, got)
	}

	requiredCoverageSentinels := []string{
		"nonexistent spec",
		"typo",
		"empty specs directory",
		"non-markdown",
		"existing spec",
	}
	for _, sentinel := range requiredCoverageSentinels {
		if !strings.Contains(strings.ToLower(src), sentinel) {
			t.Fatalf("%s should preserve scenario coverage marker %q", reviewSpecValidationReclassifiedFile, sentinel)
		}
	}

	forbiddenNondeterminism := []string{
		"time.Now(",
		"rand.",
		"map[string]func(",
	}
	for _, pattern := range forbiddenNondeterminism {
		if strings.Contains(src, pattern) {
			t.Fatalf("%s should remain deterministic; found %q", reviewSpecValidationReclassifiedFile, pattern)
		}
	}
}
