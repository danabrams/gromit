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

// fakeContractEvaluatorRejectCorrection simulates a contract evaluator where
// a pattern is in a sibling file, but the original file is mentioned in the spec AC,
// so the correction should be rejected.
type fakeContractEvaluatorRejectCorrection struct {
	callCount int
}

func (f *fakeContractEvaluatorRejectCorrection) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	// Always return failure (simulates that the pattern is not found in the specified file)
	return []contract.ContractFailure{
		{
			ScenarioName:  "reject-correction-scenario",
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

// TestScenario_ContractCorrectionRejectedWhenACNamesOriginalFile verifies that when a spec's
// acceptance criteria text contains the original file path (e.g., "keep tests in write_contracts_test.go"),
// the contract correction is rejected, the failure remains in the remaining list, and a
// contract_correction_rejected event is emitted.
// Test fixture branch: feature/beta-branch
func TestScenario_ContractCorrectionRejectedWhenACNamesOriginalFile(t *testing.T) {
	dir := t.TempDir()

	// Create a healthy worktree
	worktreePath := filepath.Join(dir, ".gromit-next", "worktrees", "wt-002")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}

	// Create evidence directory with contract file
	evidenceDir := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Create real files in worktree:
	// - write_contracts_test.go: missing the pattern (spec says to keep it here)
	// - stages_scenario_structural_regression_test.go: contains the pattern (would normally correct to this)
	pattern := "StructuralRegression"
	if err := os.WriteFile(filepath.Join(worktreePath, "write_contracts_test.go"), []byte("package main\n// no pattern here\n"), 0o644); err != nil {
		t.Fatalf("create write_contracts_test.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "stages_scenario_structural_regression_test.go"), []byte("package main\n// "+pattern+"\n"), 0o644); err != nil {
		t.Fatalf("create stages_scenario_structural_regression_test.go: %v", err)
	}

	// Create scenario contract YAML pointing to write_contracts_test.go
	contractYAML := `scenarios:
- name: reject-correction-scenario
  assertions:
  - file_contains:
      path: write_contracts_test.go
      pattern: StructuralRegression
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Create a spec with AC text that explicitly mentions write_contracts_test.go
	specText := `# Spec 0004h

## Vision
Contract correction validation...

## Acceptance Criteria

1. keep StructuralRegression test in write_contracts_test.go
2. The correction should be rejected when the AC mentions the file
3. A contract_correction_rejected event should be emitted

## Scenarios
...
`

	// Create fake evaluator
	fakeEval := &fakeContractEvaluatorRejectCorrection{}

	// Create event log to capture emitted events
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// Create fake validator that always passes
	fakeValidator := &fakeValidator{
		result: validator.FinalResult{Pass: true},
	}

	// Create validate stage with spec text so it can check AC
	stage := NewValidateStage(fakeValidator, ValidateStageConfig{
		WorkDir:          worktreePath,
		RepoDir:          dir,
		EvidenceDir:      evidenceDir,
		SearchExtensions: []string{".go"},
		SpecText:         specText,
	}, eventLog, fakeEval, &validateScenarioFakeGitOps{})

	// Run validation
	rs := runstore.NewRunState("spec-004h", "proj-001")
	rs.WorktreePath = worktreePath
	rs.SpecID = "spec-004h"
	rs.RunID = "run-002"
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify: should replan (because contract still fails) rather than continue
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}

	// Verify: contract_correction_rejected event was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var foundRejectedEvent bool
	for _, ev := range events {
		if typedEv, ok := ev.(*runstore.ContractCorrectionRejectedEvent); ok {
			foundRejectedEvent = true
			if typedEv.ScenarioName != "reject-correction-scenario" {
				t.Errorf("expected scenario name 'reject-correction-scenario', got %q", typedEv.ScenarioName)
			}
			if typedEv.Reason == "" {
				t.Errorf("expected non-empty reason, got %q", typedEv.Reason)
			}
			// Verify reason mentions the spec AC guard
			if !strings.Contains(typedEv.Reason, "write_contracts_test.go") {
				t.Errorf("expected reason to mention original file 'write_contracts_test.go', got %q", typedEv.Reason)
			}
		}
	}

	if !foundRejectedEvent {
		t.Fatal("expected contract_correction_rejected event to be emitted")
	}

	// Verify: evaluator was called exactly once (correction was rejected, so no re-evaluation)
	if fakeEval.callCount != 1 {
		t.Errorf("expected evaluator to be called once, got %d", fakeEval.callCount)
	}

	// Verify: contract should still point to original file (not corrected)
	contractContent, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if !strings.Contains(string(contractContent), "write_contracts_test.go") {
		t.Fatal("expected contract to still point to write_contracts_test.go (correction should be rejected)")
	}
	if strings.Contains(string(contractContent), "stages_scenario_structural_regression_test.go") {
		t.Fatal("contract should not be corrected to stages_scenario_structural_regression_test.go")
	}
}
