package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// TestScenario_HealthyRecovery verifies that when a worktree is unhealthy,
// recovery succeeds and validation continues with the recovered worktree path.
func TestScenario_HealthyRecovery(t *testing.T) {
	dir := t.TempDir()

	// Create a broken worktree directory (missing go.mod to trigger health check failure)
	brokenWorktree := filepath.Join(dir, "broken-worktree")
	if err := os.MkdirAll(brokenWorktree, 0o755); err != nil {
		t.Fatalf("create broken worktree: %v", err)
	}
	// Create .git but no go.mod to trigger health check failure
	if err := os.WriteFile(filepath.Join(brokenWorktree, ".git"), []byte(""), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}

	// Create a healthy recovered worktree directory
	recoveredWorktree := filepath.Join(dir, "recovered-worktree")
	if err := os.MkdirAll(recoveredWorktree, 0o755); err != nil {
		t.Fatalf("create recovered worktree: %v", err)
	}
	// Create both .git and go.mod for a healthy worktree
	if err := os.WriteFile(filepath.Join(recoveredWorktree, ".git"), []byte(""), 0o644); err != nil {
		t.Fatalf("create .git in recovered worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recoveredWorktree, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod in recovered worktree: %v", err)
	}

	// Create fakeGitOps that returns the healthy recovered worktree
	fakeGitOps := &validateScenarioFakeGitOps{
		recoveredPath: recoveredWorktree,
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: true,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "test", Pass: true}},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: "/tmp/work",
		RepoDir: "/repo",
	}, nil, nil, fakeGitOps)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = brokenWorktree
	rs.SpecID = "spec-001"
	rs.RunID = "run-001"

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect Continue since recovery succeeded
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify worktree path was updated to the recovered path
	if rs.WorktreePath != recoveredWorktree {
		t.Fatalf("expected WorktreePath to be updated to %q, got %q", recoveredWorktree, rs.WorktreePath)
	}

	// Verify recovery operations were called
	if !fakeGitOps.removeCalled {
		t.Fatal("expected RemoveWorktree to be called")
	}
	if !fakeGitOps.recoverCalled {
		t.Fatal("expected RecoverWorktree to be called")
	}

	// Verify final validation passed
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}
}
