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

// fakeContractEvaluatorFileCorrectionMulti returns failures on first call (all correctable),
// and success on second call. This simulates a contract evaluator where multiple
// assertions are corrected in a single pass.
type fakeContractEvaluatorFileCorrectionMulti struct {
	callCount int
	failures  []contract.ContractFailure
}

func (f *fakeContractEvaluatorFileCorrectionMulti) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	if f.callCount == 1 {
		// First call: return all 3 failures
		return f.failures, nil
	}
	// Second call: return success (no failures)
	return nil, nil
}

// TestScenario_MultipleAssertionsCorrectedInOnePass verifies that when a scenario contract
// has multiple file_contains assertions all pointing to the wrong file (exec_test.go),
// but the patterns exist in a sibling file (spec_test.go), all assertions are corrected
// in a single pass, contract_corrected events are emitted for each correction, and the
// stage returns Continue with no replan.
func TestScenario_MultipleAssertionsCorrectedInOnePass(t *testing.T) {
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
	// - exec_test.go: missing all patterns (wrong file)
	// - spec_test.go: contains all patterns (correct file to be corrected to)
	pattern1 := "assertion one passes"
	pattern2 := "assertion two passes"
	pattern3 := "assertion three passes"
	if err := os.WriteFile(filepath.Join(worktreePath, "exec_test.go"), []byte("package main\n// no patterns here\n"), 0o644); err != nil {
		t.Fatalf("create exec_test.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "spec_test.go"), []byte("package main\n// "+pattern1+"\n// "+pattern2+"\n// "+pattern3+"\n"), 0o644); err != nil {
		t.Fatalf("create spec_test.go: %v", err)
	}

	// Create scenario contract YAML pointing to exec_test.go (wrong file) with 3 assertions
	contractYAML := `scenarios:
- name: multi-correction-scenario
  assertions:
  - file_contains:
      path: exec_test.go
      pattern: assertion one passes
  - file_contains:
      path: exec_test.go
      pattern: assertion two passes
  - file_contains:
      path: exec_test.go
      pattern: assertion three passes
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Create fake evaluator that fails on first call (3 failures), passes on second
	fakeEval := &fakeContractEvaluatorFileCorrectionMulti{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "multi-correction-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "assertion one passes" not found in "exec_test.go"`,
			},
			{
				ScenarioName:  "multi-correction-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "assertion two passes" not found in "exec_test.go"`,
			},
			{
				ScenarioName:  "multi-correction-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "assertion three passes" not found in "exec_test.go"`,
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

	// Verify: should continue (no replan) after all corrections
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify: contract_corrected events were emitted for all 3 corrections
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

	if len(correctedEvents) != 3 {
		t.Fatalf("expected 3 contract_corrected events, got %d", len(correctedEvents))
	}

	// Verify all corrections
	expectedPatterns := []string{"assertion one passes", "assertion two passes", "assertion three passes"}
	for i, corrEv := range correctedEvents {
		if corrEv.ScenarioName != "multi-correction-scenario" {
			t.Errorf("event %d: expected scenario name 'multi-correction-scenario', got %q", i, corrEv.ScenarioName)
		}
		if corrEv.OldPath != "exec_test.go" {
			t.Errorf("event %d: expected old path 'exec_test.go', got %q", i, corrEv.OldPath)
		}
		if corrEv.NewPath != "spec_test.go" {
			t.Errorf("event %d: expected new path 'spec_test.go', got %q", i, corrEv.NewPath)
		}
		if corrEv.Pattern != expectedPatterns[i] {
			t.Errorf("event %d: expected pattern %q, got %q", i, expectedPatterns[i], corrEv.Pattern)
		}
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
