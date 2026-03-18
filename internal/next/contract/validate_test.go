package contract

import (
	"strings"
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

func TestValidateContract_RuntimeArtifacts_BareFilenames(t *testing.T) {
	tests := []struct {
		name             string
		runtimeFilename  string
		assertionFactory func(path string) ContractAssertion
	}{
		{
			name:            "run.json with file_exists",
			runtimeFilename: "run.json",
			assertionFactory: func(path string) ContractAssertion {
				return ContractAssertion{FileExists: path}
			},
		},
		{
			name:            "execution-policy.json with file_contains",
			runtimeFilename: "execution-policy.json",
			assertionFactory: func(path string) ContractAssertion {
				return ContractAssertion{FileContains: &FileContainsAssertion{Path: path, Pattern: "test"}}
			},
		},
		{
			name:            "tasks.json with file_not_exists",
			runtimeFilename: "tasks.json",
			assertionFactory: func(path string) ContractAssertion {
				return ContractAssertion{FileNotExists: path}
			},
		},
		{
			name:            "events.jsonl with file_not_contains",
			runtimeFilename: "events.jsonl",
			assertionFactory: func(path string) ContractAssertion {
				return ContractAssertion{FileNotContains: &FileContainsAssertion{Path: path, Pattern: "test"}}
			},
		},
		{
			name:            "spec-packet.md with file_not_modified",
			runtimeFilename: "spec-packet.md",
			assertionFactory: func(path string) ContractAssertion {
				return ContractAssertion{FileNotModified: path}
			},
		},
		{
			name:            "spec.md with file_exists",
			runtimeFilename: "spec.md",
			assertionFactory: func(path string) ContractAssertion {
				return ContractAssertion{FileExists: path}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ScenarioContract{
				Scenarios: []ScenarioAssertions{
					{
						Name: "test-scenario",
						Assertions: []ContractAssertion{
							tt.assertionFactory(tt.runtimeFilename),
						},
					},
				},
			}
			errs := ValidateContract(c)
			if len(errs) != 1 {
				t.Fatalf("expected 1 error for %q, got %d: %v", tt.runtimeFilename, len(errs), errs)
			}
			if !strings.Contains(errs[0], "runtime artifact") {
				t.Errorf("expected error to mention runtime artifact, got: %q", errs[0])
			}
		})
	}
}

func TestValidateContract_RuntimeArtifacts_WithGromitNextPrefix(t *testing.T) {
	tests := []struct {
		name            string
		prefixedPath    string
		assertionField  string
	}{
		{
			name:           "run.json with .gromit-next prefix in file_exists",
			prefixedPath:   ".gromit-next/run.json",
			assertionField: "file_exists",
		},
		{
			name:           "execution-policy.json with .gromit-next prefix in file_contains",
			prefixedPath:   ".gromit-next/execution-policy.json",
			assertionField: "file_contains",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c ScenarioContract
			if tt.assertionField == "file_exists" {
				c = ScenarioContract{
					Scenarios: []ScenarioAssertions{
						{
							Name: "test-scenario",
							Assertions: []ContractAssertion{
								{FileExists: tt.prefixedPath},
							},
						},
					},
				}
			} else if tt.assertionField == "file_contains" {
				c = ScenarioContract{
					Scenarios: []ScenarioAssertions{
						{
							Name: "test-scenario",
							Assertions: []ContractAssertion{
								{FileContains: &FileContainsAssertion{Path: tt.prefixedPath, Pattern: "test"}},
							},
						},
					},
				}
			}

			errs := ValidateContract(c)
			if len(errs) != 1 {
				t.Fatalf("expected 1 error for %q, got %d: %v", tt.prefixedPath, len(errs), errs)
			}
			if !strings.Contains(errs[0], "runtime artifact") {
				t.Errorf("expected error to mention runtime artifact, got: %q", errs[0])
			}
		})
	}
}

func TestValidateContract_ValidSourceCodePaths(t *testing.T) {
	c := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "valid-paths",
				Assertions: []ContractAssertion{
					{FileExists: "internal/package/file.go"},
					{FileExists: "cmd/tool/main.go"},
					{FileContains: &FileContainsAssertion{Path: "pkg/module/code.go", Pattern: "test"}},
					{FileNotModified: "go.mod"},
				},
			},
		},
	}
	errs := ValidateContract(c)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid source paths, got %d: %v", len(errs), errs)
	}
}
