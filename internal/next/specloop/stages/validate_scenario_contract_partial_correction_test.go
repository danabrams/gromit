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

// fakeContractEvaluatorPartialCorrection returns mixed failures on first call:
// some failures can be corrected (pattern exists in sibling files),
// others cannot be corrected (pattern doesn't exist anywhere).
// On the second call (after correction), returns only the uncorrectable failure.
// This simulates a contract evaluator where some assertions can be auto-corrected
// and others require a replan.
type fakeContractEvaluatorPartialCorrection struct {
	callCount              int
	firstCallFailures      []contract.ContractFailure
	secondCallFailures     []contract.ContractFailure
}

func (f *fakeContractEvaluatorPartialCorrection) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	if f.callCount == 1 {
		return f.firstCallFailures, nil
	}
	return f.secondCallFailures, nil
}

// TestScenario_ContractPartialCorrection verifies that when a scenario contract
// has multiple assertions with mixed results—some with patterns found in sibling
// files (correctable) and some with patterns not found anywhere (uncorrectable)—
// the correctable failures are fixed via contract correction, uncorrectable failures
// propagate to ReplanFrom with proper failure context, and appropriate events are
// emitted.
func TestScenario_ContractPartialCorrection(t *testing.T) {
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
	// - correct_file_1.txt contains pattern1 (correctable failure 1)
	// - correct_file_2.txt contains pattern2 (correctable failure 2)
	// - some_file.txt exists but doesn't contain the uncorrectable pattern
	if err := os.WriteFile(filepath.Join(worktreePath, "correct_file_1.txt"), []byte("expected output 1"), 0o644); err != nil {
		t.Fatalf("create correct_file_1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "correct_file_2.txt"), []byte("expected output 2"), 0o644); err != nil {
		t.Fatalf("create correct_file_2.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "some_file.txt"), []byte("other content"), 0o644); err != nil {
		t.Fatalf("create some_file.txt: %v", err)
	}

	// Create scenario contract YAML with 3 assertions:
	// - 2 assertions pointing to wrong files but patterns exist in correct sibling files (correctable)
	// - 1 assertion pointing to wrong file and pattern doesn't exist anywhere (uncorrectable)
	contractYAML := `scenarios:
- name: test-scenario
  assertions:
  - file_contains:
      path: wrong_file_1.txt
      pattern: expected output 1
  - file_contains:
      path: wrong_file_2.txt
      pattern: expected output 2
  - file_contains:
      path: some_file.txt
      pattern: this-pattern-does-not-exist-anywhere
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Create fake evaluator with mixed failures on first call,
	// only uncorrectable failure on second call (after corrections)
	fakeEval := &fakeContractEvaluatorPartialCorrection{
		firstCallFailures: []contract.ContractFailure{
			{
				ScenarioName:  "test-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "expected output 1" not found in "wrong_file_1.txt"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "wrong_file_1.txt",
						Pattern: "expected output 1",
					},
				},
			},
			{
				ScenarioName:  "test-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "expected output 2" not found in "wrong_file_2.txt"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "wrong_file_2.txt",
						Pattern: "expected output 2",
					},
				},
			},
			{
				ScenarioName:  "test-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "this-pattern-does-not-exist-anywhere" not found in "some_file.txt"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "some_file.txt",
						Pattern: "this-pattern-does-not-exist-anywhere",
					},
				},
			},
		},
		// After corrections, only the uncorrectable failure remains
		secondCallFailures: []contract.ContractFailure{
			{
				ScenarioName:  "test-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "this-pattern-does-not-exist-anywhere" not found in "some_file.txt"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "some_file.txt",
						Pattern: "this-pattern-does-not-exist-anywhere",
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

	// Verify: FailureContext contains the uncorrectable failure
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) == 0 {
		t.Fatal("expected at least one uncorrectable failure in FailureContext")
	}

	// Verify: only the uncorrectable failure is in the context (from the second evaluation)
	if len(action.Context.Failures) != 1 {
		t.Errorf("expected 1 uncorrectable failure in context, got %d", len(action.Context.Failures))
	}

	// Verify: two contract_corrected events were emitted (for the correctable failures)
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var correctedEvents []*runstore.ContractCorrectedEvent
	for _, ev := range events {
		if typedEv, ok := ev.(*runstore.ContractCorrectedEvent); ok {
			correctedEvents = append(correctedEvents, typedEv)
		}
	}

	if len(correctedEvents) != 2 {
		t.Fatalf("expected 2 contract_corrected events, got %d", len(correctedEvents))
	}

	// Verify: first correction
	if correctedEvents[0].OldPath != "wrong_file_1.txt" {
		t.Errorf("expected old path 'wrong_file_1.txt', got %q", correctedEvents[0].OldPath)
	}
	if correctedEvents[0].NewPath != "correct_file_1.txt" {
		t.Errorf("expected new path 'correct_file_1.txt', got %q", correctedEvents[0].NewPath)
	}
	if correctedEvents[0].Pattern != "expected output 1" {
		t.Errorf("expected pattern 'expected output 1', got %q", correctedEvents[0].Pattern)
	}

	// Verify: second correction
	if correctedEvents[1].OldPath != "wrong_file_2.txt" {
		t.Errorf("expected old path 'wrong_file_2.txt', got %q", correctedEvents[1].OldPath)
	}
	if correctedEvents[1].NewPath != "correct_file_2.txt" {
		t.Errorf("expected new path 'correct_file_2.txt', got %q", correctedEvents[1].NewPath)
	}
	if correctedEvents[1].Pattern != "expected output 2" {
		t.Errorf("expected pattern 'expected output 2', got %q", correctedEvents[1].Pattern)
	}

	// Verify: FinalValidationPassed was NOT set (due to uncorrectable failure)
	if rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be false due to uncorrectable failure")
	}

	// Verify: evaluator was called twice (initial evaluation + re-evaluation after corrections)
	if fakeEval.callCount != 2 {
		t.Errorf("expected evaluator to be called exactly twice, got %d", fakeEval.callCount)
	}
}
