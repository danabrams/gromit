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

// fakeContractEvaluatorProceedCorrection simulates a contract evaluator where
// a pattern is in a sibling file, and the AC does not mention the original file,
// so the correction should proceed normally.
type fakeContractEvaluatorProceedCorrection struct {
	callCount int
}

func (f *fakeContractEvaluatorProceedCorrection) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	// On second call (after correction), should succeed because sibling file has the pattern
	if f.callCount == 2 {
		return []contract.ContractFailure{}, nil
	}
	// On first call, return failure
	return []contract.ContractFailure{
		{
			ScenarioName:  "proceed-correction-scenario",
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

// TestScenario_ContractCorrectionProceedsWhenACDoesNotMentionOriginalFile verifies that when a spec's
// acceptance criteria text does NOT contain the original file path (e.g., just mentions the feature generically),
// the contract correction proceeds normally: the pattern is found in a sibling file, the correction is accepted,
// a contract_correction_accepted event is emitted, and the contract is updated to point to the sibling file.
// Test fixture branch: feature/beta-branch
func TestScenario_ContractCorrectionProceedsWhenACDoesNotMentionOriginalFile(t *testing.T) {
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
	// - write_contracts_test.go: missing the pattern (original file, contract points here initially)
	// - stages_scenario_structural_regression_test.go: contains the pattern (sibling file, correction target)
	pattern := "StructuralRegression"
	if err := os.WriteFile(filepath.Join(worktreePath, "write_contracts_test.go"), []byte("package main\n// no pattern here\n"), 0o644); err != nil {
		t.Fatalf("create write_contracts_test.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "stages_scenario_structural_regression_test.go"), []byte("package main\n// "+pattern+"\n"), 0o644); err != nil {
		t.Fatalf("create stages_scenario_structural_regression_test.go: %v", err)
	}

	// Create scenario contract YAML pointing to write_contracts_test.go
	contractYAML := `scenarios:
- name: proceed-correction-scenario
  assertions:
  - file_contains:
      path: write_contracts_test.go
      pattern: StructuralRegression
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Create a spec with AC text that does NOT mention write_contracts_test.go
	// This allows correction to proceed normally
	specText := `# Spec 0004h

## Vision
Contract correction validation...

## Acceptance Criteria

1. Implement StructuralRegression feature
2. The correction should proceed when the AC does not mention the specific file
3. A contract_correction_accepted event should be emitted

## Scenarios
...
`

	// Create fake evaluator
	fakeEval := &fakeContractEvaluatorProceedCorrection{}

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

	// Verify: should continue (because contract passes after correction) rather than replan
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify: contract_corrected event was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var foundCorrectedEvent bool
	for _, ev := range events {
		if typedEv, ok := ev.(*runstore.ContractCorrectedEvent); ok {
			foundCorrectedEvent = true
			if typedEv.ScenarioName != "proceed-correction-scenario" {
				t.Errorf("expected scenario name 'proceed-correction-scenario', got %q", typedEv.ScenarioName)
			}
			// Verify the correction details
			if typedEv.NewPath != "stages_scenario_structural_regression_test.go" {
				t.Errorf("expected new path 'stages_scenario_structural_regression_test.go', got %q", typedEv.NewPath)
			}
			if typedEv.OldPath != "write_contracts_test.go" {
				t.Errorf("expected old path 'write_contracts_test.go', got %q", typedEv.OldPath)
			}
		}
	}

	if !foundCorrectedEvent {
		t.Fatal("expected contract_corrected event to be emitted")
	}

	// Verify: evaluator was called twice (once for initial check, once after correction)
	if fakeEval.callCount != 2 {
		t.Errorf("expected evaluator to be called twice, got %d", fakeEval.callCount)
	}

	// Verify: contract should be corrected to point to sibling file
	contractContent, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if !strings.Contains(string(contractContent), "stages_scenario_structural_regression_test.go") {
		t.Fatal("expected contract to be corrected to stages_scenario_structural_regression_test.go")
	}
	if strings.Contains(string(contractContent), "write_contracts_test.go") {
		t.Fatal("contract should be corrected away from write_contracts_test.go")
	}
}
