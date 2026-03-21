package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"gopkg.in/yaml.v3"
)

func TestScenario_StructuralRegressionExercisesCorrectBranch(t *testing.T) {
	// Seed: store, spec file, spec-packet, evidence dir
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte("# Spec\n\n## Scenarios\n\n### Scenario: add-works\n**When:** add is called\n**Then:** result is correct\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Valid contract with low-specificity pattern (triggers specificity retry)
	validContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "SomeExport", // single exported identifier → low specificity
						},
					},
				},
			},
		},
	}

	// Invalid contract: ContractAssertion{} with no assertion type set
	invalidRetryContract := &contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{}, // no assertion type set — structurally invalid
				},
			},
		},
	}

	// Precondition: verify the invalid contract triggers the right conditions
	if len(invalidRetryContract.Scenarios) == 0 {
		t.Fatal("precondition: retryResult.Scenarios must be non-empty")
	}
	valErrs := contract.ValidateContract(*invalidRetryContract)
	if len(valErrs) == 0 {
		t.Fatal("precondition: ValidateContract must return errors for zero-field assertion")
	}

	writerCalls := 0
	writer := &callbackContractWriter{
		fn: func(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
			writerCalls++
			if writerCalls == 1 {
				return &validContract, nil
			}
			// Specificity retry: return structurally invalid contract
			return invalidRetryContract, nil
		},
	}

	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)
	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: stage returns Continue (not Blocked)
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue (fallback to preRetryResult), got %v", action.Kind)
	}

	// Assert: writer called exactly twice (initial + specificity retry)
	if writerCalls != 2 {
		t.Fatalf("expected 2 writer calls, got %d", writerCalls)
	}

	// Assert: contract file contains the preRetryResult, not the invalid retry
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("contract file not written: %v", err)
	}

	var finalContract contract.ScenarioContract
	if err := yaml.Unmarshal(data, &finalContract); err != nil {
		t.Fatalf("unmarshal contract: %v", err)
	}

	if len(finalContract.Scenarios) == 0 || len(finalContract.Scenarios[0].Assertions) == 0 {
		t.Fatal("final contract missing scenarios/assertions — preRetryResult not preserved")
	}
	if finalContract.Scenarios[0].Assertions[0].FileContains == nil {
		t.Fatal("expected FileContains assertion from preRetryResult, got nil")
	}
	if finalContract.Scenarios[0].Assertions[0].FileContains.Pattern != "SomeExport" {
		t.Fatalf("expected pattern 'SomeExport' from preRetryResult, got %q",
			finalContract.Scenarios[0].Assertions[0].FileContains.Pattern)
	}

	// Assert: ContractsWritten flag is set
	if !rs.ContractsWritten {
		t.Fatal("expected ContractsWritten=true after fallback to preRetryResult")
	}

	// Assert: contract_specificity_warning event emitted (regression kept low-specificity contract)
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var warningFound bool
	for _, ev := range events {
		if ev.EventType() == "contract_specificity_warning" {
			warningFound = true
		}
	}
	if !warningFound {
		t.Fatal("expected contract_specificity_warning event when structural regression triggers fallback")
	}
}
