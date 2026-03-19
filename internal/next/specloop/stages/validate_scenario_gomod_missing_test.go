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

// TestScenario_GomodMissing verifies that when a worktree has .git but is missing
// go.mod (e.g. corrupted checkout), the health check detects it, recovery
// (remove + recreate) succeeds, and validation proceeds against the recovered worktree.
func TestScenario_GomodMissing(t *testing.T) {
	dir := t.TempDir()

	// Seed: a worktree directory with .git present but go.mod missing
	brokenWorktree := filepath.Join(dir, ".gromit-next", "worktrees", "wt-abc123")
	if err := os.MkdirAll(brokenWorktree, 0o755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenWorktree, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	// Deliberately no go.mod — simulates the corrupted checkout from the incident

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

	// Seed: event log to capture worktree_recovery event
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	fakeGitOps := &validateScenarioFakeGitOps{
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
	}, eventLog, nil, fakeGitOps)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = brokenWorktree
	rs.SpecID = "spec-001"
	rs.RunID = "run-001"

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: recovery succeeded, validation continues
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v (blocker: %s)", action.Kind, rs.BlockerSummary)
	}

	// Assert: worktree path updated to recovered path
	if rs.WorktreePath != recoveredWorktree {
		t.Fatalf("expected WorktreePath %q, got %q", recoveredWorktree, rs.WorktreePath)
	}

	// Assert: final validation passed
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}

	// Assert: RemoveWorktree was called (old worktree cleaned up)
	if !fakeGitOps.removeCalled {
		t.Fatal("expected RemoveWorktree to be called for broken worktree")
	}

	// Assert: RecoverWorktree was called with the correct branch
	if !fakeGitOps.recoverCalled {
		t.Fatal("expected RecoverWorktree to be called")
	}
	wantBranch := "gromit/spec-spec-001-run-001"
	if fakeGitOps.recoverBranch != wantBranch {
		t.Fatalf("expected recovery branch %q, got %q", wantBranch, fakeGitOps.recoverBranch)
	}

	// Assert: worktree_recovery event emitted with success
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
			if !strings.Contains(wre.HealthCheckFailure, "go.mod") {
				t.Errorf("expected health check failure to mention go.mod, got %q", wre.HealthCheckFailure)
			}
		}
	}
	if !foundRecovery {
		t.Error("expected worktree_recovery event to be emitted")
	}
}
