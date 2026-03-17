package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

type fakeValidator struct {
	result validator.FinalResult
	err    error
}

func (f *fakeValidator) RunFinal(ctx context.Context, alwaysRun []validator.Check, projectChecks []validator.Check, workDir string) (validator.FinalResult, error) {
	return f.result, f.err
}

type fakeContractEvaluator struct {
	failures []contract.ContractFailure
}

func (f *fakeContractEvaluator) Evaluate(_ context.Context, _ *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
	return f.failures, nil
}

// Verify ValidateStage satisfies the Stage interface.
var _ specloop.Stage = (*ValidateStage)(nil)

func TestValidateStage_AllPass_Continue(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: true,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "test", Pass: true}},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil, nil)

	if stage.Name() != "validate" {
		t.Fatalf("expected name 'validate', got %q", stage.Name())
	}

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}
}

func TestValidateStage_Failure_ReplanFrom(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "test", Pass: false, Output: "FAIL"}},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 1
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be false")
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) == 0 {
		t.Fatal("expected failures to be non-empty")
	}
}

// TestValidateStage_MissingContractFile verifies that when EvidenceDir is set but
// scenario-contracts.yaml does not exist, the stage proceeds silently without error.
func TestValidateStage_MissingContractFile(t *testing.T) {
	dir := t.TempDir()

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{},
			ProjectChecks: validator.CheckResults{},
		},
	}
	evaluator := &fakeContractEvaluator{}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     "/tmp/work",
		EvidenceDir: dir, // dir exists but contains no scenario-contracts.yaml
	}, nil, evaluator)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error when contract file missing: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue when contract file missing, got %v", action.Kind)
	}
}

// TestValidateStage_ContractFailures verifies that contract assertion failures are
// collected and reported with the format "contract:<scenario-name> — <assertion-type> failed: <details>".
func TestValidateStage_ContractFailures(t *testing.T) {
	dir := t.TempDir()

	// Write a minimal scenario-contracts.yaml to the evidence dir.
	contractYAML := `scenarios:
  - name: subtract-works
    assertions:
      - file_exists: result.txt
`
	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{},
			ProjectChecks: validator.CheckResults{},
		},
	}
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{ScenarioName: "subtract-works", AssertionType: "file_exists", Details: `file "result.txt" does not exist`},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     "/tmp/work",
		EvidenceDir: dir,
	}, nil, evaluator)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom due to contract failure, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) == 0 {
		t.Fatal("expected failures to be non-empty")
	}
	want := `contract:subtract-works — file_exists failed: file "result.txt" does not exist`
	if action.Context.Failures[0] != want {
		t.Fatalf("expected failure %q, got %q", want, action.Context.Failures[0])
	}
}

// TestValidateStage_ContractAndShellFailures verifies that contract failures are collected
// first and then shell check failures are appended, both ending up in ReplanFrom failures.
func TestValidateStage_ContractAndShellFailures(t *testing.T) {
	dir := t.TempDir()

	contractYAML := `scenarios:
  - name: add-works
    assertions:
      - file_exists: out.txt
`
	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "test", Pass: false, Output: "FAIL"}},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{ScenarioName: "add-works", AssertionType: "file_exists", Details: `file "out.txt" does not exist`},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     "/tmp/work",
		EvidenceDir: dir,
	}, nil, evaluator)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	// contract failure should be first
	if len(action.Context.Failures) < 2 {
		t.Fatalf("expected at least 2 failures (contract + shell), got %d", len(action.Context.Failures))
	}
	if !strings.HasPrefix(action.Context.Failures[0], "contract:") {
		t.Fatalf("expected first failure to be contract failure, got %q", action.Context.Failures[0])
	}
}
