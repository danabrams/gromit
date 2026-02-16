package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const reviewSpecValidationAcceptanceFile = "cmd/gromit/review_spec_validation_acceptance_test.go"

func loadReviewSpecValidationAcceptanceSource(t *testing.T) string {
	t.Helper()

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	targetPath := filepath.Join(projectRoot, reviewSpecValidationAcceptanceFile)
	src, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("could not read %s: %v", reviewSpecValidationAcceptanceFile, err)
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
	// cmd/gromit/review_spec_validation_acceptance_test.go.
	src := loadReviewSpecValidationAcceptanceSource(t)

	if !strings.Contains(src, "func TestReviewSpecValidationScenarios_TableDriven") {
		t.Fatalf("%s should define TestReviewSpecValidationScenarios_TableDriven", reviewSpecValidationAcceptanceFile)
	}

	if !strings.Contains(src, "buildReviewSpecValidationCases") {
		t.Fatalf("%s should use buildReviewSpecValidationCases to construct table scenarios", reviewSpecValidationAcceptanceFile)
	}

	if !strings.Contains(src, "for _, tc := range cases") {
		t.Fatalf("%s should iterate table cases with for _, tc := range cases", reviewSpecValidationAcceptanceFile)
	}
}

func TestReviewSpecValidationRefactor_ExtractsSharedFixtureAndAssertionHelpers(t *testing.T) {
	// Expected failure: setupReviewSpecValidationFixture and
	// assertSpecValidationError helper functions do not exist yet.
	src := loadReviewSpecValidationAcceptanceSource(t)

	requiredHelpers := []string{
		"func setupReviewSpecValidationFixture(",
		"func assertSpecValidationError(",
	}

	for _, helper := range requiredHelpers {
		if !strings.Contains(src, helper) {
			t.Fatalf("%s should contain helper %q", reviewSpecValidationAcceptanceFile, helper)
		}
	}
}

func TestReviewSpecValidationRefactor_ReducesLinesAndKeepsDeterministicCoverage(t *testing.T) {
	// Expected failure: maxReviewSpecValidationAcceptanceLines and
	// reviewSpecValidationCoverageSentinels are not enforced yet via consolidated structure.
	src := loadReviewSpecValidationAcceptanceSource(t)

	const maxReviewSpecValidationAcceptanceLines = 420
	if got := lineCount(src); got > maxReviewSpecValidationAcceptanceLines {
		t.Fatalf("%s should be <= %d lines after consolidation, got %d", reviewSpecValidationAcceptanceFile, maxReviewSpecValidationAcceptanceLines, got)
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
			t.Fatalf("%s should preserve scenario coverage marker %q", reviewSpecValidationAcceptanceFile, sentinel)
		}
	}

	forbiddenNondeterminism := []string{
		"time.Now(",
		"rand.",
		"map[string]func(",
	}
	for _, pattern := range forbiddenNondeterminism {
		if strings.Contains(src, pattern) {
			t.Fatalf("%s should remain deterministic; found %q", reviewSpecValidationAcceptanceFile, pattern)
		}
	}
}
