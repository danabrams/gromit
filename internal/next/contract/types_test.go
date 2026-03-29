package contract

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSpecScenario(t *testing.T) {
	s := SpecScenario{
		Name:  "add-works",
		Given: "a calculator",
		When:  "I add 1 and 2",
		Then:  "the result is 3",
		Notes: "basic smoke test",
	}
	if s.Name != "add-works" {
		t.Errorf("Name = %q, want %q", s.Name, "add-works")
	}
	if s.Notes != "basic smoke test" {
		t.Errorf("Notes = %q, want %q", s.Notes, "basic smoke test")
	}
}

func TestScenarioContract(t *testing.T) {
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []ContractAssertion{
					{FileExists: "output.txt"},
				},
			},
		},
	}

	data, err := yaml.Marshal(&contract)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	var got ScenarioContract
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if len(got.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(got.Scenarios))
	}
	if got.Scenarios[0].Name != "add-works" {
		t.Errorf("Name = %q, want %q", got.Scenarios[0].Name, "add-works")
	}
	if len(got.Scenarios[0].Assertions) != 1 {
		t.Fatalf("len(Assertions) = %d, want 1", len(got.Scenarios[0].Assertions))
	}
	if got.Scenarios[0].Assertions[0].FileExists != "output.txt" {
		t.Errorf("FileExists = %q, want %q", got.Scenarios[0].Assertions[0].FileExists, "output.txt")
	}
}

func TestContractAssertion(t *testing.T) {
	tests := []struct {
		name      string
		assertion ContractAssertion
		wantKey   string
	}{
		{
			name:      "file_exists",
			assertion: ContractAssertion{FileExists: "foo.go"},
			wantKey:   "file_exists",
		},
		{
			name: "file_contains",
			assertion: ContractAssertion{FileContains: &FileContainsAssertion{
				Path:    "foo.go",
				Pattern: "hello",
			}},
			wantKey: "file_contains",
		},
		{
			name:      "file_not_modified",
			assertion: ContractAssertion{FileNotModified: "bar.go"},
			wantKey:   "file_not_modified",
		},
		{
			name:      "file_not_exists",
			assertion: ContractAssertion{FileNotExists: "baz.go"},
			wantKey:   "file_not_exists",
		},
		{
			name: "file_not_contains",
			assertion: ContractAssertion{FileNotContains: &FileContainsAssertion{
				Path:    "baz.go",
				Pattern: "secret",
			}},
			wantKey: "file_not_contains",
		},
		{
			name: "go_test_pass",
			assertion: ContractAssertion{GoTestPass: &GoTestPassAssertion{
				Pkg:      "./...",
				TestName: "TestFooBehavior",
			}},
			wantKey: "go_test_pass",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := yaml.Marshal(&tc.assertion)
			if err != nil {
				t.Fatalf("yaml.Marshal: %v", err)
			}
			if len(data) == 0 {
				t.Fatal("expected non-empty YAML output")
			}

			var got ContractAssertion
			if err := yaml.Unmarshal(data, &got); err != nil {
				t.Fatalf("yaml.Unmarshal: %v", err)
			}
			// Re-marshal to compare
			data2, err := yaml.Marshal(&got)
			if err != nil {
				t.Fatalf("yaml.Marshal round-trip: %v", err)
			}
			if string(data) != string(data2) {
				t.Errorf("round-trip mismatch:\n  got:  %s\n  want: %s", data2, data)
			}
		})
	}
}

func TestContractFailure(t *testing.T) {
	f := ContractFailure{
		ScenarioName:  "subtract-works",
		AssertionType: "file_contains",
		Details:       "expected 'result: -1' in output.txt",
	}
	if f.ScenarioName != "subtract-works" {
		t.Errorf("ScenarioName = %q, want %q", f.ScenarioName, "subtract-works")
	}
	if f.AssertionType != "file_contains" {
		t.Errorf("AssertionType = %q, want %q", f.AssertionType, "file_contains")
	}
}
