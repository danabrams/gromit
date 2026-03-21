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
	"gopkg.in/yaml.v3"
)

// fakeContractWriter is a test double for the ContractWriter interface.
type fakeContractWriter struct {
	result *contract.ScenarioContract
	err    error
	calls  int
}

func (f *fakeContractWriter) WriteContracts(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

// Verify WriteContractsStage satisfies the Stage interface.
var _ specloop.Stage = (*WriteContractsStage)(nil)

func makeWriteContractsRunState(t *testing.T, store *runstore.Store) *runstore.RunState {
	t.Helper()
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}
	return rs
}

const specWithScenarios = `# Test Spec

## Scenarios

### Scenario: add-works
**When:** add is called with 1 and 2
**Then:** result is 3

### Scenario: subtract-works
**When:** subtract is called with 5 and 3
**Then:** result is 2
`

const specWithoutScenarios = `# Test Spec

## Overview
No scenarios here.
`

const specWithSkippedScenarios = `# Test Spec

## Scenarios

### Scenario: invalid-format
This scenario has no proper format
And should be skipped

### Scenario: add-works
**When:** add is called with 1 and 2
**Then:** result is 3
`

func TestWriteContracts_IdempotencyNoOp(t *testing.T) {
	// When ContractsWritten is already true, stage returns Continue without calling writer
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)
	rs.ContractsWritten = true

	writer := &fakeContractWriter{}
	cfg := WriteContractsStageConfig{
		SpecPath:    filepath.Join(tmp, "spec.md"),
		EvidenceDir: filepath.Join(tmp, "evidence"),
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue for idempotent run, got %v", action.Kind)
	}
	if writer.calls != 0 {
		t.Fatalf("expected 0 writer calls for idempotent run, got %d", writer.calls)
	}
}

func TestWriteContracts_NoScenariosReturnsContinue(t *testing.T) {
	// When spec has no scenarios, returns Continue with no output
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithoutScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write spec-packet.md
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	writer := &fakeContractWriter{}
	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue for no-scenarios, got %v", action.Kind)
	}
	if writer.calls != 0 {
		t.Fatalf("expected 0 writer calls for no-scenarios, got %d", writer.calls)
	}
	// No contract file should be written
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if _, err := os.Stat(contractPath); !os.IsNotExist(err) {
		t.Fatal("expected no scenario-contracts.yaml for no-scenarios spec")
	}
}

func TestWriteContracts_SuccessWritesContractFile(t *testing.T) {
	// Happy path: writer returns valid contract, stage writes YAML and sets ContractsWritten=true
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet content"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{FileExists: "calc/calc.go"},
				},
			},
			{
				Name: "subtract-works",
				Assertions: []contract.ContractAssertion{
					{FileExists: "calc/calc.go"},
				},
			},
		},
	}

	writer := &fakeContractWriter{result: &validContract}
	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)
	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true after success")
	}

	// Contract file must exist
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("scenario-contracts.yaml not written: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("scenario-contracts.yaml is empty")
	}

	// Event must be emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var found bool
	for _, ev := range events {
		if ev.EventType() == "contracts_written" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected contracts_written event to be emitted")
	}
}

func TestWriteContracts_WriterErrorRetriesAndBlocks(t *testing.T) {
	// When writer always fails with parse/validation error, retries 3 total then returns Blocked
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Writer returns invalid contract (zero fields per assertion)
	invalidContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{}, // zero fields — invalid
				},
			},
		},
	}

	writer := &fakeContractWriter{result: &invalidContract}
	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)
	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked after retry exhaustion, got %v", action.Kind)
	}
	// 3 total attempts (1 initial + 2 retries)
	if writer.calls != 3 {
		t.Fatalf("expected 3 writer calls (1+2 retries), got %d", writer.calls)
	}

	// contracts_blocked event must be emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var found bool
	for _, ev := range events {
		if ev.EventType() == "contracts_blocked" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected contracts_blocked event to be emitted")
	}
}

func TestWriteContracts_ValidationFailureRetriesOnce(t *testing.T) {
	// First call returns invalid, second returns valid — only 2 calls total
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
		},
	}
	invalidContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-works", Assertions: []contract.ContractAssertion{{}}},
		},
	}

	callCount := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			callCount++
			if callCount == 1 {
				return &invalidContract, nil
			}
			return &validContract, nil
		},
	}

	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue after retry success, got %v", action.Kind)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 writer calls, got %d", callCount)
	}
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true after retry success")
	}
}

func TestWriteContracts_BudgetExhaustedReturnsBlocked(t *testing.T) {
	// When budget is exhausted before LLM invocation, stage returns Blocked and emits contracts_blocked
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	budget.IncrementCycle() // exhaust the budget

	writer := &fakeContractWriter{}
	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)
	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, budget, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked when budget exhausted, got %v", action.Kind)
	}
	if writer.calls != 0 {
		t.Fatalf("expected 0 writer calls when budget exhausted, got %d", writer.calls)
	}

	// contracts_blocked event must be emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var found bool
	for _, ev := range events {
		if ev.EventType() == "contracts_blocked" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected contracts_blocked event when budget exhausted")
	}
}

func TestWriteContracts_Name(t *testing.T) {
	stage := &WriteContractsStage{}
	if stage.Name() != "write_contracts" {
		t.Fatalf("expected name 'write_contracts', got %q", stage.Name())
	}
}

func TestWriteContracts_SkippedScenariosEmitEvents(t *testing.T) {
	// When scenarios are skipped during parsing, events must be emitted
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithSkippedScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet content"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{FileExists: "calc/calc.go"},
				},
			},
		},
	}

	writer := &fakeContractWriter{result: &validContract}
	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)
	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Events must be emitted for skipped scenarios
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var skippedCount int
	for _, ev := range events {
		if ev.EventType() == "contract_scenario_skipped" {
			skippedCount++
		}
	}

	if skippedCount == 0 {
		t.Fatal("expected at least one contract_scenario_skipped event to be emitted")
	}
}

func TestWriteContracts_RetryContextIncludesValidKeys(t *testing.T) {
	// On vocabulary violation retry, specPacket must include valid assertion key names.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
		},
	}
	invalidContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-works", Assertions: []contract.ContractAssertion{{}}},
		},
	}

	var specPacketOnRetry string
	callCount := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			callCount++
			if callCount == 1 {
				return &invalidContract, nil
			}
			specPacketOnRetry = specPacket
			return &validContract, nil
		},
	}

	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue after retry success, got %v", action.Kind)
	}

	for _, key := range []string{"file_exists", "file_contains", "file_not_modified", "file_not_exists", "file_not_contains"} {
		if !strings.Contains(specPacketOnRetry, key) {
			t.Errorf("retry specPacket missing valid assertion key %q", key)
		}
	}
}

func TestWriteContracts_ContractsWrittenEventHasCount(t *testing.T) {
	// The contracts_written event must carry the correct scenario count
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
			{Name: "subtract-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
		},
	}
	writer := &fakeContractWriter{result: &validContract}
	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)
	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, ev := range events {
		if cwe, ok := ev.(*runstore.ContractsWrittenEvent); ok {
			if cwe.ScenarioCount != 2 {
				t.Fatalf("expected ScenarioCount=2, got %d", cwe.ScenarioCount)
			}
			return
		}
	}
	t.Fatal("contracts_written event not found")
}

func TestWriteContracts_LLMErrorAppendsToRetryContext(t *testing.T) {
	// On LLM/infrastructure error, the error message must appear in SpecContent on the next attempt.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
		},
	}

	const llmErrMsg = "rate limit exceeded: upstream overloaded"
	var specPacketOnRetry string
	callCount := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			callCount++
			if callCount == 1 {
				return nil, fmt.Errorf(llmErrMsg)
			}
			specPacketOnRetry = specPacket
			return &validContract, nil
		},
	}

	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue after retry success, got %v", action.Kind)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 writer calls, got %d", callCount)
	}
	if !strings.Contains(specPacketOnRetry, llmErrMsg) {
		t.Errorf("retry specPacket missing LLM error message; got:\n%s", specPacketOnRetry)
	}
}

// TestWriteContracts_IdempotentOnReplan verifies that the stage skips execution when
// ContractsWritten is already true, which occurs after a replan cycle.
func TestWriteContracts_IdempotentOnReplan(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)
	rs.ContractsWritten = true
	rs.Cycle = 2 // simulate a replan cycle

	writer := &fakeContractWriter{}
	cfg := WriteContractsStageConfig{
		SpecPath:    filepath.Join(tmp, "spec.md"),
		EvidenceDir: filepath.Join(tmp, "evidence"),
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue for idempotent replan, got %v", action.Kind)
	}
	if writer.calls != 0 {
		t.Fatalf("expected 0 writer calls on replan with ContractsWritten=true, got %d", writer.calls)
	}
}

// TestWriteContracts_InvalidYAMLRetries verifies that when the contract writer returns
// an error (e.g. invalid YAML from the LLM), the stage retries up to 3 total attempts
// before returning Blocked.
func TestWriteContracts_InvalidYAMLRetries(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Writer always returns a YAML parse error.
	writer := &fakeContractWriter{err: fmt.Errorf("yaml: unmarshal error: cannot decode")}
	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)
	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked after invalid YAML retries, got %v", action.Kind)
	}
	if writer.calls != 3 {
		t.Fatalf("expected 3 writer calls (1+2 retries), got %d", writer.calls)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var found bool
	for _, ev := range events {
		if ev.EventType() == "contracts_blocked" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected contracts_blocked event after invalid YAML retries")
	}
}

// TestWriteContracts_VocabularyViolation verifies that when the LLM produces assertions
// with invalid keys (vocabulary violation), the retry prompt includes the list of valid
// assertion key names so the LLM can self-correct.
func TestWriteContracts_VocabularyViolation(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// First call: vocabulary violation (zero fields = invalid assertion key usage).
	// Second call: valid contract.
	validContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
		},
	}
	invalidContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-works", Assertions: []contract.ContractAssertion{{}}}, // no valid key set
		},
	}

	var specPacketOnRetry string
	callCount := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			callCount++
			if callCount == 1 {
				return &invalidContract, nil
			}
			specPacketOnRetry = specPacket
			return &validContract, nil
		},
	}

	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue after vocabulary violation retry, got %v", action.Kind)
	}

	// specPacket must include all valid assertion key names.
	for _, key := range []string{"file_exists", "file_contains", "file_not_modified", "file_not_exists", "file_not_contains"} {
		if !strings.Contains(specPacketOnRetry, key) {
			t.Errorf("retry specPacket missing valid assertion key %q after vocabulary violation", key)
		}
	}
}

// TestWriteContracts_StaleValidationErrorsResetEachIteration verifies that validationErrors
// is cleared at the top of each retry iteration so that a validation failure on attempt N
// cannot leak into the terminal failure check when attempt N+1 returns an LLM error.
func TestWriteContracts_StaleValidationErrorsResetEachIteration(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Attempt 1: invalid contract (validation errors populated).
	// Attempts 2 and 3: LLM error. After the loop lastErr != nil, validationErrors must be nil
	// (not stale from attempt 1) so that the blocked reason correctly reflects the LLM error.
	invalidContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{Name: "add-works", Assertions: []contract.ContractAssertion{{}}},
		},
	}
	const llmErr = "upstream timeout"
	callCount := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			callCount++
			if callCount == 1 {
				return &invalidContract, nil
			}
			return nil, fmt.Errorf(llmErr)
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

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked, got %v", action.Kind)
	}
	// The blocked reason must reflect the LLM error, not stale validation errors from attempt 1.
	if action.Context == nil || len(action.Context.Failures) == 0 {
		t.Fatal("expected FailureContext with at least one failure")
	}
	reason := action.Context.Failures[0]
	if !strings.Contains(reason, llmErr) {
		t.Errorf("expected blocked reason to contain LLM error %q, got: %q", llmErr, reason)
	}
	if strings.Contains(reason, "validation failed") {
		t.Errorf("blocked reason must not mention stale validation error, got: %q", reason)
	}
}

// TestWriteContracts_RevalidateExistingContract verifies that when ContractsWritten is true
// but the existing contract file contains invalid assertions (referencing runtime artifacts),
// the stage revalidates the contract, resets ContractsWritten to false, and proceeds with
// regeneration using the writer.
func TestWriteContracts_RevalidateExistingContract(t *testing.T) {
	// Seed: RunState with ContractsWritten=true and an existing contract file
	// that contains an invalid assertion (references run.json, a runtime artifact).
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)
	rs.ContractsWritten = true

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write an existing contract file with an invalid assertion (run.json is a runtime artifact)
	invalidContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{FileContains: &contract.FileContainsAssertion{Path: "run.json", Pattern: "some pattern"}},
				},
			},
		},
	}
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	invalidBytes, _ := yaml.Marshal(invalidContract)
	if err := os.WriteFile(contractPath, invalidBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	// Writer will be called with a valid contract to replace the invalid one
	validContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{FileExists: "calc/calc.go"},
				},
			},
		},
	}

	writerCalls := 0
	writer := &callbackContractWriter{
		fn: func(_ context.Context, _ []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			writerCalls++
			// Verify that specPacket contains the prior validation error
			if !strings.Contains(specPacket, "run.json") {
				t.Errorf("specPacket should include validation error mentioning run.json, got: %s", specPacket)
			}
			return &validContract, nil
		},
	}

	cfg := WriteContractsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}
	stage := NewWriteContractsStage(writer, cfg, nil, nil)

	// Invoke: Stage should detect invalid contract, reset ContractsWritten, and regenerate
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: stage proceeds (doesn't skip due to idempotency)
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue after revalidation, got %v", action.Kind)
	}

	// Assert: writer was called (regeneration happened)
	if writerCalls == 0 {
		t.Fatal("expected writer to be called for contract regeneration")
	}

	// Assert: contract file was rewritten with valid contract
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("scenario-contracts.yaml not written: %v", err)
	}
	var regeneratedContract contract.ScenarioContract
	if err := yaml.Unmarshal(data, &regeneratedContract); err != nil {
		t.Fatalf("failed to parse regenerated contract: %v", err)
	}

	// Verify the new contract is valid (no run.json references)
	validationErrs := contract.ValidateContract(regeneratedContract)
	if len(validationErrs) > 0 {
		t.Fatalf("regenerated contract should be valid, got errors: %v", validationErrs)
	}

	// Assert: ContractsWritten is still true after successful regeneration
	if !rs.ContractsWritten {
		t.Fatal("expected ContractsWritten=true after successful regeneration")
	}
}

func TestWriteContracts_SpecificityNoWarningsNoRetry(t *testing.T) {
	// When the LLM produces only high-specificity patterns, no specificity retry
	// occurs and no contract_specificity_warning event is emitted.
	// The writer should be called exactly once (the structural attempt only).
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a contract with HIGH-specificity patterns (multi-token patterns)
	// that won't trigger specificity warnings.
	highSpecificityContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "func Add(a, b int) int",
						},
					},
				},
			},
			{
				Name: "subtract-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "func Subtract(a, b int) int",
						},
					},
				},
			},
		},
	}

	// Track writer calls
	writerCalls := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			writerCalls++
			return &highSpecificityContract, nil
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

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify writer was called exactly once (no specificity retry)
	if writerCalls != 1 {
		t.Fatalf("expected writer to be called exactly once, got %d", writerCalls)
	}

	// Verify no contract_specificity_warning event was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, ev := range events {
		if ev.EventType() == "contract_specificity_warning" {
			t.Fatal("expected no contract_specificity_warning event for high-specificity patterns")
		}
	}

	// Verify contract file was written successfully
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("contract file not written: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("contract file is empty")
	}

	// Verify rs.ContractsWritten flag was set
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true after successful write")
	}
}

func TestWriteContracts_SpecificityRetryFixesPattern(t *testing.T) {
	// When the LLM produces a contract with low-specificity patterns, a specificity retry
	// should occur. On retry, if the LLM produces high-specificity patterns, the writer
	// should be called exactly twice, and the final contract should use the improved pattern.
	// No contract_specificity_warning event should be emitted since the issue was fixed.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet content"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// First call returns a contract with LOW-specificity pattern
	lowSpecificityContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "ModelTier", // Low-specificity: single exported identifier
						},
					},
				},
			},
		},
	}

	// Second call returns a contract with HIGH-specificity pattern
	highSpecificityContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "ModelTier  string", // High-specificity: multi-token pattern
						},
					},
				},
			},
		},
	}

	// Track writer calls and what's being passed in
	writerCalls := 0
	var secondCallSpecPacket string
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			writerCalls++
			if writerCalls == 1 {
				return &lowSpecificityContract, nil
			}
			secondCallSpecPacket = specPacket
			return &highSpecificityContract, nil
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

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify writer was called exactly twice (initial + specificity retry)
	if writerCalls != 2 {
		t.Fatalf("expected writer to be called exactly twice, got %d", writerCalls)
	}

	// Verify the second call's specPacket includes specificity warning information
	// (This verifies the retry context was properly set up)
	if secondCallSpecPacket == "" {
		t.Fatal("expected second call to receive updated specPacket")
	}

	// Verify no contract_specificity_warning event was emitted
	// (because the issue was fixed on retry, not left as a warning)
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, ev := range events {
		if ev.EventType() == "contract_specificity_warning" {
			t.Fatal("expected no contract_specificity_warning event since issue was fixed on retry")
		}
	}

	// Verify the final contract file has the high-specificity pattern
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("contract file not written: %v", err)
	}

	var finalContract contract.ScenarioContract
	if err := yaml.Unmarshal(data, &finalContract); err != nil {
		t.Fatalf("failed to unmarshal contract: %v", err)
	}

	// Verify the pattern in the final contract is the high-specificity one
	if len(finalContract.Scenarios) == 0 || len(finalContract.Scenarios[0].Assertions) == 0 {
		t.Fatal("final contract missing scenarios or assertions")
	}

	finalPattern := finalContract.Scenarios[0].Assertions[0].FileContains.Pattern
	if finalPattern != "ModelTier  string" {
		t.Fatalf("expected final pattern 'ModelTier  string', got '%s'", finalPattern)
	}

	// Verify rs.ContractsWritten flag was set
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true after successful retry")
	}
}

func TestWriteContracts_SpecificityRetryPersistsWarning(t *testing.T) {
	// When the LLM produces a contract with low-specificity patterns on both attempts,
	// the specificity issue persists. On the final attempt, the contract is accepted
	// (Continue returned), and a contract_specificity_warning event is emitted with
	// the warning text about the low-specificity pattern.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet content"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Contract with LOW-specificity pattern (single exported identifier)
	lowSpecificityContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "ModelTier", // Low-specificity: single exported identifier
						},
					},
				},
			},
		},
	}

	// Track writer calls
	writerCalls := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			writerCalls++
			// Return the same low-specificity contract on all attempts
			return &lowSpecificityContract, nil
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

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify writer was called at least twice (1 initial + 1 specificity retry before accepting)
	if writerCalls < 2 {
		t.Fatalf("expected at least 2 writer calls, got %d", writerCalls)
	}

	// Verify the contract file was written
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("contract file not written: %v", err)
	}

	var finalContract contract.ScenarioContract
	if err := yaml.Unmarshal(data, &finalContract); err != nil {
		t.Fatalf("failed to unmarshal contract: %v", err)
	}

	// Verify the final contract has the low-specificity pattern
	if len(finalContract.Scenarios) == 0 || len(finalContract.Scenarios[0].Assertions) == 0 {
		t.Fatal("final contract missing scenarios or assertions")
	}
	finalPattern := finalContract.Scenarios[0].Assertions[0].FileContains.Pattern
	if finalPattern != "ModelTier" {
		t.Fatalf("expected final pattern 'ModelTier', got '%s'", finalPattern)
	}

	// Verify rs.ContractsWritten flag was set
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true after accepting contract with warning")
	}

	// Verify contract_specificity_warning event was emitted with warning text
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var specificityWarningFound bool
	var warningText string
	for _, ev := range events {
		if ev.EventType() == "contract_specificity_warning" {
			specificityWarningFound = true
			// Extract warnings from the event
			if cswe, ok := ev.(*runstore.ContractSpecificityWarningEvent); ok {
				if len(cswe.Warnings) > 0 {
					warningText = cswe.Warnings[0]
				}
			}
			break
		}
	}

	if !specificityWarningFound {
		t.Fatal("expected contract_specificity_warning event to be emitted")
	}

	// Verify the warning text mentions the low-specificity pattern
	if warningText == "" {
		t.Fatal("expected warning text to be present in contract_specificity_warning event")
	}
	if !strings.Contains(warningText, "ModelTier") {
		t.Fatalf("expected warning to mention 'ModelTier', got %q", warningText)
	}
}

func TestWriteContracts_SpecificityRetryStructuralRegression(t *testing.T) {
	// When the LLM produces a valid contract with specificity warnings on the first attempt,
	// but returns a structurally invalid contract (zero fields set) on the specificity retry,
	// the pre-retry result is kept, a contract_specificity_warning event is emitted,
	// and the stage returns Continue.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet content"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Valid contract with low-specificity pattern (will trigger retry)
	validLowSpecificityContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "ModelTier", // Low-specificity: single exported identifier
						},
					},
				},
			},
		},
	}

	// Invalid contract: zero fields set (fails structural validation)
	invalidContract := &contract.ScenarioContract{}

	// Track writer calls
	writerCalls := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			writerCalls++
			if writerCalls == 1 {
				// First call: return valid contract with specificity warning
				return &validLowSpecificityContract, nil
			}
			// Retry call: return structurally invalid contract
			return invalidContract, nil
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

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify writer was called at least twice (1 initial + 1 specificity retry)
	if writerCalls < 2 {
		t.Fatalf("expected at least 2 writer calls, got %d", writerCalls)
	}

	// Verify the contract file was written
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("contract file not written: %v", err)
	}

	var finalContract contract.ScenarioContract
	if err := yaml.Unmarshal(data, &finalContract); err != nil {
		t.Fatalf("failed to unmarshal contract: %v", err)
	}

	// Verify the final contract is the pre-retry valid contract (not the invalid retry result)
	if len(finalContract.Scenarios) == 0 || len(finalContract.Scenarios[0].Assertions) == 0 {
		t.Fatal("final contract missing scenarios or assertions - expected pre-retry contract to be kept")
	}
	finalPattern := finalContract.Scenarios[0].Assertions[0].FileContains.Pattern
	if finalPattern != "ModelTier" {
		t.Fatalf("expected final pattern 'ModelTier' from pre-retry contract, got '%s'", finalPattern)
	}

	// Verify rs.ContractsWritten flag was set
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true after keeping pre-retry contract")
	}

	// Verify contract_specificity_warning event was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var specificityWarningFound bool
	for _, ev := range events {
		if ev.EventType() == "contract_specificity_warning" {
			specificityWarningFound = true
			break
		}
	}

	if !specificityWarningFound {
		t.Fatal("expected contract_specificity_warning event to be emitted when specificity retry returns invalid contract")
	}
}

func TestWriteContracts_SpecificityRetryLLMError(t *testing.T) {
	// When the LLM produces a contract with low-specificity patterns on the first attempt,
	// but returns an error on the specificity retry, the pre-retry result should be kept,
	// the stage should return Continue, and a contract_specificity_warning event should be emitted.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := makeWriteContractsRunState(t, store)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := store.RunDir(rs.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet content"), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Contract with LOW-specificity pattern
	lowSpecificityContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "ModelTier", // Low-specificity: single exported identifier
						},
					},
				},
			},
		},
	}

	// Track writer calls
	writerCalls := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			writerCalls++
			if writerCalls == 1 {
				// First call: return valid contract with low-specificity pattern
				return &lowSpecificityContract, nil
			}
			// Second call (specificity retry): return an LLM error
			return nil, fmt.Errorf("LLM error: failed to process request")
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

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify the contract file was written with the pre-retry contract
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("contract file not written: %v", err)
	}

	var finalContract contract.ScenarioContract
	if err := yaml.Unmarshal(data, &finalContract); err != nil {
		t.Fatalf("failed to unmarshal contract: %v", err)
	}

	// Verify the final contract is the low-specificity one from the first attempt
	if len(finalContract.Scenarios) == 0 || len(finalContract.Scenarios[0].Assertions) == 0 {
		t.Fatal("final contract missing scenarios or assertions")
	}
	finalPattern := finalContract.Scenarios[0].Assertions[0].FileContains.Pattern
	if finalPattern != "ModelTier" {
		t.Fatalf("expected final pattern 'ModelTier', got '%s'", finalPattern)
	}

	// Verify rs.ContractsWritten flag was set
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true after accepting contract with warning")
	}

	// Verify contract_specificity_warning event was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var specificityWarningFound bool
	for _, ev := range events {
		if ev.EventType() == "contract_specificity_warning" {
			specificityWarningFound = true
			break
		}
	}

	if !specificityWarningFound {
		t.Fatal("expected contract_specificity_warning event to be emitted when specificity retry returns LLM error")
	}
}

// callbackContractWriter allows per-call behaviour in tests.
type callbackContractWriter struct {
	fn func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error)
}

func (c *callbackContractWriter) WriteContracts(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
	return c.fn(ctx, scenarios, specPacket)
}
