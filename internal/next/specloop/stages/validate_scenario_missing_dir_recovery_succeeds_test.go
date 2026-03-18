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

// TestScenario_WorktreeDirMissingRecoverySucceeds verifies that when the entire
// worktree directory is absent (not just missing .git or go.mod), the health
// check detects it, recovery (remove + recreate) succeeds, and validation
// proceeds against the recovered worktree.
func TestScenario_WorktreeDirMissingRecoverySucceeds(t *testing.T) {
	base := t.TempDir()

	// Missing worktree: a path that does not exist under the base dir
	missingWorktree := filepath.Join(base, "nonexistent-worktree")
	originalMissingPath := missingWorktree

	// Create a healthy recovered worktree directory with .git and go.mod
	recoveredWorktree := filepath.Join(base, "recovered-worktree")
	if err := os.MkdirAll(recoveredWorktree, 0o755); err != nil {
		t.Fatalf("create recovered worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recoveredWorktree, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git in recovered worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recoveredWorktree, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod in recovered worktree: %v", err)
	}

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
		RepoDir: base,
	}, nil, nil, fakeGitOps)

	rs := runstore.NewRunState("spec-001", "run-001")
	rs.WorktreePath = missingWorktree
	rs.SpecID = "spec-001"
	rs.RunID = "run-001"

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: recovery succeeded, validation continues
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v (blocker: %s)", action.Kind, rs.BlockerSummary)
	}

	// Assert: final validation passed
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}

	// Assert: worktree path updated to recovered path (not the original missing path)
	if rs.WorktreePath == originalMissingPath {
		t.Fatalf("expected WorktreePath to be updated from missing path %q", originalMissingPath)
	}
	if rs.WorktreePath != recoveredWorktree {
		t.Fatalf("expected WorktreePath %q, got %q", recoveredWorktree, rs.WorktreePath)
	}

	// Assert: no blocker summary
	if rs.BlockerSummary != "" {
		t.Fatalf("expected empty BlockerSummary, got %q", rs.BlockerSummary)
	}

	// Assert: RemoveWorktree was called (attempted removal of missing path)
	if !fakeGitOps.removeCalled {
		t.Fatal("expected RemoveWorktree to be called")
	}

	// Assert: RecoverWorktree was called
	if !fakeGitOps.recoverCalled {
		t.Fatal("expected RecoverWorktree to be called")
	}
}
