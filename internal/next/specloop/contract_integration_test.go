package specloop_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/specloop/stages"
	"github.com/danabrams/gromit/internal/next/validator"
)

// --- Test doubles ---

type fakeContractWriter struct {
	result *contract.ScenarioContract
	err    error
	calls  int
}

func (f *fakeContractWriter) WriteContracts(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
	f.calls++
	return f.result, f.err
}

type fakeContractEvaluator struct {
	failures []contract.ContractFailure
}

func (f *fakeContractEvaluator) Evaluate(_ context.Context, _ *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
	return f.failures, nil
}

type callbackEvaluator struct {
	fn func(c *contract.ScenarioContract, workDir string) []contract.ContractFailure
}

func (c *callbackEvaluator) Evaluate(_ context.Context, sc *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	return c.fn(sc, workDir), nil
}

type fakeFinalValidator struct {
	result validator.FinalResult
	err    error
}

func (f *fakeFinalValidator) RunFinal(_ context.Context, _ []validator.Check, _ []validator.Check, _ string) (validator.FinalResult, error) {
	return f.result, f.err
}

func passValidator() *fakeFinalValidator {
	return &fakeFinalValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{},
			ProjectChecks: validator.CheckResults{},
		},
	}
}

// contractScenarioStage is a flexible mock stage for contract integration tests.
type contractScenarioStage struct {
	name      string
	callCount int
	fn        func(ctx context.Context, rs *runstore.RunState, call int) (specloop.NextAction, error)
}

func (s *contractScenarioStage) Name() string { return s.name }
func (s *contractScenarioStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	s.callCount++
	return s.fn(ctx, rs, s.callCount)
}

// --- Environment setup ---

type contractTestEnv struct {
	tmp         string
	specPath    string
	evidenceDir string
	store       *runstore.Store
	rs          *runstore.RunState
}

func setupContractEnv(t *testing.T, specContent string) *contractTestEnv {
	t.Helper()
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-contract", "proj-contract")

	// Create run dir with spec-packet.md (required by WriteContractsStage).
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("compiled spec packet"), 0o644); err != nil {
		t.Fatalf("write spec-packet: %v", err)
	}

	// Write spec file.
	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	// Create evidence dir.
	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	return &contractTestEnv{
		tmp:         tmp,
		specPath:    specPath,
		evidenceDir: evidenceDir,
		store:       store,
		rs:          rs,
	}
}

func contractBudget(maxCycles int) *specloop.Budget {
	return specloop.NewBudget(execpolicy.Budgets{
		MaxSpecCycles:          maxCycles,
		MaxRunCostUSD:          99,
		MaxRunDurationSeconds:  3600,
		MaxTaskDurationSeconds: 300,
	})
}

func readyForReviewStage() *contractScenarioStage {
	return &contractScenarioStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (specloop.NextAction, error) {
			rs.Status = runstore.StatusReadyForReview
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}
}

const specWithContractScenarios = `# Contract Test Spec

## Scenarios

### Scenario: feature-works
**When:** feature is invoked
**Then:** output file exists
`

const specWithNoScenarios = `# Contract Test Spec

## Overview
No scenarios here.
`

// validContract is a well-formed ScenarioContract with one assertion.
func validContract() contract.ScenarioContract {
	return contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name:       "feature-works",
				Assertions: []contract.ContractAssertion{{FileExists: "output.txt"}},
			},
		},
	}
}

// --- Scenario 1: Happy path — contracts pass, pipeline continues without replan ---

// TestIntegration_ContractHappyPath verifies that when contracts are written and all
// assertions pass, the pipeline continues without triggering a replan.
func TestIntegration_ContractHappyPath(t *testing.T) {
	env := setupContractEnv(t, specWithContractScenarios)

	writer := &fakeContractWriter{result: func() *contract.ScenarioContract { c := validContract(); return &c }()}
	evaluator := &fakeContractEvaluator{failures: nil} // all assertions pass

	writeContractsStage := stages.NewWriteContractsStage(writer,
		stages.WriteContractsStageConfig{
			SpecPath:    env.specPath,
			EvidenceDir: env.evidenceDir,
			Store:       env.store,
		}, nil, nil)

	validateStage := stages.NewValidateStage(passValidator(), stages.ValidateStageConfig{WorkDir: env.tmp, EvidenceDir: env.evidenceDir}, nil, evaluator, nil)

	loop := specloop.NewSpecLoop(
		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
		specloop.SpecLoopConfig{
			Budget:      contractBudget(3),
			ReplanStage: "write_contracts",
		},
	)

	if err := loop.Run(context.Background(), env.rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if env.rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, env.rs.Status)
	}
	if env.rs.Cycle != 1 {
		t.Errorf("want cycle 1 (no replan), got %d", env.rs.Cycle)
	}
	if !env.rs.ContractsWritten {
		t.Error("ContractsWritten should be true after write_contracts stage succeeds")
	}
	if writer.calls != 1 {
		t.Errorf("expected 1 writer call, got %d", writer.calls)
	}
	if env.rs.TotalReplans != 0 {
		t.Errorf("expected 0 replans on happy path, got %d", env.rs.TotalReplans)
	}
}

// --- Scenario 2: Contract failure triggers replan, fix cycle passes ---

// TestIntegration_ContractFailureTriggersReplan_ReplanStageBypassesWriteContracts verifies that a contract assertion
// failure in ValidateStage triggers a replan, and the pipeline passes on the fix cycle.
func TestIntegration_ContractFailureTriggersReplan_ReplanStageBypassesWriteContracts(t *testing.T) {
	env := setupContractEnv(t, specWithContractScenarios)

	writer := &fakeContractWriter{result: func() *contract.ScenarioContract { c := validContract(); return &c }()}

	// First evaluation returns failures; second returns none.
	evalCalls := 0
	evaluator := &callbackEvaluator{
		fn: func(_ *contract.ScenarioContract, _ string) []contract.ContractFailure {
			evalCalls++
			if evalCalls == 1 {
				return []contract.ContractFailure{
					{
						ScenarioName:  "feature-works",
						AssertionType: "file_exists",
						Details:       `file "output.txt" does not exist`,
					},
				}
			}
			return nil
		},
	}

	writeContractsStage := stages.NewWriteContractsStage(writer,
		stages.WriteContractsStageConfig{
			SpecPath:    env.specPath,
			EvidenceDir: env.evidenceDir,
			Store:       env.store,
		}, nil, nil)

	validateStage := stages.NewValidateStage(passValidator(), stages.ValidateStageConfig{WorkDir: env.tmp, EvidenceDir: env.evidenceDir}, nil, evaluator, nil)

	// ReplanStage: "validate" causes replan to skip write_contracts entirely because the replan
	// stage sequence resumes at the validate stage, bypassing write_contracts. This demonstrates
	// stage-ordering behavior, NOT the ContractsWritten idempotency guard (which is tested in
	// TestIntegration_WriteContractsIdempotentOnReplanFromPlan with ReplanStage: "plan").
	loop := specloop.NewSpecLoop(
		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
		specloop.SpecLoopConfig{
			Budget:      contractBudget(3),
			ReplanStage: "validate",
		},
	)

	if err := loop.Run(context.Background(), env.rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if env.rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q after replan fix, got %q", runstore.StatusReadyForReview, env.rs.Status)
	}
	if env.rs.Cycle != 2 {
		t.Errorf("want cycle 2 (one replan), got %d", env.rs.Cycle)
	}
	if env.rs.TotalReplans != 1 {
		t.Errorf("expected 1 replan, got %d", env.rs.TotalReplans)
	}
	// Contract file must have been written in cycle 1 and reused in cycle 2
	contractPath := filepath.Join(env.evidenceDir, "scenario-contracts.yaml")
	if _, err := os.Stat(contractPath); err != nil {
		t.Errorf("scenario-contracts.yaml must exist after write_contracts: %v", err)
	}
	// Note: write_contracts is not re-run because ReplanStage=validate bypasses it in stage ordering,
	// not because ContractsWritten=true. See TestIntegration_WriteContractsIdempotentOnReplan for AC8 coverage.
	if writer.calls != 1 {
		t.Errorf("expected 1 writer call (stage sequence skips write_contracts on replan), got %d", writer.calls)
	}
}

// --- Scenario 3: WriteContracts idempotency — skipped when ContractsWritten=true ---

// TestIntegration_WriteContractsIdempotentOnReplan verifies that WriteContractsStage
// is skipped (ContractWriter not called) when ContractsWritten=true on a replan cycle.
func TestIntegration_WriteContractsIdempotentOnReplan(t *testing.T) {
	env := setupContractEnv(t, specWithContractScenarios)
	env.rs.ContractsWritten = true // simulate prior cycle already wrote contracts

	writer := &fakeContractWriter{} // must never be called

	writeContractsStage := stages.NewWriteContractsStage(writer,
		stages.WriteContractsStageConfig{
			SpecPath:    env.specPath,
			EvidenceDir: env.evidenceDir,
			Store:       env.store,
		}, nil, nil)

	validateCallCount := 0
	validateStage := &contractScenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (specloop.NextAction, error) {
			validateCallCount++
			if validateCallCount == 1 {
				// First call: trigger replan so write_contracts runs again on cycle 2.
				return specloop.NextAction{
					Kind: specloop.ReplanFrom,
					Context: &specloop.FailureContext{
						Failures: []string{"test failure to exercise replan"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			// Second call: pass.
			rs.FinalValidationPassed = true
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}

	loop := specloop.NewSpecLoop(
		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
		specloop.SpecLoopConfig{
			Budget:      contractBudget(3),
			ReplanStage: "write_contracts",
		},
	)

	if err := loop.Run(context.Background(), env.rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if env.rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, env.rs.Status)
	}
	if env.rs.Cycle != 2 {
		t.Errorf("want cycle 2 after replan, got %d", env.rs.Cycle)
	}
	// Writer must never be called: ContractsWritten=true causes idempotent skip both cycles.
	if writer.calls != 0 {
		t.Errorf("expected 0 writer calls (idempotent when ContractsWritten=true), got %d", writer.calls)
	}
	if env.rs.TotalReplans != 1 {
		t.Errorf("expected 1 replan, got %d", env.rs.TotalReplans)
	}
}

// TestIntegration_WriteContractsIdempotentOnReplanFromPlan verifies that when ContractsWritten=true
// and replan starts from "plan" (reaching write_contracts stage), the stage is skipped entirely
// due to the ContractsWritten guard, not due to bypassing the stage. This exercises AC8: the
// idempotency guard that prevents re-writing contracts when they already exist.
func TestIntegration_WriteContractsIdempotentOnReplanFromPlan(t *testing.T) {
	env := setupContractEnv(t, specWithContractScenarios)
	env.rs.ContractsWritten = true // simulate prior cycle already wrote contracts

	writer := &fakeContractWriter{} // must never be called

	writeContractsStage := stages.NewWriteContractsStage(writer,
		stages.WriteContractsStageConfig{
			SpecPath:    env.specPath,
			EvidenceDir: env.evidenceDir,
			Store:       env.store,
		}, nil, nil)

	validateCallCount := 0
	validateStage := &contractScenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (specloop.NextAction, error) {
			validateCallCount++
			if validateCallCount == 1 {
				// First call: trigger replan from plan stage.
				return specloop.NextAction{
					Kind: specloop.ReplanFrom,
					Context: &specloop.FailureContext{
						Failures: []string{"test failure to exercise replan from plan"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			// Second call: pass.
			rs.FinalValidationPassed = true
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}

	loop := specloop.NewSpecLoop(
		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
		specloop.SpecLoopConfig{
			Budget:      contractBudget(3),
			ReplanStage: "plan",
		},
	)

	if err := loop.Run(context.Background(), env.rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if env.rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, env.rs.Status)
	}
	if env.rs.Cycle != 2 {
		t.Errorf("want cycle 2 after replan, got %d", env.rs.Cycle)
	}
	// Writer must never be called: ContractsWritten=true causes idempotent skip on replan.
	// This is AC8 behavior — write_contracts is guarded by ContractsWritten, not bypassed by ReplanStage.
	if writer.calls != 0 {
		t.Errorf("expected 0 writer calls (idempotent when ContractsWritten=true), got %d", writer.calls)
	}
	if env.rs.TotalReplans != 1 {
		t.Errorf("expected 1 replan, got %d", env.rs.TotalReplans)
	}
}

// --- Scenario 4: No scenarios — WriteContracts is no-op, Validate skips contracts ---

// TestIntegration_NoScenariosWriteContractsNoOp verifies that when the spec has no
// scenarios, WriteContractsStage is a no-op (writer not called, no contract file created)
// and ValidateStage proceeds without contract evaluation.
func TestIntegration_NoScenariosWriteContractsNoOp(t *testing.T) {
	env := setupContractEnv(t, specWithNoScenarios)

	writer := &fakeContractWriter{} // must never be called
	evaluator := &fakeContractEvaluator{failures: nil}

	writeContractsStage := stages.NewWriteContractsStage(writer,
		stages.WriteContractsStageConfig{
			SpecPath:    env.specPath,
			EvidenceDir: env.evidenceDir,
			Store:       env.store,
		}, nil, nil)

	validateStage := stages.NewValidateStage(passValidator(), stages.ValidateStageConfig{WorkDir: env.tmp, EvidenceDir: env.evidenceDir}, nil, evaluator, nil)

	loop := specloop.NewSpecLoop(
		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
		specloop.SpecLoopConfig{
			Budget:      contractBudget(3),
			ReplanStage: "write_contracts",
		},
	)

	if err := loop.Run(context.Background(), env.rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if env.rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, env.rs.Status)
	}
	if env.rs.Cycle != 1 {
		t.Errorf("want cycle 1 (no replan), got %d", env.rs.Cycle)
	}
	if writer.calls != 0 {
		t.Errorf("expected 0 writer calls (no scenarios), got %d", writer.calls)
	}
	// No contract file should be written when spec has no scenarios.
	contractPath := filepath.Join(env.evidenceDir, "scenario-contracts.yaml")
	if _, err := os.Stat(contractPath); !os.IsNotExist(err) {
		t.Error("expected no scenario-contracts.yaml for no-scenarios spec")
	}
	// ContractsWritten remains false when there are no scenarios to write.
	if env.rs.ContractsWritten {
		t.Error("ContractsWritten should be false when spec has no scenarios")
	}
}

// --- Scenario 5: Missing contract file at Validate time — skipped silently ---

// TestIntegration_MissingContractFileSkippedSilently verifies that when EvidenceDir is
// configured but scenario-contracts.yaml does not exist at validate time, the stage
// proceeds silently without error and without triggering a replan.
func TestIntegration_MissingContractFileSkippedSilently(t *testing.T) {
	env := setupContractEnv(t, specWithContractScenarios)

	// Evaluator is configured with failures but must never be called — the missing
	// file causes the evaluation to be skipped entirely.
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{ScenarioName: "feature-works", AssertionType: "file_exists", Details: "should not appear"},
		},
	}

	// ValidateStage with EvidenceDir set, but no scenario-contracts.yaml written.
	validateStage := stages.NewValidateStage(passValidator(), stages.ValidateStageConfig{WorkDir: env.tmp, EvidenceDir: env.evidenceDir}, nil, evaluator, nil)

	loop := specloop.NewSpecLoop(
		[]specloop.Stage{validateStage, readyForReviewStage()},
		specloop.SpecLoopConfig{
			Budget:      contractBudget(3),
			ReplanStage: "validate",
		},
	)

	if err := loop.Run(context.Background(), env.rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Missing contract file must not trigger a replan — silently skipped.
	if env.rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q (missing contract file silently skipped), got %q",
			runstore.StatusReadyForReview, env.rs.Status)
	}
	if env.rs.Cycle != 1 {
		t.Errorf("want cycle 1 (no replan when contract file missing), got %d", env.rs.Cycle)
	}
	if env.rs.TotalReplans != 0 {
		t.Errorf("expected 0 replans when contract file absent, got %d", env.rs.TotalReplans)
	}
}
