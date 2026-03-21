package contract

import (
	"testing"
)

func TestScenario_StructScopedPatternPassesSpecificityCheck(t *testing.T) {
	// Seed: a contract with a struct-scoped pattern (multi-token: "ModelTier  string")
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "struct-scoped-pattern",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "internal/next/reviewdistiller/types.go",
							Pattern: "ModelTier  string",
						},
					},
				},
			},
		},
	}

	// Invoke
	warnings := ValidateContractSpecificity(c)

	// Assert: no warning returned for the struct-scoped pattern
	if len(warnings) != 0 {
		t.Fatalf("expected no specificity warnings for struct-scoped pattern, got %d: %v", len(warnings), warnings)
	}
}
