package contract

import (
	"testing"
)

func TestScenario_UnexportedIdentifierNotFlagged(t *testing.T) {
	// Seed: a contract with file_contains using an unexported identifier pattern
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "unexported-id",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "internal/foo/bar.go",
							Pattern: "modelTier",
						},
					},
				},
			},
		},
	}

	// Invoke
	warnings := ValidateContractSpecificity(c)

	// Assert: no warning returned — unexported identifiers are less likely
	// to be ambiguous across multiple type definitions
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for unexported identifier 'modelTier', got %d: %v", len(warnings), warnings)
	}
}
