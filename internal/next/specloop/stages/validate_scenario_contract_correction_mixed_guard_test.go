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

// fakeContractEvaluatorMixedGuard simulates a contract evaluator where
// the same scenario has two assertions, one for a guarded file and one unguarded.
// It inspects the contract parameter to determine which assertions are present,
// and returns failures accordingly.
type fakeContractEvaluatorMixedGuard struct{}

func (f *fakeContractEvaluatorMixedGuard) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	// Check if the contract still has the unguarded.go assertion.
	// If yes, return both failures (initial evaluation).
	// If the contract has been updated to corrected_unguarded.go, return only the guarded failure.
	hasUnguardedAssertion := false
	for _, scenario := range c.Scenarios {
		if scenario.Name == "mixed-guard-scenario" {
			for _, assertion := range scenario.Assertions {
				if assertion.FileContains != nil && assertion.FileContains.Path == "unguarded.go" {
					hasUnguardedAssertion = true
					break
				}
			}
			break
		}
	}

	if hasUnguardedAssertion {
		// Contract still references unguarded.go: return both failures
		return []contract.ContractFailure{
			{
				ScenarioName:  "mixed-guard-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "GuardedPattern" not found in "guarded.go"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "guarded.go",
						Pattern: "GuardedPattern",
					},
				},
			},
			{
				ScenarioName:  "mixed-guard-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "UnguardedPattern" not found in "unguarded.go"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "unguarded.go",
						Pattern: "UnguardedPattern",
					},
				},
			},
		}, nil
	}

	// Contract has been updated to corrected_unguarded.go: return only guarded.go failure
	return []contract.ContractFailure{
		{
			ScenarioName:  "mixed-guard-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "GuardedPattern" not found in "guarded.go"`,
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "guarded.go",
					Pattern: "GuardedPattern",
				},
			},
		},
	}, nil
}

// TestScenario_ContractCorrectionMixedGuard verifies that when a scenario has two
// file_contains assertions, one guarded (AC mentions the file) and one unguarded
// (AC does not mention the file), the correction system correctly:
// - Rejects the guarded assertion (emits contract_correction_rejected)
// - Accepts and applies the unguarded assertion (emits contract_corrected)
// - Keeps the action as ReplanFrom because the rejected failure remains
// - Emits both contract_correction_rejected and contract_corrected events
func TestScenario_ContractCorrectionMixedGuard(t *testing.T) {
	dir := t.TempDir()

	// Create a healthy worktree
	worktreePath := filepath.Join(dir, ".gromit-next", "worktrees", "wt-mixed")
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

	// Create files:
	// - guarded.go: missing GuardedPattern (cannot be corrected, AC mentions it)
	// - corrected_guarded.go: has GuardedPattern (would be used for correction, but will be rejected)
	// - unguarded.go: missing UnguardedPattern (can be corrected, AC does NOT mention it)
	// - corrected_unguarded.go: has UnguardedPattern (will be used for correction)
	guarded := filepath.Join(worktreePath, "guarded.go")
	if err := os.WriteFile(guarded, []byte("package main\n// no pattern here\n"), 0o644); err != nil {
		t.Fatalf("create guarded.go: %v", err)
	}

	correctedGuarded := filepath.Join(worktreePath, "corrected_guarded.go")
	if err := os.WriteFile(correctedGuarded, []byte("package main\n// GuardedPattern\n"), 0o644); err != nil {
		t.Fatalf("create corrected_guarded.go: %v", err)
	}

	unguarded := filepath.Join(worktreePath, "unguarded.go")
	if err := os.WriteFile(unguarded, []byte("package main\n// no pattern here\n"), 0o644); err != nil {
		t.Fatalf("create unguarded.go: %v", err)
	}

	correctedUnguarded := filepath.Join(worktreePath, "corrected_unguarded.go")
	if err := os.WriteFile(correctedUnguarded, []byte("package main\n// UnguardedPattern\n"), 0o644); err != nil {
		t.Fatalf("create corrected_unguarded.go: %v", err)
	}

	// Create scenario contract YAML with two assertions for the same scenario
	contractYAML := `scenarios:
- name: mixed-guard-scenario
  assertions:
  - file_contains:
      path: guarded.go
      pattern: GuardedPattern
  - file_contains:
      path: unguarded.go
      pattern: UnguardedPattern
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Create a spec with AC text that mentions guarded.go (guard) but NOT unguarded.go
	specText := `# Spec Mixed Guard
## Vision
Testing mixed guard behavior with two assertions...

## Acceptance Criteria

1. guarded.go must have GuardedPattern
2. Ensure other validation passes

## Scenarios
...
`

	// Create fake evaluator that returns both failures
	fakeEval := &fakeContractEvaluatorMixedGuard{}

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
	rs := runstore.NewRunState("spec-mixed", "proj-mixed")
	rs.WorktreePath = worktreePath
	rs.SpecID = "spec-mixed"
	rs.RunID = "run-mixed"
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify: should replan (because guarded failure remains) rather than continue
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}

	// Verify: both contract_correction_rejected and contract_corrected events were emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var rejectedCount int
	var correctedCount int
	var rejectedReason string

	for _, ev := range events {
		if typedEv, ok := ev.(*runstore.ContractCorrectionRejectedEvent); ok {
			rejectedCount++
			if typedEv.ScenarioName != "mixed-guard-scenario" {
				t.Errorf("rejected event: expected scenario name 'mixed-guard-scenario', got %q", typedEv.ScenarioName)
			}
			if typedEv.OldPath != "guarded.go" {
				t.Errorf("rejected event: expected OldPath %q, got %q", "guarded.go", typedEv.OldPath)
			}
			if typedEv.CandidatePath != "corrected_guarded.go" {
				t.Errorf("rejected event: expected CandidatePath %q, got %q", "corrected_guarded.go", typedEv.CandidatePath)
			}
			if typedEv.Reason == "" {
				t.Errorf("rejected event: expected non-empty reason")
			}
			rejectedReason = typedEv.Reason
			// Verify reason mentions the guarded file (should contain the guard from AC)
			if !strings.Contains(typedEv.Reason, "guarded.go") {
				t.Errorf("rejected event: expected reason to mention 'guarded.go', got %q", typedEv.Reason)
			}
		}
		if typedEv, ok := ev.(*runstore.ContractCorrectedEvent); ok {
			correctedCount++
			if typedEv.ScenarioName != "mixed-guard-scenario" {
				t.Errorf("corrected event: expected scenario name 'mixed-guard-scenario', got %q", typedEv.ScenarioName)
			}
			if typedEv.OldPath != "unguarded.go" {
				t.Errorf("corrected event: expected OldPath %q, got %q", "unguarded.go", typedEv.OldPath)
			}
			if typedEv.NewPath != "corrected_unguarded.go" {
				t.Errorf("corrected event: expected NewPath %q, got %q", "corrected_unguarded.go", typedEv.NewPath)
			}
		}
	}

	if rejectedCount != 1 {
		t.Fatalf("expected exactly 1 contract_correction_rejected event for guarded.go, got %d", rejectedCount)
	}
	if correctedCount != 1 {
		t.Fatalf("expected exactly 1 contract_corrected event for unguarded.go, got %d", correctedCount)
	}

	t.Logf("Rejected reason: %s", rejectedReason)

	// Verify: contract file was updated with the corrected path for unguarded.go
	contractContent, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if !strings.Contains(string(contractContent), "corrected_unguarded.go") {
		t.Fatal("expected contract to be corrected to corrected_unguarded.go")
	}
	// Verify: contract still points to guarded.go (rejection means no correction)
	if !strings.Contains(string(contractContent), "path: guarded.go") {
		t.Fatal("expected contract to still point to path: guarded.go (rejection means no correction)")
	}
	// Verify: contract does not point to the wrong sibling
	if strings.Contains(string(contractContent), "path: corrected_guarded.go") {
		t.Fatal("expected contract to not suggest path: corrected_guarded.go")
	}
}
