package contract

import (
	"testing"
)

func TestScenario_RegexCompiledOnceAtPackageLevel(t *testing.T) {
	// Seed: a contract with a single exported identifier pattern (triggers warning)
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "first-call",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "pkg/foo.go",
							Pattern: "MyStruct",
						},
					},
				},
			},
		},
	}

	// Invoke: call ValidateContractSpecificity twice using the same package-level regex
	warnings1 := ValidateContractSpecificity(c)
	warnings2 := ValidateContractSpecificity(c)

	// Assert: both calls produce consistent results
	if len(warnings1) != 1 {
		t.Fatalf("first call: expected 1 warning, got %d", len(warnings1))
	}
	if len(warnings2) != 1 {
		t.Fatalf("second call: expected 1 warning, got %d", len(warnings2))
	}
	if warnings1[0].Pattern != warnings2[0].Pattern {
		t.Errorf("inconsistent pattern: first=%q, second=%q", warnings1[0].Pattern, warnings2[0].Pattern)
	}
}
