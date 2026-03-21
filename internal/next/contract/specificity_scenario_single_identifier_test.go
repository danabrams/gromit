package contract

import (
	"strings"
	"testing"
)

func TestScenario_SingleIdentifierPatternTriggersSpecificityWarning(t *testing.T) {
	// Seed: LLM generates a contract with a single exported identifier pattern
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "single-identifier-warning",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "internal/next/reviewdistiller/types.go",
							Pattern: "ModelTier",
						},
					},
				},
			},
		},
	}

	// Invoke
	warnings := ValidateContractSpecificity(c)

	// Assert: exactly one warning returned
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}

	// Assert: warning is for the correct scenario
	if warnings[0].ScenarioName != "single-identifier-warning" {
		t.Errorf("expected scenario name 'single-identifier-warning', got %q", warnings[0].ScenarioName)
	}

	// Assert: warning pattern and reason are set correctly
	if warnings[0].Pattern != "ModelTier" {
		t.Errorf("expected pattern 'ModelTier', got %q", warnings[0].Pattern)
	}
	if !strings.Contains(warnings[0].Reason, "single exported identifier") {
		t.Errorf("expected reason to indicate single exported identifier, got %q", warnings[0].Reason)
	}
}
