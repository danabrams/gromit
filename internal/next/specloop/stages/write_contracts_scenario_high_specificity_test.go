package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_HighSpecificityFirstAttempt_NoRetry(t *testing.T) {
	// Seed: store with a fresh RunState, spec with scenarios, and a writer
	// that returns only multi-token (high-specificity) patterns.
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

	// All patterns are multi-token — high specificity, no warnings expected.
	highSpecContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "internal/calc/calc.go",
							Pattern: "ModelTier  string",
						},
					},
				},
			},
			{
				Name: "subtract-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "internal/calc/types.go",
							Pattern: "type DistillationResult struct",
						},
					},
				},
			},
		},
	}

	writerCalls := 0
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			writerCalls++
			return &highSpecContract, nil
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

	// Assert: stage returns Continue
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Assert: writer called exactly once — no specificity retry
	if writerCalls != 1 {
		t.Fatalf("expected exactly 1 writer call (no retry), got %d", writerCalls)
	}

	// Assert: no contract_specificity_warning event emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, ev := range events {
		if ev.EventType() == "contract_specificity_warning" {
			t.Fatal("expected no contract_specificity_warning event for high-specificity patterns")
		}
	}

	// Assert: contracts_written event was emitted with correct count
	var contractsWrittenFound bool
	for _, ev := range events {
		if cwe, ok := ev.(*runstore.ContractsWrittenEvent); ok {
			contractsWrittenFound = true
			if cwe.ScenarioCount != 2 {
				t.Fatalf("expected ScenarioCount=2, got %d", cwe.ScenarioCount)
			}
		}
	}
	if !contractsWrittenFound {
		t.Fatal("expected contracts_written event")
	}

	// Assert: ContractsWritten flag set
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true")
	}
}
