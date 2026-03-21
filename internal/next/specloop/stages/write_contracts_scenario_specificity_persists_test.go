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
	"gopkg.in/yaml.v3"
)

func TestScenario_LowSpecificityPersistsAfterRetry_AcceptedWithWarning(t *testing.T) {
	// Scenario: The LLM generates "ModelTier" (a single exported identifier) as the
	// file_contains pattern on both the initial attempt and the specificity retry.
	// The contract should be accepted with a warning event, and the stage returns Continue.

	// --- Seed ---
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

	// Both attempts return the same low-specificity pattern "ModelTier"
	lowSpecContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{FileContains: &contract.FileContainsAssertion{
						Path:    "calc/calc.go",
						Pattern: "ModelTier",
					}},
				},
			},
		},
	}

	writerCalls := 0
	writer := &callbackContractWriter{
		fn: func(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
			writerCalls++
			return &lowSpecContract, nil
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

	// --- Invoke ---
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// --- Assert ---

	// Stage returns Continue (contract accepted despite low specificity)
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Writer called twice: initial attempt + one specificity retry
	if writerCalls != 2 {
		t.Fatalf("expected 2 writer calls (initial + specificity retry), got %d", writerCalls)
	}

	// ContractsWritten flag set
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true")
	}

	// Contract file written with the low-specificity pattern
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
		t.Fatal("final contract missing scenarios or assertions")
	}
	if finalContract.Scenarios[0].Assertions[0].FileContains.Pattern != "ModelTier" {
		t.Fatalf("expected pattern 'ModelTier', got %q", finalContract.Scenarios[0].Assertions[0].FileContains.Pattern)
	}

	// contract_specificity_warning event emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var warningEvent *runstore.ContractSpecificityWarningEvent
	for _, ev := range events {
		if cswe, ok := ev.(*runstore.ContractSpecificityWarningEvent); ok {
			warningEvent = cswe
			break
		}
	}
	if warningEvent == nil {
		t.Fatal("expected contract_specificity_warning event to be emitted")
	}
	if len(warningEvent.Warnings) == 0 {
		t.Fatal("expected at least one warning in contract_specificity_warning event")
	}
	if !strings.Contains(warningEvent.Warnings[0], "ModelTier") {
		t.Fatalf("expected warning to mention 'ModelTier', got %q", warningEvent.Warnings[0])
	}
}
