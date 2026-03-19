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

// fakeContractEvaluatorFileCorrection returns failures on first call, success on second call.
// This simulates a contract evaluator where the pattern is in a sibling file and can be corrected.
type fakeContractEvaluatorFileCorrection struct {
	callCount int
	failures  []contract.ContractFailure
}

func (f *fakeContractEvaluatorFileCorrection) Evaluate(ctx context.Context, contract *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	if f.callCount == 1 {
		// First call: return failures
		return f.failures, nil
	}
	// Second call: return success (no failures)
	return nil, nil
}

// TestScenario_FileContainsAssertionCorrectedToSiblingFile verifies that when a file_contains
// assertion points to the wrong file (exec_test.go) but a sibling file (spec_test.go) contains
// the pattern, the contract is corrected to point to the sibling file, a contract_corrected
// event is emitted, and the second evaluation passes with no replan.
// Test fixture branch: feature/beta-branch
func TestScenario_FileContainsAssertionCorrectedToSiblingFile(t *testing.T) {
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
	// - exec_test.go: missing the pattern (wrong file)
	// - spec_test.go: contains the pattern (correct file to be corrected to)
	pattern := "test assertion pass"
	if err := os.WriteFile(filepath.Join(worktreePath, "exec_test.go"), []byte("package main\n// no pattern here\n"), 0o644); err != nil {
		t.Fatalf("create exec_test.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "spec_test.go"), []byte("package main\n// "+pattern+"\n"), 0o644); err != nil {
		t.Fatalf("create spec_test.go: %v", err)
	}

	// Create scenario contract YAML pointing to exec_test.go (wrong file)
	contractYAML := `scenarios:
- name: file-correction-scenario
  assertions:
  - file_contains:
      path: exec_test.go
      pattern: test assertion pass
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Create fake evaluator that fails on first call, passes on second
	fakeEval := &fakeContractEvaluatorFileCorrection{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "file-correction-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "test assertion pass" not found in "exec_test.go"`,
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

	// Verify: should continue (no replan) after correction
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
			if typedEv.ScenarioName != "file-correction-scenario" {
				t.Errorf("expected scenario name 'file-correction-scenario', got %q", typedEv.ScenarioName)
			}
			if typedEv.OldPath != "exec_test.go" {
				t.Errorf("expected old path 'exec_test.go', got %q", typedEv.OldPath)
			}
			if typedEv.NewPath != "spec_test.go" {
				t.Errorf("expected new path 'spec_test.go', got %q", typedEv.NewPath)
			}
			if typedEv.Pattern != "test assertion pass" {
				t.Errorf("expected pattern 'test assertion pass', got %q", typedEv.Pattern)
			}
		}
	}

	if !foundCorrectedEvent {
		t.Fatal("expected contract_corrected event to be emitted")
	}

	// Verify: evaluator was called twice (first failure, then success after correction)
	if fakeEval.callCount != 2 {
		t.Errorf("expected evaluator to be called twice, got %d", fakeEval.callCount)
	}

	// Verify: final validation passed
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}
}
