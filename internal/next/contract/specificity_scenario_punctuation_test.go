package contract

import (
	"testing"
)

func TestValidateContractSpecificity_PunctuationPatternNoWarning(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "test-scenario",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "some/file.go",
							Pattern: "func Validate(",
						},
					},
				},
			},
		},
	}

	warnings := ValidateContractSpecificity(c)

	if len(warnings) != 0 {
		t.Errorf("expected no warning for punctuation pattern, got %d: %v", len(warnings), warnings)
	}
}
