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

// fakeContractEvaluatorPatternNotFound returns a failure indicating the pattern was not found.
// This simulates a contract evaluator where the pattern doesn't exist in any sibling files.
// It tracks call count to verify the evaluator is called exactly once (no correction attempt).
type fakeContractEvaluatorPatternNotFound struct {
	callCount int
}

func (f *fakeContractEvaluatorPatternNotFound) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	return []contract.ContractFailure{
		{
			ScenarioName:  "pattern-not-found-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "missing-pattern" not found in "file1.go"`,
		},
	}, nil
}

// TestScenario_PatternNotFoundAnywhere verifies that when a file_contains assertion
// has a pattern that is not found anywhere in the worktree (not in the indicated file
// nor in any sibling files), the failure propagates to ReplanFrom, no correction is
// attempted, no contract_corrected event is emitted, and the contract YAML remains unchanged.
// Test fixture branch: feature/foo
func TestScenario_PatternNotFoundAnywhere(t *testing.T) {
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

	// Create sibling files in worktree with different content, but NOT the pattern we'll look for
	if err := os.WriteFile(filepath.Join(worktreePath, "file1.go"), []byte("package main\n// file1 content\n"), 0o644); err != nil {
		t.Fatalf("create file1.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "file2.go"), []byte("package main\n// file2 content\n"), 0o644); err != nil {
		t.Fatalf("create file2.go: %v", err)
	}

	// Create scenario contract that requires a pattern that doesn't exist anywhere
	contractYAML := `scenarios:
- name: pattern-not-found-scenario
  assertions:
  - file_contains:
      path: file1.go
      pattern: missing-pattern
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Keep original contract content for later verification
	originalContractContent := contractYAML

	// Create fake evaluator that returns failure (pattern not found anywhere)
	fakeEval := &fakeContractEvaluatorPatternNotFound{}

	// Create event log to capture emitted events
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// Create fake validator that always passes (validation should not be the issue)
	fakeValidator := &fakeValidator{
		result: validator.FinalResult{Pass: true},
	}

	// Create validate stage with evaluator and event log
	stage := NewValidateStage(fakeValidator, ValidateStageConfig{
		WorkDir:          worktreePath,
		RepoDir:          dir,
		EvidenceDir:      evidenceDir,
		SearchExtensions: []string{".go"},
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

	// Verify: should return ReplanFrom (not Continue) due to contract failure
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}

	// Verify: FailureContext contains the contract failure
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) == 0 {
		t.Fatal("expected at least one failure in FailureContext")
	}

	// Verify: no contract_corrected event was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	for _, ev := range events {
		if _, ok := ev.(*runstore.ContractCorrectedEvent); ok {
			t.Fatal("expected NO contract_corrected event to be emitted when pattern is not found anywhere")
		}
	}

	// Verify: contract YAML remains unchanged
	finalContractContent, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract file: %v", err)
	}
	if string(finalContractContent) != originalContractContent {
		t.Fatalf("expected contract YAML to remain unchanged, but it was modified")
	}

	// Verify: FinalValidationPassed was NOT set (due to contract failure)
	if rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be false due to contract failure")
	}

	// Verify: evaluator was called exactly once (no correction attempt)
	if fakeEval.callCount != 1 {
		t.Errorf("expected evaluator to be called exactly once, got %d (indicates correction attempt)", fakeEval.callCount)
	}
}
