package contract

import (
	"testing"
)

func TestValidateContractSpecificity_SingleExportedIdentifier(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "test-scenario",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "some/file.go",
							Pattern: "ModelTier",
						},
					},
				},
			},
		},
	}

	warnings := ValidateContractSpecificity(c)

	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for single exported identifier, got %d", len(warnings))
	}
	if len(warnings) > 0 {
		w := warnings[0]
		if w.ScenarioName != "test-scenario" {
			t.Errorf("expected ScenarioName 'test-scenario', got %q", w.ScenarioName)
		}
		if w.AssertionIdx != 0 {
			t.Errorf("expected AssertionIdx 0, got %d", w.AssertionIdx)
		}
		if w.Pattern != "ModelTier" {
			t.Errorf("expected Pattern 'ModelTier', got %q", w.Pattern)
		}
		if w.Path != "some/file.go" {
			t.Errorf("expected Path 'some/file.go', got %q", w.Path)
		}
		if w.Reason == "" {
			t.Errorf("expected non-empty Reason, got empty string")
		}
	}
}

func TestValidateContractSpecificity_MultiTokenPatternNoWarning(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "test-scenario",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "some/file.go",
							Pattern: "ModelTier  string",
						},
					},
				},
			},
		},
	}

	warnings := ValidateContractSpecificity(c)

	if len(warnings) != 0 {
		t.Errorf("expected no warning for multi-token pattern, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateContractSpecificity_UnexportedIdentifierNoWarning(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "test-scenario",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "some/file.go",
							Pattern: "modelTier",
						},
					},
				},
			},
		},
	}

	warnings := ValidateContractSpecificity(c)

	if len(warnings) != 0 {
		t.Errorf("expected no warning for unexported identifier, got %d: %v", len(warnings), warnings)
	}
}

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

func TestValidateContractSpecificity_FileNotContainsSkipped(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "test-scenario",
				Assertions: []ContractAssertion{
					{
						FileNotContains: &FileContainsAssertion{
							Path:    "some/file.go",
							Pattern: "DeprecatedField",
						},
					},
				},
			},
		},
	}

	warnings := ValidateContractSpecificity(c)

	if len(warnings) != 0 {
		t.Errorf("expected no warning for file_not_contains assertion, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateContractSpecificity_MultipleWarnings(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "test-scenario",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "some/file.go",
							Pattern: "ModelTier",
						},
					},
					{
						FileContains: &FileContainsAssertion{
							Path:    "other/file.go",
							Pattern: "Proposal",
						},
					},
					{
						FileContains: &FileContainsAssertion{
							Path:    "struct/file.go",
							Pattern: "type DistillationResult struct",
						},
					},
				},
			},
		},
	}

	warnings := ValidateContractSpecificity(c)

	if len(warnings) != 2 {
		t.Errorf("expected exactly 2 warnings (for 'ModelTier' and 'Proposal'), got %d", len(warnings))
	}
	if len(warnings) > 0 {
		// First warning: ModelTier
		if warnings[0].Pattern != "ModelTier" {
			t.Errorf("expected first warning Pattern 'ModelTier', got %q", warnings[0].Pattern)
		}
		if warnings[0].Path != "some/file.go" {
			t.Errorf("expected first warning Path 'some/file.go', got %q", warnings[0].Path)
		}
		if warnings[0].AssertionIdx != 0 {
			t.Errorf("expected first warning AssertionIdx 0, got %d", warnings[0].AssertionIdx)
		}
	}
	if len(warnings) > 1 {
		// Second warning: Proposal
		if warnings[1].Pattern != "Proposal" {
			t.Errorf("expected second warning Pattern 'Proposal', got %q", warnings[1].Pattern)
		}
		if warnings[1].Path != "other/file.go" {
			t.Errorf("expected second warning Path 'other/file.go', got %q", warnings[1].Path)
		}
		if warnings[1].AssertionIdx != 1 {
			t.Errorf("expected second warning AssertionIdx 1, got %d", warnings[1].AssertionIdx)
		}
	}
}
