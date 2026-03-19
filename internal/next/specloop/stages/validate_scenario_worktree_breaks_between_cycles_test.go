package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// TestScenario_WorktreeBreaksBetweenCycles verifies that when a worktree was
// healthy on cycle 1 but its .git file is deleted before cycle 2 (e.g.,
// filesystem corruption), the validate stage detects the missing .git file,
// recovers the worktree via RemoveWorktree + RecoverWorktree, updates
// rs.WorktreePath, proceeds with validation, and emits a WorktreeRecoveryEvent
// with RecoverySucceeded: true.
func TestScenario_WorktreeBreaksBetweenCycles(t *testing.T) {
	dir := t.TempDir()

	// Seed: a worktree directory that was healthy on cycle 1, but .git was
	// deleted between cycles (simulating filesystem corruption / manual deletion).
	brokenWorktree := filepath.Join(dir, ".gromit-next", "worktrees", "wt-abc123")
	if err := os.MkdirAll(brokenWorktree, 0o755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}
	// go.mod is still present — only .git was deleted
	if err := os.WriteFile(filepath.Join(brokenWorktree, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}
	// Deliberately no .git file — simulates deletion between cycle 1 and cycle 2

	// Seed: a healthy recovered worktree with both .git and go.mod
	recoveredWorktree := filepath.Join(dir, ".gromit-next", "worktrees", "wt-recovered")
	if err := os.MkdirAll(recoveredWorktree, 0o755); err != nil {
		t.Fatalf("create recovered worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recoveredWorktree, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git in recovered: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recoveredWorktree, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod in recovered: %v", err)
	}

	// Seed: event log to capture WorktreeRecoveryEvent
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	fakeGit := &validateScenarioFakeGitOps{
		recoveredPath: recoveredWorktree,
	}

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
		WorkDir: "/tmp/work",
		RepoDir: dir,
	}, eventLog, nil, fakeGit)

	// Seed: RunState simulating cycle 2 — cycle 1 completed successfully,
	// worktree was healthy then but .git has since been deleted.
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = brokenWorktree
	rs.SpecID = "spec-001"
	rs.RunID = "run-001"
	rs.Cycle = 2

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: recovery succeeded, validation continues
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v (blocker: %s)", action.Kind, rs.BlockerSummary)
	}

	// Assert: worktree path updated to the recovered path
	if rs.WorktreePath != recoveredWorktree {
		t.Fatalf("expected WorktreePath %q, got %q", recoveredWorktree, rs.WorktreePath)
	}

	// Assert: final validation passed
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}

	// Assert: RemoveWorktree was called to clean up the broken worktree
	if !fakeGit.removeCalled {
		t.Fatal("expected RemoveWorktree to be called for broken worktree")
	}

	// Assert: RecoverWorktree was called with the correct branch
	if !fakeGit.recoverCalled {
		t.Fatal("expected RecoverWorktree to be called")
	}
	wantBranch := "gromit/spec-spec-001-run-001"
	if fakeGit.recoverBranch != wantBranch {
		t.Fatalf("expected recovery branch %q, got %q", wantBranch, fakeGit.recoverBranch)
	}

	// Assert: WorktreeRecoveryEvent emitted with RecoverySucceeded: true
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	var foundRecovery bool
	for _, ev := range events {
		if wre, ok := ev.(*runstore.WorktreeRecoveryEvent); ok {
			foundRecovery = true
			if !wre.RecoverySucceeded {
				t.Error("expected RecoverySucceeded=true in event")
			}
			if wre.NewWorktreePath != recoveredWorktree {
				t.Errorf("event NewWorktreePath = %q, want %q", wre.NewWorktreePath, recoveredWorktree)
			}
			// Health check failure should mention the missing .git file
			if !strings.Contains(wre.HealthCheckFailure, ".git") {
				t.Errorf("expected health check failure to mention .git, got %q", wre.HealthCheckFailure)
			}
		}
	}
	if !foundRecovery {
		t.Error("expected worktree_recovery event to be emitted")
	}
}
