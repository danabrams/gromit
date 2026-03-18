package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// callbackStage is a flexible mock stage for write_contracts integration tests.
type callbackStage struct {
	name      string
	callCount int
	fn        func(ctx context.Context, rs *runstore.RunState, call int) (specloop.NextAction, error)
}

func (s *callbackStage) Name() string { return s.name }
func (s *callbackStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	s.callCount++
	return s.fn(ctx, rs, s.callCount)
}

func integrationBudget(maxCycles int) *specloop.Budget {
	return specloop.NewBudget(execpolicy.Budgets{
		MaxSpecCycles:          maxCycles,
		MaxRunCostUSD:          99,
		MaxRunDurationSeconds:  3600,
		MaxTaskDurationSeconds: 300,
	})
}

func passingValidator() *fakeValidator {
	return &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{},
			ProjectChecks: validator.CheckResults{},
		},
	}
}

func finalizeStage() *callbackStage {
	return &callbackStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (specloop.NextAction, error) {
			rs.Status = runstore.StatusReadyForReview
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}
}

// setupIntegrationEnv creates a temp dir with run dir, spec file, and evidence dir.
func setupIntegrationEnv(t *testing.T, specContent string) (tmp, specPath, evidenceDir string, store *runstore.Store, rs *runstore.RunState) {
	t.Helper()
	tmp = t.TempDir()
	store = runstore.NewStore(tmp)
	rs = runstore.NewRunState("spec-wc-integ", "proj-wc-integ")

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

// fileContainsContract returns a ScenarioContract asserting "output.txt" contains "expected content".
func fileContainsContract() contract.ScenarioContract {
	return contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "output.txt",
							Pattern: "expected content",
						},
					},
				},
			},
		},
	}
}

// TestIntegration_WriteContracts_FullPipelineWithReplan exercises the full flow:
// (1) WriteContracts produces a file_contains contract,
// (2) Execute creates the file WITHOUT the required content,
// (3) Validate detects failure and triggers ReplanFrom,
// (4) On replan cycle, WriteContracts is a no-op (ContractsWritten=true),
// (5) Execute fix cycle writes correct content and Validate passes.
func TestIntegration_WriteContracts_FullPipelineWithReplan(t *testing.T) {
	tmp, specPath, evidenceDir, store, rs := setupIntegrationEnv(t, specWithScenarios)

	c := fileContainsContract()
	writer := &fakeContractWriter{result: &c}

	// Execute stage: cycle 1 writes wrong content; cycle 2 writes correct content.
	outputPath := filepath.Join(tmp, "output.txt")
	executeStage := &callbackStage{
		name: "execute",
		fn: func(_ context.Context, _ *runstore.RunState, call int) (specloop.NextAction, error) {
			content := "wrong content"
			if call > 1 {
				content = "expected content"
			}
			if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
				return specloop.NextAction{}, err
			}
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}

	// Use the real evaluator so filesystem state drives the contract result.
	evaluator := &contract.DefaultContractEvaluator{}

	writeContractsStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, nil, nil)

	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
		WorkDir:     tmp,
		EvidenceDir: evidenceDir,
	}, nil, evaluator, nil)

	loop := specloop.NewSpecLoop(
		[]specloop.Stage{writeContractsStage, executeStage, validateStage, finalizeStage()},
		specloop.SpecLoopConfig{
			Budget:      integrationBudget(3),
			ReplanStage: "write_contracts",
		},
	)

	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, rs.Status)
	}
	if rs.Cycle != 2 {
		t.Errorf("want cycle 2 (one replan), got %d", rs.Cycle)
	}
	if rs.TotalReplans != 1 {
		t.Errorf("expected 1 replan, got %d", rs.TotalReplans)
	}
	// Writer called exactly once — ContractsWritten=true causes idempotent skip on replan cycle.
	if writer.calls != 1 {
		t.Errorf("expected 1 writer call (idempotent on replan), got %d", writer.calls)
	}
	if !rs.ContractsWritten {
		t.Error("ContractsWritten should be true after write_contracts stage")
	}
	// Execute was called twice: once broken, once fixed.
	if executeStage.callCount != 2 {
		t.Errorf("expected execute called 2 times, got %d", executeStage.callCount)
	}
	// Contract file must persist in the evidence dir.
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if _, err := os.Stat(contractPath); err != nil {
		t.Errorf("scenario-contracts.yaml must exist after write_contracts: %v", err)
	}
}

// TestIntegration_WriteContracts_NoScenariosNoContractFile verifies that a spec
// with no scenarios causes WriteContractsStage to be a no-op: the ContractWriter
// is never called and no contract file is created. ValidateStage proceeds without
// contract evaluation and the pipeline completes without a replan.
func TestIntegration_WriteContracts_NoScenariosNoContractFile(t *testing.T) {
	tmp, specPath, evidenceDir, store, rs := setupIntegrationEnv(t, specWithoutScenarios)

	writer := &fakeContractWriter{} // must never be called
	evaluator := &fakeContractEvaluator{failures: nil}

	writeContractsStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, nil, nil)

	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
		WorkDir:     tmp,
		EvidenceDir: evidenceDir,
	}, nil, evaluator, nil)

	loop := specloop.NewSpecLoop(
		[]specloop.Stage{writeContractsStage, validateStage, finalizeStage()},
		specloop.SpecLoopConfig{
			Budget:      integrationBudget(3),
			ReplanStage: "write_contracts",
		},
	)

	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, rs.Status)
	}
	if rs.Cycle != 1 {
		t.Errorf("want cycle 1 (no replan), got %d", rs.Cycle)
	}
	if writer.calls != 0 {
		t.Errorf("expected 0 writer calls for no-scenarios spec, got %d", writer.calls)
	}
	if rs.ContractsWritten {
		t.Error("ContractsWritten should be false when spec has no scenarios")
	}
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if _, err := os.Stat(contractPath); !os.IsNotExist(err) {
		t.Error("expected no scenario-contracts.yaml when spec has no scenarios")
	}
}

// TestIntegration_WriteContracts_MissingContractFileGraceful verifies that when
// EvidenceDir is configured but scenario-contracts.yaml is absent at validate time,
// ValidateStage proceeds silently without error and without triggering a replan.
func TestIntegration_WriteContracts_MissingContractFileGraceful(t *testing.T) {
	tmp := t.TempDir()
	rs := runstore.NewRunState("spec-missing-contract", "proj-missing")

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Evaluator is configured with failures but must never be called — the missing
	// contract file causes evaluation to be skipped entirely.
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{ScenarioName: "any", AssertionType: "file_exists", Details: "should not appear"},
		},
	}

	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
		WorkDir:     tmp,
		EvidenceDir: evidenceDir, // dir exists but scenario-contracts.yaml is absent
	}, nil, evaluator, nil)

	loop := specloop.NewSpecLoop(
		[]specloop.Stage{validateStage, finalizeStage()},
		specloop.SpecLoopConfig{
			Budget:      integrationBudget(3),
			ReplanStage: "validate",
		},
	)

	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q (missing contract silently skipped), got %q",
			runstore.StatusReadyForReview, rs.Status)
	}
	if rs.Cycle != 1 {
		t.Errorf("want cycle 1 (no replan when contract file missing), got %d", rs.Cycle)
	}
	if rs.TotalReplans != 0 {
		t.Errorf("expected 0 replans, got %d", rs.TotalReplans)
	}
}
