package contract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScenario_AugmentWithTestAssertions_MixedSomeWithTestsSomeWithout(t *testing.T) {
	// Seed
	workDir := t.TempDir()
	testSrc := `package contract_test

import "testing"

func TestScenario_ReworkResume(t *testing.T) {}
`
	testPath := filepath.Join(workDir, "augment_scenario_rework_resume_test.go")
	if err := os.WriteFile(testPath, []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write scenario test: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "Rework resume",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "pkg/planner/plan.go",
							Pattern: "rework resume marker",
						},
					},
				},
			},
			{
				Name: "Edge case timeout",
				Assertions: []ContractAssertion{
					{
						FileContains: &FileContainsAssertion{
							Path:    "pkg/runner/timeout.go",
							Pattern: "edge timeout marker",
						},
					},
				},
			},
		},
	}

	// Invoke
	if err := AugmentWithTestAssertions(sc, workDir); err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// Assert
	rework := sc.Scenarios[0].Assertions
	if len(rework) != 1 {
		t.Fatalf("expected 1 assertion for Rework resume, got %d", len(rework))
	}
	if rework[0].GoTestPass == nil {
		t.Fatalf("expected go_test_pass for Rework resume, got %+v", rework[0])
	}
	if rework[0].FileContains != nil {
		t.Fatal("expected file_contains to be dropped for Rework resume")
	}
	if rework[0].GoTestPass.Pkg != "." {
		t.Fatalf("expected pkg '.', got %q", rework[0].GoTestPass.Pkg)
	}
	if rework[0].GoTestPass.TestName != "TestScenario_ReworkResume" {
		t.Fatalf("unexpected test name: %q", rework[0].GoTestPass.TestName)
	}

	timeout := sc.Scenarios[1].Assertions
	if len(timeout) != 1 {
		t.Fatalf("expected 1 assertion for Edge case timeout, got %d", len(timeout))
	}
	if timeout[0].FileContains == nil {
		t.Fatalf("expected file_contains to remain for Edge case timeout, got %+v", timeout[0])
	}
	if timeout[0].GoTestPass != nil {
		t.Fatalf("expected no go_test_pass for Edge case timeout, got %+v", timeout[0].GoTestPass)
	}
	if timeout[0].FileContains.Path != "pkg/runner/timeout.go" {
		t.Fatalf("unexpected file_contains path: %q", timeout[0].FileContains.Path)
	}
	if timeout[0].FileContains.Pattern != "edge timeout marker" {
		t.Fatalf("unexpected file_contains pattern: %q", timeout[0].FileContains.Pattern)
	}
}
