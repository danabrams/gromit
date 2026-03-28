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

// fakeContractEvaluatorPlannedDeliverable simulates a contract evaluator where
// the failing file is in a task's ExpectedTouchedArea.
type fakeContractEvaluatorPlannedDeliverable struct {
	callCount int
}

func (f *fakeContractEvaluatorPlannedDeliverable) Evaluate(_ context.Context, _ *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
	f.callCount++
	return []contract.ContractFailure{
		{
			ScenarioName:  "planned-deliverable-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "NewFeatureFunc" not found in "internal/pkg/foo.go"`,
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "internal/pkg/foo.go",
					Pattern: "NewFeatureFunc",
				},
			},
		},
	}, nil
}

// newPlannedDeliverableWorktree creates a temp worktree with a sibling file containing the pattern.
// The original target file (internal/pkg/foo.go) is absent — simulating a failed create task.
func newPlannedDeliverableWorktree(t *testing.T, worktreePath, pattern string) {
	t.Helper()
	pkgDir := filepath.Join(worktreePath, "internal", "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("create pkg dir: %v", err)
	}
	// Sibling file that contains the pattern (would normally be the correction target)
	if err := os.WriteFile(filepath.Join(pkgDir, "bar.go"), []byte("package pkg\n// "+pattern+"\nfunc "+pattern+"() {}\n"), 0o644); err != nil {
		t.Fatalf("create bar.go: %v", err)
	}
	// foo.go is intentionally absent (the task failed to create it)
}

// TestAttemptContractCorrection_RejectsWhenPathIsPlannedDeliverable verifies that
// when a task has "internal/pkg/foo.go" in its ExpectedTouchedArea and the contract
// fails on that path, the correction is rejected even though a sibling (bar.go)
// contains the pattern. The failure must remain and cause ReplanFrom.
func TestAttemptContractCorrection_RejectsWhenPathIsPlannedDeliverable(t *testing.T) {
	dir := t.TempDir()

	worktreePath := filepath.Join(dir, "worktree")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}

	pattern := "NewFeatureFunc"
	newPlannedDeliverableWorktree(t, worktreePath, pattern)

	evidenceDir := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	contractYAML := `scenarios:
- name: planned-deliverable-scenario
  assertions:
  - file_contains:
      path: internal/pkg/foo.go
      pattern: NewFeatureFunc
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	fakeEval := &fakeContractEvaluatorPlannedDeliverable{}

	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	fakeVal := &fakeValidator{result: validator.FinalResult{Pass: true}}

	stage := NewValidateStage(fakeVal, ValidateStageConfig{
		WorkDir:          worktreePath,
		RepoDir:          dir,
		EvidenceDir:      evidenceDir,
		SearchExtensions: []string{".go"},
		SpecText:         "# Spec\n## Acceptance Criteria\n1. Implement NewFeatureFunc\n",
	}, eventLog, fakeEval, &validateScenarioFakeGitOps{})

	rs := runstore.NewRunState("spec-deliverable", "proj-001")
	rs.WorktreePath = worktreePath
	rs.Cycle = 1
	// Task that planned to create internal/pkg/foo.go but failed (files_changed: [])
	rs.Tasks = []runstore.Task{
		{
			TaskID:              "task-001",
			Objective:           "Create internal/pkg/foo.go with NewFeatureFunc",
			Status:              "failed",
			ExpectedTouchedArea: []string{"internal/pkg/foo.go"},
			FilesChanged:        []string{},
		},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Correction was rejected → contract still fails → ReplanFrom
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom (deliverable gap must surface), got %v", action.Kind)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var foundRejectedEvent bool
	for _, ev := range events {
		if typedEv, ok := ev.(*runstore.ContractCorrectionRejectedEvent); ok {
			foundRejectedEvent = true
			if typedEv.ScenarioName != "planned-deliverable-scenario" {
				t.Errorf("expected scenario 'planned-deliverable-scenario', got %q", typedEv.ScenarioName)
			}
			if typedEv.OldPath != "internal/pkg/foo.go" {
				t.Errorf("expected OldPath 'internal/pkg/foo.go', got %q", typedEv.OldPath)
			}
			if !strings.Contains(typedEv.Reason, "planned deliverable") {
				t.Errorf("expected reason to mention 'planned deliverable', got %q", typedEv.Reason)
			}
			if !strings.Contains(typedEv.Reason, "internal/pkg/foo.go") {
				t.Errorf("expected reason to mention 'internal/pkg/foo.go', got %q", typedEv.Reason)
			}
		}
	}

	if !foundRejectedEvent {
		t.Fatal("expected contract_correction_rejected event to be emitted")
	}

	// No contract_corrected event (correction was rejected)
	for _, ev := range events {
		if _, ok := ev.(*runstore.ContractCorrectedEvent); ok {
			t.Fatal("expected no contract_corrected event — correction must be rejected for planned deliverables")
		}
	}

	// Evaluator called exactly once (no re-evaluation after rejected correction)
	if fakeEval.callCount != 1 {
		t.Errorf("expected evaluator called once (no re-evaluation), got %d", fakeEval.callCount)
	}

	// Contract still points to foo.go (not corrected to bar.go)
	contractContent, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if !strings.Contains(string(contractContent), "foo.go") {
		t.Fatal("expected contract to still point to foo.go after rejected correction")
	}
	if strings.Contains(string(contractContent), "bar.go") {
		t.Fatal("contract must not be corrected to bar.go for a planned deliverable")
	}
}

// TestAttemptContractCorrection_AllowsWhenPathNotInAnyTask verifies that when no task
// covers the failing path, normal correction proceeds (existing behavior unchanged).
func TestAttemptContractCorrection_AllowsWhenPathNotInAnyTask(t *testing.T) {
	dir := t.TempDir()

	worktreePath := filepath.Join(dir, "worktree")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}

	// Sibling file bar.go has the pattern; foo.go is absent (not a planned deliverable)
	pkgDir := filepath.Join(worktreePath, "internal", "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("create pkg dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "bar.go"), []byte("package pkg\nfunc NewFeatureFunc() {}\n"), 0o644); err != nil {
		t.Fatalf("create bar.go: %v", err)
	}

	evidenceDir := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	contractYAML := `scenarios:
- name: no-task-covers-path-scenario
  assertions:
  - file_contains:
      path: internal/pkg/foo.go
      pattern: NewFeatureFunc
`
	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("create contract: %v", err)
	}

	// Evaluator: first call returns failure; second call (after correction) returns no failures.
	callCount := 0
	fakeEval := &fakeCallCountingEvaluatorAllowsNoTask{&callCount}

	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	fakeVal := &fakeValidator{result: validator.FinalResult{Pass: true}}

	stage := NewValidateStage(fakeVal, ValidateStageConfig{
		WorkDir:          worktreePath,
		RepoDir:          dir,
		EvidenceDir:      evidenceDir,
		SearchExtensions: []string{".go"},
		SpecText:         "# Spec\n## Acceptance Criteria\n1. Implement the feature\n",
	}, eventLog, fakeEval, &validateScenarioFakeGitOps{})

	rs := runstore.NewRunState("spec-no-task", "proj-001")
	rs.WorktreePath = worktreePath
	rs.Cycle = 1
	// Tasks exist but none cover internal/pkg/foo.go
	rs.Tasks = []runstore.Task{
		{
			TaskID:              "task-002",
			Objective:           "Update internal/pkg/other.go",
			Status:              "done",
			ExpectedTouchedArea: []string{"internal/pkg/other.go"},
		},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Correction proceeds → re-evaluation passes → Continue
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue after successful correction, got %v", action.Kind)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	// A contract_corrected event should be emitted
	var foundCorrectedEvent bool
	for _, ev := range events {
		if _, ok := ev.(*runstore.ContractCorrectedEvent); ok {
			foundCorrectedEvent = true
		}
	}
	if !foundCorrectedEvent {
		t.Fatal("expected contract_corrected event when path is not a planned deliverable")
	}

	// No rejection event
	for _, ev := range events {
		if typedEv, ok := ev.(*runstore.ContractCorrectionRejectedEvent); ok {
			t.Fatalf("expected no rejection event, got one with reason %q", typedEv.Reason)
		}
	}
}

// fakeCallCountingEvaluatorAllowsNoTask: first call fails, second call passes.
type fakeCallCountingEvaluatorAllowsNoTask struct {
	callCount *int
}

func (f *fakeCallCountingEvaluatorAllowsNoTask) Evaluate(_ context.Context, _ *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
	*f.callCount++
	if *f.callCount == 1 {
		return []contract.ContractFailure{
			{
				ScenarioName:  "no-task-covers-path-scenario",
				AssertionType: "file_contains",
				Details:       `pattern "NewFeatureFunc" not found in "internal/pkg/foo.go"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "internal/pkg/foo.go",
						Pattern: "NewFeatureFunc",
					},
				},
			},
		}, nil
	}
	// After correction: no failures
	return nil, nil
}

// TestAttemptContractCorrection_RejectsRegardlessOfTaskStatus verifies that the guard
// fires for tasks in any non-pending status — a failed, done, running, or needs_split
// task with a path in ExpectedTouchedArea still prevents correction.
//
// Note: "pending" is excluded because deferContractFailures (which runs before
// attemptContractCorrection) already removes failures for pending-task paths from
// the slice that reaches correction. The guard is thus only reachable for non-pending
// statuses.
func TestAttemptContractCorrection_RejectsRegardlessOfTaskStatus(t *testing.T) {
	for _, status := range []string{"running", "done", "failed", "needs_split"} {
		status := status
		t.Run("status="+status, func(t *testing.T) {
			dir := t.TempDir()

			worktreePath := filepath.Join(dir, "worktree")
			if err := os.MkdirAll(worktreePath, 0o755); err != nil {
				t.Fatalf("create worktree: %v", err)
			}
			if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
				t.Fatalf("create .git: %v", err)
			}
			if err := os.WriteFile(filepath.Join(worktreePath, "go.mod"), []byte("module test\n"), 0o644); err != nil {
				t.Fatalf("create go.mod: %v", err)
			}

			// Sibling bar.go has the pattern; foo.go is absent
			pkgDir := filepath.Join(worktreePath, "internal", "pkg")
			if err := os.MkdirAll(pkgDir, 0o755); err != nil {
				t.Fatalf("create pkg dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(pkgDir, "bar.go"), []byte("package pkg\nfunc NewFeatureFunc() {}\n"), 0o644); err != nil {
				t.Fatalf("create bar.go: %v", err)
			}

			evidenceDir := filepath.Join(dir, "evidence")
			if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
				t.Fatalf("create evidence dir: %v", err)
			}

			contractYAML := `scenarios:
- name: status-scenario
  assertions:
  - file_contains:
      path: internal/pkg/foo.go
      pattern: NewFeatureFunc
`
			contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
			if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
				t.Fatalf("create contract: %v", err)
			}

			fakeEval := &fakeContractEvaluatorStatusVariant{}

			eventLogPath := filepath.Join(dir, "events.jsonl")
			eventLog := runstore.NewEventLog(eventLogPath)

			fakeVal := &fakeValidator{result: validator.FinalResult{Pass: true}}

			stage := NewValidateStage(fakeVal, ValidateStageConfig{
				WorkDir:          worktreePath,
				RepoDir:          dir,
				EvidenceDir:      evidenceDir,
				SearchExtensions: []string{".go"},
				SpecText:         "# Spec\n## Acceptance Criteria\n1. Implement the feature\n",
			}, eventLog, fakeEval, &validateScenarioFakeGitOps{})

			rs := runstore.NewRunState("spec-status", "proj-001")
			rs.WorktreePath = worktreePath
			rs.Cycle = 1
			rs.Tasks = []runstore.Task{
				{
					TaskID:              "task-status",
					Objective:           "Create internal/pkg/foo.go",
					Status:              status,
					ExpectedTouchedArea: []string{"internal/pkg/foo.go"},
					FilesChanged:        []string{},
				},
			}

			action, err := stage.Run(context.Background(), rs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Regardless of task status, correction must be rejected
			if action.Kind != specloop.ReplanFrom {
				t.Fatalf("status=%s: expected ReplanFrom (planned deliverable), got %v", status, action.Kind)
			}

			events, err := eventLog.ReadAll()
			if err != nil {
				t.Fatalf("read events: %v", err)
			}

			var foundRejectedEvent bool
			for _, ev := range events {
				if typedEv, ok := ev.(*runstore.ContractCorrectionRejectedEvent); ok {
					foundRejectedEvent = true
					if !strings.Contains(typedEv.Reason, "planned deliverable") {
						t.Errorf("status=%s: expected 'planned deliverable' in reason, got %q", status, typedEv.Reason)
					}
				}
			}
			if !foundRejectedEvent {
				t.Fatalf("status=%s: expected contract_correction_rejected event", status)
			}

			// Contract must not be corrected to bar.go
			contractContent, err := os.ReadFile(contractPath)
			if err != nil {
				t.Fatalf("read contract: %v", err)
			}
			if strings.Contains(string(contractContent), "bar.go") {
				t.Fatalf("status=%s: contract must not be corrected to bar.go for a planned deliverable", status)
			}
		})
	}
}

// fakeContractEvaluatorStatusVariant always returns a failure for the status-variant test.
type fakeContractEvaluatorStatusVariant struct{}

func (f *fakeContractEvaluatorStatusVariant) Evaluate(_ context.Context, _ *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
	return []contract.ContractFailure{
		{
			ScenarioName:  "status-scenario",
			AssertionType: "file_contains",
			Details:       `pattern "NewFeatureFunc" not found in "internal/pkg/foo.go"`,
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "internal/pkg/foo.go",
					Pattern: "NewFeatureFunc",
				},
			},
		},
	}, nil
}
