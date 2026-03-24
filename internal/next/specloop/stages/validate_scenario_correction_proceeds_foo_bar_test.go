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

// fakeContractEvaluatorFooBar simulates a contract evaluator where the
// original file (foo_test.go) lacks the pattern and the sibling (bar_test.go)
// contains it. The AC does not mention foo_test.go, so correction proceeds.
type fakeContractEvaluatorFooBar struct {
	callCount int
}

func (f *fakeContractEvaluatorFooBar) Evaluate(_ context.Context, _ *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
	f.callCount++
	if f.callCount >= 2 {
		return []contract.ContractFailure{}, nil
	}
	return []contract.ContractFailure{
		{
			ScenarioName:  "foo-bar-correction-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "SomeFeature" not found in "foo_test.go"`,
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "foo_test.go",
					Pattern: "SomeFeature",
				},
			},
		},
	}, nil
}

// TestScenario_ContractCorrectionProceedsWhenACDoesNotMentionFooTestGo verifies that
// when a spec's AC text does not mention foo_test.go, and a sibling file bar_test.go
// contains the pattern, attemptContractCorrection proceeds normally: the contract is
// updated to point at bar_test.go and a contract_corrected event is emitted.
func TestScenario_ContractCorrectionProceedsWhenACDoesNotMentionFooTestGo(t *testing.T) {
	dir := t.TempDir()

	// Seed: create a healthy worktree with foo_test.go (missing pattern) and
	// bar_test.go (has pattern).
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

	// foo_test.go: does NOT contain the pattern.
	if err := os.WriteFile(filepath.Join(worktreePath, "foo_test.go"), []byte("package main\n// unrelated content\n"), 0o644); err != nil {
		t.Fatalf("write foo_test.go: %v", err)
	}
	// bar_test.go: DOES contain the pattern.
	if err := os.WriteFile(filepath.Join(worktreePath, "bar_test.go"), []byte("package main\n// SomeFeature\n"), 0o644); err != nil {
		t.Fatalf("write bar_test.go: %v", err)
	}

	// Seed: evidence dir with contract pointing at foo_test.go.
	evidenceDir := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}
	contractYAML := `scenarios:
- name: foo-bar-correction-scenario
  assertions:
  - file_contains:
      path: foo_test.go
      pattern: SomeFeature
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract: %v", err)
	}

	// Spec AC mentions only the feature name, NOT "foo_test.go".
	specText := `# Spec 0004h

## Vision
Implement SomeFeature.

## Acceptance Criteria

1. SomeFeature must be implemented and tested
2. Tests must cover the happy path

## Scenarios
...
`

	fakeEval := &fakeContractEvaluatorFooBar{}
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	fakeVal := &fakeValidator{result: validator.FinalResult{Pass: true}}

	// Invoke: run ValidateStage with the spec text that does not mention foo_test.go.
	stage := NewValidateStage(fakeVal, ValidateStageConfig{
		WorkDir:          worktreePath,
		RepoDir:          dir,
		EvidenceDir:      evidenceDir,
		SearchExtensions: []string{".go"},
		SpecText:         specText,
	}, eventLog, fakeEval, &validateScenarioFakeGitOps{})

	rs := runstore.NewRunState("spec-004h", "proj-001")
	rs.WorktreePath = worktreePath
	rs.SpecID = "spec-004h"
	rs.RunID = "run-foo-bar"
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("stage.Run: %v", err)
	}

	// Assert: correction succeeded so the final result should be Continue.
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Assert: a contract_corrected event was emitted for the right scenario.
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var correctedEv *runstore.ContractCorrectedEvent
	for _, ev := range events {
		if ce, ok := ev.(*runstore.ContractCorrectedEvent); ok {
			correctedEv = ce
			break
		}
	}
	if correctedEv == nil {
		t.Fatal("expected contract_corrected event, none found")
	}
	if correctedEv.ScenarioName != "foo-bar-correction-scenario" {
		t.Errorf("scenario name: want %q, got %q", "foo-bar-correction-scenario", correctedEv.ScenarioName)
	}
	if correctedEv.OldPath != "foo_test.go" {
		t.Errorf("old path: want %q, got %q", "foo_test.go", correctedEv.OldPath)
	}
	if correctedEv.NewPath != "bar_test.go" {
		t.Errorf("new path: want %q, got %q", "bar_test.go", correctedEv.NewPath)
	}
	if correctedEv.Pattern != "SomeFeature" {
		t.Errorf("pattern: want %q, got %q", "SomeFeature", correctedEv.Pattern)
	}

	// Assert: no contract_correction_rejected event was emitted.
	for _, ev := range events {
		if _, ok := ev.(*runstore.ContractCorrectionRejectedEvent); ok {
			t.Fatal("unexpected contract_correction_rejected event")
		}
	}

	// Assert: evaluator called twice (initial check + re-evaluation after correction).
	if fakeEval.callCount != 2 {
		t.Errorf("evaluator call count: want 2, got %d", fakeEval.callCount)
	}

	// Assert: contract file on disk now points at bar_test.go, not foo_test.go.
	contractContent, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if !strings.Contains(string(contractContent), "bar_test.go") {
		t.Error("contract should point to bar_test.go after correction")
	}
	if strings.Contains(string(contractContent), "foo_test.go") {
		t.Error("contract should no longer point to foo_test.go")
	}
}
