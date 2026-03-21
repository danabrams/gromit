package contract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScenario_DuplicateTestFilesRemovedWithoutLosingCoverage(t *testing.T) {
	// Seed: determine the contract package directory.
	// This test only inspects files in the contract package; it has no cross-package dependencies.
	contractDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	// Assert: specificity_test.go no longer exists in contract/
	oldFile := filepath.Join(contractDir, "specificity_test.go")
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("specificity_test.go should have been deleted from contract/, but still exists")
	}

	// Assert: all behaviors from the deleted file have scenario-named counterparts
	scenarioFiles := map[string]string{
		"specificity_scenario_single_identifier_test.go": "SingleExportedIdentifier → single identifier warning",
		"specificity_scenario_struct_scoped_test.go":     "MultiTokenPatternNoWarning → struct-scoped pattern passes",
		"specificity_scenario_unexported_test.go":        "UnexportedIdentifierNoWarning → unexported not flagged",
		"specificity_scenario_punctuation_test.go":       "PunctuationPatternNoWarning → punctuation pattern no warning",
		"specificity_scenario_file_not_contains_test.go": "FileNotContainsSkipped → file_not_contains not checked",
		"specificity_scenario_multiple_low_test.go":      "MultipleWarnings → multiple low-specificity patterns",
	}
	for file, behavior := range scenarioFiles {
		path := filepath.Join(contractDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("scenario counterpart %s missing (covers: %s)", file, behavior)
		}
	}
}
