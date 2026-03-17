package contract

import (
	"testing"
)

func TestValidateContract_ValidSingleField(t *testing.T) {
	tests := []struct {
		name      string
		assertion ContractAssertion
	}{
		{
			name:      "file_exists",
			assertion: ContractAssertion{FileExists: "some/file.go"},
		},
		{
			name: "file_contains",
			assertion: ContractAssertion{FileContains: &FileContainsAssertion{
				Path:    "some/file.go",
				Pattern: "hello",
			}},
		},
		{
			name:      "file_not_modified",
			assertion: ContractAssertion{FileNotModified: "some/file.go"},
		},
		{
			name:      "file_not_exists",
			assertion: ContractAssertion{FileNotExists: "some/file.go"},
		},
		{
			name: "file_not_contains",
			assertion: ContractAssertion{FileNotContains: &FileContainsAssertion{
				Path:    "some/file.go",
				Pattern: "hello",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ScenarioContract{
				Scenarios: []ScenarioAssertions{
					{Name: "s1", Assertions: []ContractAssertion{tt.assertion}},
				},
			}
			errs := ValidateContract(c)
			if len(errs) != 0 {
				t.Errorf("expected no errors, got %v", errs)
			}
		})
	}
}

func TestValidateContract_ZeroFieldsSet(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{Name: "s1", Assertions: []ContractAssertion{
				{}, // no fields set
			}},
		},
	}
	errs := ValidateContract(c)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestValidateContract_MultipleFieldsSet(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{Name: "s1", Assertions: []ContractAssertion{
				{
					FileExists:    "a.go",
					FileNotExists: "b.go",
				},
			}},
		},
	}
	errs := ValidateContract(c)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestValidateContract_MixedValidAndInvalid(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "s1",
				Assertions: []ContractAssertion{
					{FileExists: "ok.go"}, // valid
					{},                   // zero fields — invalid
					{FileNotExists: "x"}, // valid
				},
			},
			{
				Name: "s2",
				Assertions: []ContractAssertion{
					{FileExists: "a.go", FileNotExists: "b.go"}, // multiple fields — invalid
					{FileExists: "ok.go"},                       // valid
				},
			},
		},
	}
	errs := ValidateContract(c)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}
