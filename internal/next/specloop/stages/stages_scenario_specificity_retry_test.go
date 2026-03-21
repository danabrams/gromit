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

func TestScenario_LowSpecificityPatternTriggersOneRetry(t *testing.T) {
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

	// First attempt: low-specificity pattern (single exported identifier)
	lowSpecContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "ModelTier",
						},
					},
				},
			},
		},
	}

	// Second attempt: high-specificity pattern (multi-token)
	highSpecContract := contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "add-works",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "calc/calc.go",
							Pattern: "ModelTier  string",
						},
					},
				},
			},
		},
	}

	writerCalls := 0
	var retrySpecPacket string
	writer := &callbackContractWriter{
		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
			writerCalls++
			if writerCalls == 1 {
				return &lowSpecContract, nil
			}
			retrySpecPacket = specPacket
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

	// --- Invoke ---
	action, err := stage.Run(context.Background(), rs)

	// --- Assert ---
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Exactly two LLM calls: initial + one specificity retry
	if writerCalls != 2 {
		t.Fatalf("expected exactly 2 writer calls (initial + specificity retry), got %d", writerCalls)
	}

	// The retry prompt must mention the specificity issue
	if !strings.Contains(retrySpecPacket, "Specificity") {
		t.Error("retry specPacket missing specificity context")
	}
	if !strings.Contains(retrySpecPacket, "ModelTier") {
		t.Error("retry specPacket should mention the flagged pattern 'ModelTier'")
	}

	// No contract_specificity_warning event — the issue was fixed
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, ev := range events {
		if ev.EventType() == "contract_specificity_warning" {
			t.Fatal("expected no contract_specificity_warning event since retry fixed the pattern")
		}
	}

	// contracts_written event must be present
	var contractsWrittenFound bool
	for _, ev := range events {
		if ev.EventType() == "contracts_written" {
			contractsWrittenFound = true
		}
	}
	if !contractsWrittenFound {
		t.Fatal("expected contracts_written event")
	}

	// Final contract file must contain the improved pattern
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("contract file not written: %v", err)
	}

	var finalContract contract.ScenarioContract
	if err := yaml.Unmarshal(data, &finalContract); err != nil {
		t.Fatalf("failed to unmarshal contract: %v", err)
	}
	if len(finalContract.Scenarios) == 0 || len(finalContract.Scenarios[0].Assertions) == 0 {
		t.Fatal("final contract missing scenarios or assertions")
	}
	finalPattern := finalContract.Scenarios[0].Assertions[0].FileContains.Pattern
	if finalPattern != "ModelTier  string" {
		t.Fatalf("expected final pattern 'ModelTier  string', got %q", finalPattern)
	}

	// ContractsWritten flag must be set
	if !rs.ContractsWritten {
		t.Fatal("expected rs.ContractsWritten=true")
	}
}
