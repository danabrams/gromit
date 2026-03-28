package stages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/validator"
)

// TestUnit_ContractCorrectionNilEventLog verifies that attemptContractCorrection
// handles a nil eventLog gracefully when the spec AC mentions the original path.
// The failure should remain in the remaining slice and no panic should occur.
func TestUnit_ContractCorrectionNilEventLog(t *testing.T) {
	dir := t.TempDir()

	// Setup: create a minimal worktree
	worktreePath := filepath.Join(dir, "worktree")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// Create original file without the pattern
	if err := os.WriteFile(filepath.Join(worktreePath, "original.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write original.go: %v", err)
	}

	// Create sibling file with the pattern (so a correction would be possible)
	if err := os.WriteFile(filepath.Join(worktreePath, "sibling.go"), []byte("package main\n// TestPattern\n"), 0o644); err != nil {
		t.Fatalf("write sibling.go: %v", err)
	}

	// Setup: spec text that mentions original.go in its AC section
	specText := `# Test Spec

## Vision
Test feature.

## Acceptance Criteria

1. original.go must contain TestPattern
2. Other requirements

## Scenarios
...
`

	// Setup: contract with a failure for original.go
	sc := &contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "test-scenario",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "original.go",
							Pattern: "TestPattern",
						},
					},
				},
			},
		},
	}

	failures := []contract.ContractFailure{
		{
			ScenarioName:  "test-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "TestPattern" not found in "original.go"`,
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "original.go",
					Pattern: "TestPattern",
				},
			},
		},
	}

	// Setup: ValidateStage with nil eventLog
	stage := NewValidateStage(
		&fakeValidator{result: validator.FinalResult{Pass: true}},
		ValidateStageConfig{
			WorkDir:          worktreePath,
			RepoDir:          dir,
			SearchExtensions: []string{".go"},
			SpecText:         specText,
		},
		nil, // eventLog is nil
		nil, // contractEvaluator is nil
		nil, // gitOps is nil
	)

	contractPath := filepath.Join(dir, "scenario-contracts.yaml")

	// Act: call attemptContractCorrection with nil eventLog
	corrected, remaining := stage.attemptContractCorrection(sc, failures, worktreePath, contractPath, nil)

	// Assert: no correction should occur (AC mentions original.go)
	if len(corrected) != 0 {
		t.Errorf("expected no corrections, got %d", len(corrected))
	}

	// Assert: failure should remain
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining failure, got %d", len(remaining))
	}

	if len(remaining) > 0 {
		if remaining[0].ScenarioName != "test-scenario" {
			t.Errorf("scenario name: want %q, got %q", "test-scenario", remaining[0].ScenarioName)
		}
		if remaining[0].Assertion.FileContains.Path != "original.go" {
			t.Errorf("path: want %q, got %q", "original.go", remaining[0].Assertion.FileContains.Path)
		}
	}

}

// TestUnit_ContractCorrectionNilEventLog_GuardDoesNotFire verifies that when
// the spec AC does NOT mention the original path, attemptContractCorrection
// accepts the correction even with a nil eventLog (no panic, correction applied).
func TestUnit_ContractCorrectionNilEventLog_GuardDoesNotFire(t *testing.T) {
	dir := t.TempDir()

	// Setup: create a minimal worktree
	worktreePath := filepath.Join(dir, "worktree")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// Create original file without the pattern
	if err := os.WriteFile(filepath.Join(worktreePath, "original.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write original.go: %v", err)
	}

	// Create sibling file with the pattern (so a correction is possible)
	if err := os.WriteFile(filepath.Join(worktreePath, "sibling.go"), []byte("package main\n// TestPattern\n"), 0o644); err != nil {
		t.Fatalf("write sibling.go: %v", err)
	}

	// Setup: spec text that does NOT mention original.go in its AC section
	specText := `# Test Spec

## Vision
Test feature.

## Acceptance Criteria

1. Some unrelated requirement
2. Other requirements

## Scenarios
...
`

	// Setup: contract with a failure for original.go
	sc := &contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{
			{
				Name: "test-scenario",
				Assertions: []contract.ContractAssertion{
					{
						FileContains: &contract.FileContainsAssertion{
							Path:    "original.go",
							Pattern: "TestPattern",
						},
					},
				},
			},
		},
	}

	failures := []contract.ContractFailure{
		{
			ScenarioName:  "test-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "TestPattern" not found in "original.go"`,
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "original.go",
					Pattern: "TestPattern",
				},
			},
		},
	}

	contractPath := filepath.Join(dir, "scenario-contracts.yaml")

	// Setup: ValidateStage with nil eventLog
	stage := NewValidateStage(
		&fakeValidator{result: validator.FinalResult{Pass: true}},
		ValidateStageConfig{
			WorkDir:          worktreePath,
			RepoDir:          dir,
			SearchExtensions: []string{".go"},
			SpecText:         specText,
		},
		nil, // eventLog is nil
		nil, // contractEvaluator is nil
		nil, // gitOps is nil
	)

	// Act: call attemptContractCorrection — guard should NOT fire, correction accepted
	corrected, remaining := stage.attemptContractCorrection(sc, failures, worktreePath, contractPath, nil)

	// Assert: correction was accepted (corrected count > 0)
	if len(corrected) == 0 {
		t.Errorf("expected at least one correction, got 0 (guard must not fire when AC does not name the path)")
	}

	// Assert: no remaining failures
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining failures, got %d", len(remaining))
	}
}
