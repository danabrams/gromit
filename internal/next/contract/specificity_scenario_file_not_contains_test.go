package contract

import (
	"testing"
)

func TestScenario_FileNotContainsAssertionNotChecked(t *testing.T) {
	// Seed: a contract with a file_not_contains assertion using a single exported identifier
	// that would trigger a warning if it were a file_contains assertion.
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "deprecation-removed",
				Assertions: []ContractAssertion{
					{
						FileNotContains: &FileContainsAssertion{
							Path:    "types.go",
							Pattern: "DeprecatedField",
						},
					},
				},
			},
		},
	}

	// Invoke
	warnings := ValidateContractSpecificity(c)

	// Assert: no warning returned — only file_contains assertions are checked
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for file_not_contains assertion, got %d: %v", len(warnings), warnings)
	}
}
