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

// fakeContractEvaluatorEmptySpecNoop simulates a contract evaluator where
// the spec text is empty, so the guard should be a no-op and correction
// should proceed normally.
type fakeContractEvaluatorEmptySpecNoop struct {
	callCount int
}

func (f *fakeContractEvaluatorEmptySpecNoop) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	// On second call (after correction), should succeed because sibling file has the pattern
	if f.callCount == 2 {
		return []contract.ContractFailure{}, nil
	}
	// On first call, return failure
	return []contract.ContractFailure{
		{
			ScenarioName:  "noop-empty-spec-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "EmptySpecTest" not found in "original_test.go"`,
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "original_test.go",
					Pattern: "EmptySpecTest",
				},
			},
		},
	}, nil
}

// TestScenario_ContractCorrectionGuardNoopWhenSpecTextEmpty verifies that when a spec's
// SpecText is empty, the contract correction guard is a no-op, allowing correction to proceed
// normally (backward compatible). The pattern is found in a sibling file, the correction is
// accepted, a contract_corrected event is emitted, and the contract is updated to point to
// the sibling file.
// Test fixture branch: feature/beta-branch
func TestScenario_ContractCorrectionGuardNoopWhenSpecTextEmpty(t *testing.T) {
	dir := t.TempDir()

	// Create a healthy worktree
	worktreePath := filepath.Join(dir, ".gromit-next", "worktrees", "wt-003")
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
	// - original_test.go: missing the pattern (original file, contract points here initially)
	// - sibling_test.go: contains the pattern (sibling file, correction target)
	pattern := "EmptySpecTest"
	if err := os.WriteFile(filepath.Join(worktreePath, "original_test.go"), []byte("package main\n// no pattern here\n"), 0o644); err != nil {
		t.Fatalf("create original_test.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "sibling_test.go"), []byte("package main\n// "+pattern+"\n"), 0o644); err != nil {
		t.Fatalf("create sibling_test.go: %v", err)
	}

	// Create scenario contract YAML pointing to original_test.go
	contractYAML := `scenarios:
- name: noop-empty-spec-scenario
  assertions:
  - file_contains:
      path: original_test.go
      pattern: EmptySpecTest
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// SpecText is empty — the guard should be a no-op and correction should proceed
	specText := ""

	// Create fake evaluator
	fakeEval := &fakeContractEvaluatorEmptySpecNoop{}

	// Create event log to capture emitted events
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// Create fake validator that always passes
	fakeValidator := &fakeValidator{
		result: validator.FinalResult{Pass: true},
	}

	// Create validate stage with empty spec text so guard is noop
	stage := NewValidateStage(fakeValidator, ValidateStageConfig{
		WorkDir:          worktreePath,
		RepoDir:          dir,
		EvidenceDir:      evidenceDir,
		SearchExtensions: []string{".go"},
		SpecText:         specText, // EMPTY - guard should be noop
	}, eventLog, fakeEval, &validateScenarioFakeGitOps{})

	// Run validation
	rs := runstore.NewRunState("spec-004h", "proj-001")
	rs.WorktreePath = worktreePath
	rs.SpecID = "spec-004h"
	rs.RunID = "run-003"
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify: should continue (because contract passes after correction) rather than replan
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify: contract_corrected event was emitted (guard was noop, so correction proceeded)
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var foundCorrectedEvent bool
	var foundRejectedEvent bool
	for _, ev := range events {
		if typedEv, ok := ev.(*runstore.ContractCorrectedEvent); ok {
			foundCorrectedEvent = true
			if typedEv.ScenarioName != "noop-empty-spec-scenario" {
				t.Errorf("expected scenario name 'noop-empty-spec-scenario', got %q", typedEv.ScenarioName)
			}
			if typedEv.NewPath != "sibling_test.go" {
				t.Errorf("expected new path 'sibling_test.go', got %q", typedEv.NewPath)
			}
			if typedEv.OldPath != "original_test.go" {
				t.Errorf("expected old path 'original_test.go', got %q", typedEv.OldPath)
			}
		}
		if _, ok := ev.(*runstore.ContractCorrectionRejectedEvent); ok {
			foundRejectedEvent = true
		}
	}

	if !foundCorrectedEvent {
		t.Fatal("expected contract_corrected event to be emitted (guard should be noop)")
	}

	if foundRejectedEvent {
		t.Fatal("expected NO contract_correction_rejected event (guard should be noop with empty spec)")
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
	if !strings.Contains(string(contractContent), "sibling_test.go") {
		t.Fatal("expected contract to be corrected to sibling_test.go")
	}
	if strings.Contains(string(contractContent), "original_test.go") {
		t.Fatal("contract should be corrected away from original_test.go")
	}
}
