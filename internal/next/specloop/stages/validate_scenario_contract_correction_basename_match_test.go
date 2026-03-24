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

// fakeContractEvaluatorBasenameMatch simulates a contract evaluator where
// the contract failure contains a full relative path, but the AC only mentions the basename,
// and the correction should be rejected because specACMentionsPath returns true (basename match).
type fakeContractEvaluatorBasenameMatch struct {
	callCount int
}

func (f *fakeContractEvaluatorBasenameMatch) Evaluate(ctx context.Context, c *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
	f.callCount++
	// Always return failure with a full relative path
	return []contract.ContractFailure{
		{
			ScenarioName:  "basename-match-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "TestPattern" not found in "internal/next/specloop/stages/write_contracts_test.go"`,
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "internal/next/specloop/stages/write_contracts_test.go",
					Pattern: "TestPattern",
				},
			},
		},
	}, nil
}

// TestScenario_ContractCorrectionRejectedViaBasenameMatchOnFullPath verifies that when a spec's
// acceptance criteria text contains the basename of a full relative path (e.g., "write_contracts_test.go"),
// the contract correction is rejected even though the contract failure path contains a full relative path
// prefix (e.g., "internal/next/specloop/stages/write_contracts_test.go").
// This validates that specACMentionsPath correctly matches basenames regardless of the path prefix.
// Test fixture branch: feature/beta-branch
func TestScenario_ContractCorrectionRejectedViaBasenameMatchOnFullPath(t *testing.T) {
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

	// Create the file structure that matches the full path in the contract failure
	stagesDir := filepath.Join(worktreePath, "internal", "next", "specloop", "stages")
	if err := os.MkdirAll(stagesDir, 0o755); err != nil {
		t.Fatalf("create stages dir: %v", err)
	}

	// Create files:
	// - internal/next/specloop/stages/write_contracts_test.go: missing the pattern
	// - internal/next/specloop/stages/alternative_test.go: contains the pattern (would normally correct to this)
	pattern := "TestPattern"
	if err := os.WriteFile(filepath.Join(stagesDir, "write_contracts_test.go"), []byte("package stages\n// no pattern here\n"), 0o644); err != nil {
		t.Fatalf("create write_contracts_test.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stagesDir, "alternative_test.go"), []byte("package stages\n// "+pattern+"\n"), 0o644); err != nil {
		t.Fatalf("create alternative_test.go: %v", err)
	}

	// Create scenario contract YAML pointing to the relative path
	contractYAML := `scenarios:
- name: basename-match-scenario
  assertions:
  - file_contains:
      path: internal/next/specloop/stages/write_contracts_test.go
      pattern: TestPattern
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Create a spec with AC text that mentions the basename "write_contracts_test.go"
	// The AC does NOT mention the full path, just the basename
	specText := `# Spec 0004h

## Vision
Basename matching in contract correction...

## Acceptance Criteria

1. Ensure TestPattern is in write_contracts_test.go
2. The correction should be rejected when AC mentions the file basename
3. A contract_correction_rejected event should be emitted

## Scenarios
...
`

	// Create fake evaluator
	fakeEval := &fakeContractEvaluatorBasenameMatch{}

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
	rs.RunID = "run-003"
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
			if typedEv.ScenarioName != "basename-match-scenario" {
				t.Errorf("expected scenario name 'basename-match-scenario', got %q", typedEv.ScenarioName)
			}
			if typedEv.Reason == "" {
				t.Errorf("expected non-empty reason, got %q", typedEv.Reason)
			}
			// Verify reason mentions the spec AC guard (should contain the basename)
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

	// Verify: contract should still point to the full path (not corrected)
	contractContent, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if !strings.Contains(string(contractContent), "internal/next/specloop/stages/write_contracts_test.go") {
		t.Fatal("expected contract to still point to internal/next/specloop/stages/write_contracts_test.go (correction should be rejected)")
	}
	if strings.Contains(string(contractContent), "alternative_test.go") {
		t.Fatal("contract should not be corrected to alternative_test.go")
	}
}
