package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

func TestScenario_AugmentationAutomaticallyPicksUpNewScenarioTestsAcrossCycles(t *testing.T) {
	// Seed: Create temporary directories for evidence and work
	evidenceDir := t.TempDir()
	workDir := newValidateWorkDir(t)

	// Contract with file_contains assertion (no matching scenario test yet)
	contractYAML := `scenarios:
  - name: Add feature and validate
    assertions:
      - file_exists: pkg/feature/main.go
      - file_contains:
          path: pkg/feature/main.go
          pattern: add_feature_marker
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	validator := &fakeValidator{result: validator.FinalResult{Pass: true}}
	evaluator := &recordingContractEvaluator{}
	stage := NewValidateStage(validator, ValidateStageConfig{
		WorkDir:     workDir,
		EvidenceDir: evidenceDir,
	}, nil, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")

	// First cycle: No scenario test file exists yet
	action1, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("first validate run: %v", err)
	}
	if action1.Kind != specloop.Continue {
		t.Fatalf("expected Continue on first cycle, got %v", action1.Kind)
	}
	if len(evaluator.Received) != 1 {
		t.Fatalf("expected evaluator to receive 1 contract, got %d", len(evaluator.Received))
	}

	// First cycle assertions: file_contains should be preserved (no scenario test exists)
	firstCycleAssertions := evaluator.Received[0].Scenarios[0].Assertions
	if len(firstCycleAssertions) != 2 {
		t.Fatalf("first cycle: expected 2 assertions (file_exists + file_contains), got %d", len(firstCycleAssertions))
	}
	if firstCycleAssertions[0].FileExists != "pkg/feature/main.go" {
		t.Fatalf("first cycle: expected file_exists preserved")
	}
	if firstCycleAssertions[1].FileContains == nil {
		t.Fatalf("first cycle: expected file_contains to be preserved when no scenario test exists")
	}
	if firstCycleAssertions[1].GoTestPass != nil {
		t.Fatalf("first cycle: expected no go_test_pass when scenario test doesn't exist")
	}

	// Create scenario test file between cycles
	testFileDir := filepath.Join(workDir, "pkg", "feature")
	if err := os.MkdirAll(testFileDir, 0o755); err != nil {
		t.Fatalf("mkdir test dir: %v", err)
	}
	testFile := filepath.Join(testFileDir, "feature_scenario_add_feature_and_validate_test.go")
	testSrc := `package feature

import "testing"

func TestScenario_AddFeatureAndValidate(t *testing.T) {}
`
	if err := os.WriteFile(testFile, []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write scenario test file: %v", err)
	}

	// Reset evaluator for second cycle
	evaluator.Received = nil

	// Second cycle: Scenario test file now exists
	rs.Cycle++
	action2, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("second validate run: %v", err)
	}
	if action2.Kind != specloop.Continue {
		t.Fatalf("expected Continue on second cycle, got %v", action2.Kind)
	}
	if len(evaluator.Received) != 1 {
		t.Fatalf("expected evaluator to receive 1 contract on second cycle, got %d", len(evaluator.Received))
	}

	// Second cycle assertions: file_contains should be replaced with go_test_pass
	secondCycleAssertions := evaluator.Received[0].Scenarios[0].Assertions
	if len(secondCycleAssertions) != 2 {
		t.Fatalf("second cycle: expected 2 assertions (file_exists + go_test_pass), got %d", len(secondCycleAssertions))
	}
	if secondCycleAssertions[0].FileExists != "pkg/feature/main.go" {
		t.Fatalf("second cycle: expected file_exists preserved")
	}
	if secondCycleAssertions[1].GoTestPass == nil {
		t.Fatalf("second cycle: expected go_test_pass to replace file_contains after scenario test is created")
	}
	if secondCycleAssertions[1].FileContains != nil {
		t.Fatalf("second cycle: file_contains should be removed when go_test_pass exists")
	}

	// Verify go_test_pass details
	if secondCycleAssertions[1].GoTestPass.Pkg != "./pkg/feature" {
		t.Fatalf("second cycle: unexpected pkg pattern: %q", secondCycleAssertions[1].GoTestPass.Pkg)
	}
	if secondCycleAssertions[1].GoTestPass.TestName != "TestScenario_AddFeatureAndValidate" {
		t.Fatalf("second cycle: unexpected test name: %q", secondCycleAssertions[1].GoTestPass.TestName)
	}
}
