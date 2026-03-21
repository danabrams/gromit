package stages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScenario_DuplicateTestFilesRemovalInStages(t *testing.T) {
	// Seed: determine the stages package directory
	stagesDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	// Assert: the 3 duplicate Specificity tests are removed from write_contracts_test.go
	stagesTestPath := filepath.Join(stagesDir, "write_contracts_test.go")
	stagesTestBytes, err := os.ReadFile(stagesTestPath)
	if err != nil {
		t.Fatalf("read write_contracts_test.go: %v", err)
	}
	stagesTestContent := string(stagesTestBytes)

	deletedTests := []string{
		"TestWriteContracts_SpecificityNoWarningsNoRetry",
		"TestWriteContracts_SpecificityRetryFixesPattern",
		"TestWriteContracts_SpecificityRetryPersistsWarning",
	}
	for _, name := range deletedTests {
		if strings.Contains(stagesTestContent, "func "+name+"(") {
			t.Errorf("duplicate test %s should have been removed from write_contracts_test.go", name)
		}
	}

	// Assert: StructuralRegression and LLMError tests still exist in stages/
	keptTests := []string{
		"TestWriteContracts_SpecificityRetryStructuralRegression",
		"TestWriteContracts_SpecificityRetryLLMError",
	}
	for _, name := range keptTests {
		if !strings.Contains(stagesTestContent, "func "+name+"(") {
			t.Errorf("test %s must still exist in write_contracts_test.go", name)
		}
	}
}
