package contract

import (
	"testing"
)

func TestScenario_MultipleLowSpecificityPatterns(t *testing.T) {
	// Seed: contract with three file_contains assertions —
	// two bare exported identifiers and one struct declaration
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "distillation-types",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "internal/next/types.go",
							Pattern: "ModelTier",
						},
					},
					{
						FileContains: &FileContainsAssertion{
							Path:    "internal/next/types.go",
							Pattern: "Proposal",
						},
					},
					{
						FileContains: &FileContainsAssertion{
							Path:    "internal/next/types.go",
							Pattern: "type DistillationResult struct",
						},
					},
				},
			},
		},
	}

	// Invoke
	warnings := ValidateContractSpecificity(c)

	// Assert: exactly 2 warnings for ModelTier and Proposal, none for the struct declaration
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}

	// Assert: first warning is for ModelTier
	if warnings[0].Pattern != "ModelTier" {
		t.Errorf("expected first warning pattern 'ModelTier', got %q", warnings[0].Pattern)
	}

	// Assert: second warning is for Proposal
	if warnings[1].Pattern != "Proposal" {
		t.Errorf("expected second warning pattern 'Proposal', got %q", warnings[1].Pattern)
	}
}
