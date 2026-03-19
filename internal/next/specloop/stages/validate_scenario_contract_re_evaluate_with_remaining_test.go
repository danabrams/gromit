package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// fakeContractEvaluatorMultipleCallsWithRemaining tracks calls and returns
// different failures on first vs second Evaluate call. This simulates the bug
// scenario where corrections are made but uncorrectable failures remain.
// On first call: mixed failures (some correctable, some uncorrectable)
// On second call: only uncorrectable failures (patterns not found anywhere)
type fakeContractEvaluatorMultipleCallsWithRemaining struct {
	callCount int
	// firstCallFailures: failures on first Evaluate call (mixed)
	firstCallFailures []contract.ContractFailure
	// secondCallFailures: failures on second Evaluate call (should be used instead of remaining)
	secondCallFailures []contract.ContractFailure
}

func (f *fakeContractEvaluatorMultipleCallsWithRemaining) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	if f.callCount == 1 {
		return f.firstCallFailures, nil
	}
	// Second call returns different failures (simulating the re-evaluation result)
	return f.secondCallFailures, nil
}

// TestScenario_ContractReEvaluateWithRemainingFailures verifies that when
// corrections are made but uncorrectable failures remain, the evaluator is
// called a second time and the second evaluation result is used for determining
// failures (not the "remaining" from the first evaluation).
//
// This test addresses the bug where the conditional `if len(remaining) == 0`
// prevented re-evaluation when uncorrectable failures existed. The spec requires
// the second evaluation result to be used for rest of validate stage logic.
func TestScenario_ContractReEvaluateWithRemainingFailures(t *testing.T) {
	dir := t.TempDir()

	// Create a healthy worktree
	worktreePath := filepath.Join(dir, ".gromit-next", "worktrees", "wt-001")
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
	// - corrected_file.txt contains the correctable pattern
	if err := os.WriteFile(filepath.Join(worktreePath, "corrected_file.txt"), []byte("corrected pattern"), 0o644); err != nil {
		t.Fatalf("create corrected_file.txt: %v", err)
	}

	// Create scenario contract YAML with 2 assertions:
	// - 1 correctable (pattern in sibling file)
	// - 1 uncorrectable (pattern not found anywhere)
	contractYAML := `scenarios:
- name: test-scenario
  assertions:
  - file_contains:
      path: wrong_file.txt
      pattern: corrected pattern
  - file_contains:
      path: some_file.txt
      pattern: pattern-not-found
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Create fake evaluator that returns:
	// - First call: 2 failures (1 correctable, 1 uncorrectable)
	// - Second call: 1 failure (only the uncorrectable after re-evaluation)
	fakeEval := &fakeContractEvaluatorMultipleCallsWithRemaining{
		firstCallFailures: []contract.ContractFailure{
			{
				ScenarioName:  "test-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "corrected pattern" not found in "wrong_file.txt"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "wrong_file.txt",
						Pattern: "corrected pattern",
					},
				},
			},
			{
				ScenarioName:  "test-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "pattern-not-found" not found in "some_file.txt"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "some_file.txt",
						Pattern: "pattern-not-found",
					},
				},
			},
		},
		// On re-evaluation, the contract has been corrected to point to corrected_file.txt,
		// so only the uncorrectable failure remains
		secondCallFailures: []contract.ContractFailure{
			{
				ScenarioName:  "test-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "pattern-not-found" not found in "some_file.txt"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "some_file.txt",
						Pattern: "pattern-not-found",
					},
				},
			},
		},
	}

	// Create event log to capture emitted events
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// Create fake validator that always passes
	fakeValidator := &fakeValidator{
		result: validator.FinalResult{Pass: true},
	}

	// Create validate stage with evaluator and event log
	stage := NewValidateStage(fakeValidator, ValidateStageConfig{
		WorkDir:          worktreePath,
		RepoDir:          dir,
		EvidenceDir:      evidenceDir,
		SearchExtensions: []string{".txt"},
	}, eventLog, fakeEval, &validateScenarioFakeGitOps{})

	// Run validation
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = worktreePath
	rs.SpecID = "spec-001"
	rs.RunID = "run-001"
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify: should return ReplanFrom due to uncorrectable failure
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}

	// CRITICAL: Verify that evaluator was called TWICE
	// - First call: initial evaluation with mixed failures
	// - Second call: re-evaluation after corrections (this is the fix!)
	if fakeEval.callCount != 2 {
		t.Errorf("expected evaluator to be called exactly twice (initial + re-evaluate), got %d", fakeEval.callCount)
	}

	// Verify: FailureContext contains only the failure from the second evaluation
	// (not both failures from the first evaluation)
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) != 1 {
		t.Errorf("expected 1 failure from second evaluation, got %d", len(action.Context.Failures))
	}

	// Verify: FinalValidationPassed was NOT set (due to uncorrectable failure)
	if rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be false")
	}
}
