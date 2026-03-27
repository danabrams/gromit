package stages

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// TestScenario_RemoveWorktreeFailsDuringRecovery verifies that when a worktree
// has a missing .git file and RemoveWorktree returns an error (e.g., permission
// denied), recovery is aborted without calling RecoverWorktree. The stage sets
// BlockerSummary to "infrastructure: worktree cleanup failed: <error>" and
// returns Blocked. A WorktreeRecoveryEvent is emitted with RecoverySucceeded: false.
func TestScenario_RemoveWorktreeFailsDuringRecovery(t *testing.T) {
	dir := t.TempDir()

	// Seed: a worktree directory with go.mod but NO .git file (broken worktree)
	brokenWorktree := filepath.Join(dir, ".gromit-next", "worktrees", "wt-abc123")
	if err := os.MkdirAll(brokenWorktree, 0o755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenWorktree, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}
	// Deliberately no .git — simulates a broken worktree

	// Seed: event log to capture WorktreeRecoveryEvent
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// Seed: work directory
	workDir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("create work dir: %v", err)
	}

	// Seed: fakeGitOps where RemoveWorktree returns an error
	fakeGit := &validateScenarioFakeGitOps{
		removeErr: errors.New("permission denied on worktree directory"),
	}

	// Use a passing validator — recovery failure should prevent it from running
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: true,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "go test ./...", Pass: true}},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: workDir,
		RepoDir: dir,
	}, eventLog, nil, fakeGit)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = brokenWorktree
	rs.SpecID = "spec-001"
	rs.RunID = "run-001"

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: stage returns Blocked
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked, got %v", action.Kind)
	}

	// Assert: no replan context
	if action.Context != nil {
		t.Fatalf("expected nil FailureContext (no replan), got %+v", action.Context)
	}

	// Assert: BlockerSummary starts with "infrastructure: worktree cleanup failed:"
	if !strings.HasPrefix(rs.BlockerSummary, "infrastructure: worktree cleanup failed:") {
		t.Fatalf("expected BlockerSummary to start with 'infrastructure: worktree cleanup failed:', got %q", rs.BlockerSummary)
	}

	// Assert: BlockerSummary contains the removal error details
	if !strings.Contains(rs.BlockerSummary, "permission denied on worktree directory") {
		t.Fatalf("expected BlockerSummary to contain removal error details, got %q", rs.BlockerSummary)
	}

	// Assert: RemoveWorktree was called
	if !fakeGit.removeCalled {
		t.Fatal("expected RemoveWorktree to be called")
	}

	// Assert: RecoverWorktree was NOT called (recovery aborted after cleanup failure)
	if fakeGit.recoverCalled {
		t.Fatal("expected RecoverWorktree NOT to be called when RemoveWorktree fails")
	}

	// Assert: WorktreeRecoveryEvent emitted with RecoverySucceeded=false
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	var foundRecoveryEvent bool
	for _, ev := range events {
		if wre, ok := ev.(*runstore.WorktreeRecoveryEvent); ok {
			foundRecoveryEvent = true
			if wre.RecoverySucceeded {
				t.Error("expected RecoverySucceeded=false in WorktreeRecoveryEvent")
			}
			if wre.NewWorktreePath != "" {
				t.Errorf("expected empty NewWorktreePath on failure, got %q", wre.NewWorktreePath)
			}
			if !strings.Contains(wre.HealthCheckFailure, ".git file missing") {
				t.Errorf("expected HealthCheckFailure to mention '.git file missing', got %q", wre.HealthCheckFailure)
			}
		}
	}
	if !foundRecoveryEvent {
		t.Error("expected WorktreeRecoveryEvent to be emitted")
	}

	// Assert: FinalValidationPassed was NOT set (validation never ran)
	if rs.FinalValidationPassed {
		t.Error("expected FinalValidationPassed to be false since recovery was aborted")
	}
}
