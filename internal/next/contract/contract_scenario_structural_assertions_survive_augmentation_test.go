package contract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScenario_StructuralAssertionsSurviveAugmentation(t *testing.T) {
	// Seed
	workDir := t.TempDir()

	testDir := filepath.Join(workDir, "internal", "next", "planner")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir test dir: %v", err)
	}

	testSrc := `package planner

import "testing"

func TestScenario_ReworkVisionChange(t *testing.T) {}
`
	testPath := filepath.Join(testDir, "planner_scenario_rework_vision_change_test.go")
	if err := os.WriteFile(testPath, []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write scenario test: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "Rework vision change",
				Assertions: []ContractAssertion{
					{FileExists: "internal/next/planner/types.go"},
					{FileContains: &FileContainsAssertion{
						Path:    "plan.go",
						Pattern: "rework_vision_change",
					}},
				},
			},
		},
	}

	// Invoke
	if err := AugmentWithTestAssertions(sc, workDir); err != nil {
		t.Fatalf("AugmentWithTestAssertions: %v", err)
	}

	// Assert
	assertions := sc.Scenarios[0].Assertions
	if len(assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(assertions))
	}

	if assertions[0].FileExists != "internal/next/planner/types.go" {
		t.Fatalf("expected file_exists to be retained, got %q", assertions[0].FileExists)
	}

	if assertions[1].GoTestPass == nil {
		t.Fatalf("expected go_test_pass assertion at index 1")
	}
	if assertions[1].FileContains != nil {
		t.Fatalf("expected file_contains to be removed when go_test_pass exists")
	}
	if assertions[1].GoTestPass.Pkg != "./internal/next/planner/..." {
		t.Fatalf("unexpected go_test_pass pkg: %q", assertions[1].GoTestPass.Pkg)
	}
	if assertions[1].GoTestPass.TestName != "TestScenario_ReworkVisionChange" {
		t.Fatalf("unexpected go_test_pass test name: %q", assertions[1].GoTestPass.TestName)
	}
}
