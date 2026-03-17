package stages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// setupScenarioEnv creates a complete test environment with spec file, evidence dir,
// run dir with spec-packet.md, and a fresh RunState.
func setupScenarioEnv(t *testing.T, specContent string) (tmp, specPath, evidenceDir string, store *runstore.Store, rs *runstore.RunState) {
	t.Helper()
	tmp = t.TempDir()
	store = runstore.NewStore(tmp)
	rs = runstore.NewRunState("spec-scenario", "proj-scenario")

	// Create run dir with spec-packet.md (required by WriteContractsStage).
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("compiled spec packet"), 0o644); err != nil {
		t.Fatalf("write spec-packet: %v", err)
	}

	specPath = filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	evidenceDir = filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}
	return
}

const scenarioSpecWith2Scenarios = `# Calculator Spec

## Scenarios

### Scenario: add-function-works
**When:** the Add function is called with 1 and 2
**Then:** the result is 3

### Scenario: subtract-function-works
**When:** the Subtract function is called with 5 and 3
**Then:** the result is 2
`

const scenarioSpecWithNoScenarios = `# Calculator Spec

## Overview
This spec describes a calculator.

## Goals
- Implement basic math operations
`

// --- Scenario 1: Happy path -- contracts pass ---

func TestScenario_HappyPath_ContractsPass(t *testing.T) {
	// Seed: RunState with spec packet, 2 scenarios in spec markdown.
	tmp, specPath, evidenceDir, store, rs := setupScenarioEnv(t, scenarioSpecWith2Scenarios)

	sc := &contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-function-works",
				Assertions: []contract.ContractAssertion{
					{FileExists: "calc/calc.go"},
					{FileContains: &contract.FileContainsAssertion{Path: "calc/calc.go", Pattern: "func Add"}},
				},
			},
			{
				Name: "subtract-function-works",
				Assertions: []contract.ContractAssertion{
					{FileContains: &contract.FileContainsAssertion{Path: "calc/calc.go", Pattern: "func Subtract"}},
				},
			},
		},
	}
	writer := &fakeContractWriter{result: sc}
	evaluator := &fakeContractEvaluator{failures: nil} // all assertions pass

	// Invoke: WriteContracts stage -> Validate stage (with mock evaluator returning no failures).
	writeStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, nil, nil)

	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
		WorkDir:     tmp,
		EvidenceDir: evidenceDir,
	}, nil, evaluator)

	// Run WriteContracts.
	action, err := writeStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("WriteContracts error: %v", err)
	}

	// Assert: scenario-contracts.yaml written.
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if _, err := os.Stat(contractPath); err != nil {
		t.Fatalf("scenario-contracts.yaml not written: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected WriteContracts to Continue, got %v", action.Kind)
	}
	if !rs.ContractsWritten {
		t.Fatal("expected ContractsWritten=true after WriteContracts")
	}

	// Run Validate.
	action, err = validateStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}

	// Assert: validation passes, pipeline continues.
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Validate to Continue, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed=true")
	}
}

// --- Scenario 2: Contract assertion fails, triggers replan ---

func TestScenario_ContractFailure_TriggersReplan(t *testing.T) {
	// Seed: RunState with tasks, contract file with assertions.
	tmp, specPath, evidenceDir, store, rs := setupScenarioEnv(t, scenarioSpecWith2Scenarios)

	sc := &contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "subtract-works",
				Assertions: []contract.ContractAssertion{
					{FileContains: &contract.FileContainsAssertion{Path: "calc/calc.go", Pattern: "func Subtract"}},
				},
			},
		},
	}
	writer := &fakeContractWriter{result: sc}

	// Write contracts first.
	writeStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, nil, nil)
	if _, err := writeStage.Run(context.Background(), rs); err != nil {
		t.Fatalf("WriteContracts: %v", err)
	}

	// Invoke: Validate stage with evaluator returning a failure.
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "subtract-works",
				AssertionType: "file_contains",
				Details:       `pattern 'func Subtract' not found in calc/calc.go`,
			},
		},
	}
	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
		WorkDir:     tmp,
		EvidenceDir: evidenceDir,
	}, nil, evaluator)

	action, err := validateStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}

	// Assert: ReplanFrom returned.
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil || len(action.Context.Failures) == 0 {
		t.Fatal("expected non-empty failure context")
	}

	// Assert: failure context matches format "contract:<name> -- <type> failed: <details>".
	want := `contract:subtract-works — file_contains failed: pattern 'func Subtract' not found in calc/calc.go`
	if action.Context.Failures[0] != want {
		t.Fatalf("expected failure %q, got %q", want, action.Context.Failures[0])
	}
}

// --- Scenario 3: WriteContracts produces invalid YAML ---

func TestScenario_InvalidYAML_RetriesAndSucceeds(t *testing.T) {
	// Seed: RunState with scenarios.
	_, specPath, evidenceDir, store, rs := setupScenarioEnv(t, scenarioSpecWith2Scenarios)

	validContract := &contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-function-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
		},
	}

	// Invoke: WriteContracts with mock writer that returns unparseable output first 2 times,
	// then valid on 3rd attempt.
	callCount := 0
	writer := &callbackContractWriter{
		fn: func(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
			callCount++
			if callCount <= 2 {
				return nil, fmt.Errorf("yaml: unmarshal error: cannot decode")
			}
			return validContract, nil
		},
	}

	writeStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, nil, nil)

	action, err := writeStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("WriteContracts error: %v", err)
	}

	// Assert: retries 2 times, succeeds on 3rd attempt.
	if callCount != 3 {
		t.Fatalf("expected 3 writer calls (1 initial + 2 retries), got %d", callCount)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue after successful 3rd attempt, got %v", action.Kind)
	}
	if !rs.ContractsWritten {
		t.Fatal("expected ContractsWritten=true after successful retry")
	}

	// Contract file must exist.
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if _, err := os.Stat(contractPath); err != nil {
		t.Fatalf("scenario-contracts.yaml not written after retry: %v", err)
	}
}

func TestScenario_InvalidYAML_BlockedAfterAllRetries(t *testing.T) {
	// Seed: RunState with scenarios.
	_, specPath, evidenceDir, store, rs := setupScenarioEnv(t, scenarioSpecWith2Scenarios)

	// Invoke: Writer always returns errors (all 3 attempts fail).
	callCount := 0
	writer := &callbackContractWriter{
		fn: func(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
			callCount++
			return nil, fmt.Errorf("yaml: unmarshal error: attempt %d", callCount)
		},
	}

	writeStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, nil, nil)

	action, err := writeStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("WriteContracts error: %v", err)
	}

	// Assert: blocked after all 3 attempts.
	if callCount != 3 {
		t.Fatalf("expected 3 writer calls, got %d", callCount)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked after all retries, got %v", action.Kind)
	}
}

// --- Scenario 4: Spec has no scenarios section ---

func TestScenario_NoScenariosSection(t *testing.T) {
	// Seed: RunState with spec that has no "### Scenario:" headers.
	_, specPath, evidenceDir, store, rs := setupScenarioEnv(t, scenarioSpecWithNoScenarios)

	writer := &fakeContractWriter{} // must never be called

	// Invoke: WriteContracts.
	writeStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, nil, nil)

	action, err := writeStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("WriteContracts error: %v", err)
	}

	// Assert: returns Continue, no contract file written.
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue for no-scenarios, got %v", action.Kind)
	}
	if writer.calls != 0 {
		t.Fatalf("expected 0 writer calls for no-scenarios, got %d", writer.calls)
	}
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if _, err := os.Stat(contractPath); !os.IsNotExist(err) {
		t.Fatal("expected no scenario-contracts.yaml for spec with no scenarios")
	}
	if rs.ContractsWritten {
		t.Fatal("expected ContractsWritten=false for no-scenarios spec")
	}
}

// --- Scenario 5: Contract file missing at Validate time ---

func TestScenario_ContractFileMissingAtValidateTime(t *testing.T) {
	// Seed: RunState, no scenario-contracts.yaml in evidence dir.
	tmp, _, evidenceDir, _, rs := setupScenarioEnv(t, scenarioSpecWith2Scenarios)

	// Evaluator is configured with failures but must never be reached --
	// the missing file causes contract evaluation to be skipped entirely.
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{ScenarioName: "should-not-appear", AssertionType: "file_exists", Details: "unreachable"},
		},
	}

	// Invoke: Validate stage with no contract file in evidence dir.
	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
		WorkDir:     tmp,
		EvidenceDir: evidenceDir,
	}, nil, evaluator)

	action, err := validateStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}

	// Assert: contract checking skipped silently, shell checks still run.
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue when contract file missing, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed=true (shell checks pass, contract skipped)")
	}
}

// --- Scenario 6: Replan preserves contracts ---

func TestScenario_ReplanPreservesContracts(t *testing.T) {
	// Seed: RunState with ContractsWritten=true, existing contract file.
	_, specPath, evidenceDir, store, rs := setupScenarioEnv(t, scenarioSpecWith2Scenarios)
	rs.ContractsWritten = true

	// Write an existing contract file.
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	originalContent := "scenarios:\n  - name: original\n    assertions:\n      - file_exists: original.go\n"
	if err := os.WriteFile(contractPath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	writer := &fakeContractWriter{} // must never be called

	// Invoke: WriteContracts stage.
	writeStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, nil, nil)

	action, err := writeStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("WriteContracts error: %v", err)
	}

	// Assert: returns Continue immediately (no-op), contract file unchanged.
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue for idempotent replan, got %v", action.Kind)
	}
	if writer.calls != 0 {
		t.Fatalf("expected 0 writer calls when ContractsWritten=true, got %d", writer.calls)
	}

	// Contract file must be unchanged.
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract file: %v", err)
	}
	if string(data) != originalContent {
		t.Fatalf("contract file was modified during replan:\nwant: %s\ngot:  %s", originalContent, string(data))
	}
}

// --- Scenario 7: Multiple assertions with partial failure ---

func TestScenario_MultipleAssertionsPartialFailure(t *testing.T) {
	// Seed: Contract with 2 assertions for one scenario (file_exists passes,
	// file_contains fails).
	tmp, specPath, evidenceDir, store, rs := setupScenarioEnv(t, scenarioSpecWith2Scenarios)

	sc := &contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "calculator-module-exists",
				Assertions: []contract.ContractAssertion{
					{FileExists: "calc/calc.go"},
					{FileContains: &contract.FileContainsAssertion{Path: "calc/calc.go", Pattern: "func Multiply"}},
				},
			},
		},
	}
	writer := &fakeContractWriter{result: sc}

	writeStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, nil, nil)
	if _, err := writeStage.Run(context.Background(), rs); err != nil {
		t.Fatalf("WriteContracts: %v", err)
	}

	// Invoke: Validate with evaluator returning partial failure.
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "calculator-module-exists",
				AssertionType: "file_contains",
				Details:       `pattern "func Multiply" not found in calc/calc.go`,
			},
		},
	}
	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
		WorkDir:     tmp,
		EvidenceDir: evidenceDir,
	}, nil, evaluator)

	action, err := validateStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}

	// Assert: both checked (no short-circuit), only the failure returned.
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom for partial failure, got %v", action.Kind)
	}
	if len(action.Context.Failures) != 1 {
		t.Fatalf("expected exactly 1 failure, got %d: %v", len(action.Context.Failures), action.Context.Failures)
	}
	if !strings.Contains(action.Context.Failures[0], "contract:calculator-module-exists") {
		t.Fatalf("expected failure to reference calculator-module-exists, got %q", action.Context.Failures[0])
	}
	if !strings.Contains(action.Context.Failures[0], "file_contains failed") {
		t.Fatalf("expected file_contains failure, got %q", action.Context.Failures[0])
	}
}

// --- Scenario 8: Budget exhaustion ---

func TestScenario_BudgetExhaustion(t *testing.T) {
	// Seed: RunState with scenarios, budget that reports exhausted.
	_, specPath, evidenceDir, store, rs := setupScenarioEnv(t, scenarioSpecWith2Scenarios)

	budget := specloop.NewBudget(execpolicy.Budgets{
		MaxSpecCycles:          1,
		MaxRunCostUSD:          99,
		MaxRunDurationSeconds:  3600,
		MaxTaskDurationSeconds: 300,
	})
	budget.IncrementCycle() // exhaust the budget

	writer := &fakeContractWriter{} // must never be called

	// Invoke: WriteContracts with budget that returns Exceeded=true.
	writeStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, budget, nil)

	action, err := writeStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("WriteContracts error: %v", err)
	}

	// Assert: returns Blocked with budget-exhausted message.
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked when budget exhausted, got %v", action.Kind)
	}
	if writer.calls != 0 {
		t.Fatalf("expected 0 writer calls when budget exhausted, got %d", writer.calls)
	}
	if action.Context == nil || len(action.Context.Failures) == 0 {
		t.Fatal("expected failure context with budget-exhausted message")
	}
	if !strings.Contains(action.Context.Failures[0], "budget exhausted") {
		t.Fatalf("expected budget-exhausted message, got %q", action.Context.Failures[0])
	}
}
