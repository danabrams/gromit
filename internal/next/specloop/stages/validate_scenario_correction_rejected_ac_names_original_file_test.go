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

// fakeContractEvaluatorACNamesOriginalFile simulates a contract evaluator where
// the AC explicitly names the original file, so the correction should be rejected
// even though a sibling file contains the pattern.
type fakeContractEvaluatorACNamesOriginalFile struct {
	callCount int
}

func (f *fakeContractEvaluatorACNamesOriginalFile) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	return []contract.ContractFailure{
		{
			ScenarioName:  "ac-names-original-file-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "StructuralRegression" not found in "write_contracts_test.go"`,
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "write_contracts_test.go",
					Pattern: "StructuralRegression",
				},
			},
		},
	}, nil
}

// TestScenario_CorrectionRejectedWhenACNamesOriginalFile verifies that when a spec's
// acceptance criteria text explicitly names the original file (e.g., "keep StructuralRegression
// test in write_contracts_test.go"), the contract correction is rejected even though a sibling
// file (stages_scenario_structural_regression_test.go) contains the pattern.
// The contract must remain pointing at write_contracts_test.go, a contract_correction_rejected
// event must be emitted, and the failure must remain in the remaining slice (causing ReplanFrom).
func TestScenario_CorrectionRejectedWhenACNamesOriginalFile(t *testing.T) {
	dir := t.TempDir()

	// Seed: create a healthy worktree
	worktreePath := filepath.Join(dir, ".gromit-next", "worktrees", "wt-ac-names")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}

	// Seed: create evidence directory
	evidenceDir := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Seed: create files in the worktree root
	// - write_contracts_test.go: missing the pattern (original file the contract points to)
	// - stages_scenario_structural_regression_test.go: contains the pattern (would be the correction target)
	pattern := "StructuralRegression"
	if err := os.WriteFile(filepath.Join(worktreePath, "write_contracts_test.go"), []byte("package stages\n// no pattern here\n"), 0o644); err != nil {
		t.Fatalf("create write_contracts_test.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "stages_scenario_structural_regression_test.go"), []byte("package stages\n// "+pattern+"\n"), 0o644); err != nil {
		t.Fatalf("create stages_scenario_structural_regression_test.go: %v", err)
	}

	// Seed: create scenario contract YAML pointing to write_contracts_test.go
	contractYAML := `scenarios:
- name: ac-names-original-file-scenario
  assertions:
  - file_contains:
      path: write_contracts_test.go
      pattern: StructuralRegression
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Seed: create a spec with AC text that explicitly names write_contracts_test.go
	// This is the key: the AC says "keep StructuralRegression test in write_contracts_test.go"
	specText := `# Spec 0004h

## Vision
Contract correction guard when AC names the original file...

## Acceptance Criteria

1. keep StructuralRegression test in write_contracts_test.go
2. The correction should be rejected when AC mentions the original file by name
3. A contract_correction_rejected event should be emitted

## Scenarios
...
`

	// Seed: fake evaluator always returns failure (correction is rejected, no re-evaluation)
	fakeEval := &fakeContractEvaluatorACNamesOriginalFile{}

	// Seed: event log to capture emitted events
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// Seed: fake validator that always passes (so only contract failures drive the outcome)
	fakeVal := &fakeValidator{
		result: validator.FinalResult{Pass: true},
	}

	// Seed: validate stage with spec text so it can check AC
	stage := NewValidateStage(fakeVal, ValidateStageConfig{
		WorkDir:          worktreePath,
		RepoDir:          dir,
		EvidenceDir:      evidenceDir,
		SearchExtensions: []string{".go"},
		SpecText:         specText,
	}, eventLog, fakeEval, &validateScenarioFakeGitOps{})

	rs := runstore.NewRunState("spec-004h", "proj-001")
	rs.WorktreePath = worktreePath
	rs.SpecID = "spec-004h"
	rs.RunID = "run-ac-names"
	rs.Cycle = 1

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: correction was rejected → contract still fails → ReplanFrom
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom (contract still failing), got %v", action.Kind)
	}

	// Assert: contract_correction_rejected event was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var foundRejectedEvent bool
	for _, ev := range events {
		if typedEv, ok := ev.(*runstore.ContractCorrectionRejectedEvent); ok {
			foundRejectedEvent = true
			if typedEv.ScenarioName != "ac-names-original-file-scenario" {
				t.Errorf("expected scenario name 'ac-names-original-file-scenario', got %q", typedEv.ScenarioName)
			}
			if typedEv.Reason == "" {
				t.Errorf("expected non-empty reason in rejection event")
			}
			if !strings.Contains(typedEv.Reason, "write_contracts_test.go") {
				t.Errorf("expected rejection reason to mention 'write_contracts_test.go', got %q", typedEv.Reason)
			}
			if typedEv.OldPath != "write_contracts_test.go" {
				t.Errorf("expected OldPath to be 'write_contracts_test.go', got %q", typedEv.OldPath)
			}
			if typedEv.CandidatePath != "stages_scenario_structural_regression_test.go" {
				t.Errorf("expected CandidatePath to be 'stages_scenario_structural_regression_test.go', got %q", typedEv.CandidatePath)
			}
		}
	}

	if !foundRejectedEvent {
		t.Fatal("expected contract_correction_rejected event to be emitted")
	}

	// Assert: no contract_corrected event (correction was rejected)
	for _, ev := range events {
		if _, ok := ev.(*runstore.ContractCorrectedEvent); ok {
			t.Fatal("expected no contract_corrected event — correction should have been rejected")
		}
	}

	// Assert: evaluator called exactly once (no re-evaluation after rejected correction)
	if fakeEval.callCount != 1 {
		t.Errorf("expected evaluator called once (no re-evaluation after rejection), got %d", fakeEval.callCount)
	}

	// Assert: contract still points to write_contracts_test.go (not corrected)
	contractContent, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if !strings.Contains(string(contractContent), "write_contracts_test.go") {
		t.Fatal("expected contract to still point to write_contracts_test.go after rejected correction")
	}
	if strings.Contains(string(contractContent), "stages_scenario_structural_regression_test.go") {
		t.Fatal("contract must not be corrected to stages_scenario_structural_regression_test.go")
	}
}
