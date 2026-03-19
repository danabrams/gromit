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

// TestScenario_CommitOnValidationPass verifies that when validation passes and
// a worktree is configured, CommitAll is called with the worktree path and a
// message containing the spec ID and cycle number.
func TestScenario_CommitOnValidationPass(t *testing.T) {
	dir := t.TempDir()

	// Seed: a healthy worktree
	worktreePath := filepath.Join(dir, ".gromit-next", "worktrees", "wt-abc123")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}

	fakeGit := &validateScenarioFakeGitOps{}
	v := &fakeValidator{result: validator.FinalResult{Pass: true}}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: "/tmp/work",
		RepoDir: dir,
	}, nil, nil, fakeGit)

	rs := runstore.NewRunState("spec-042", "proj-001")
	rs.WorktreePath = worktreePath
	rs.SpecID = "spec-042"
	rs.Cycle = 3

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Assert: CommitAll was called
	if !fakeGit.commitCalled {
		t.Fatal("expected CommitAll to be called on validation pass with worktree")
	}

	// Assert: CommitAll received the worktree path as workDir
	if fakeGit.commitWorkDir != worktreePath {
		t.Errorf("expected commitWorkDir=%q, got %q", worktreePath, fakeGit.commitWorkDir)
	}

	// Assert: commit message contains spec ID and cycle
	if fakeGit.commitMessage == "" {
		t.Fatal("expected non-empty commit message")
	}
	wantMsg := "gromit: spec-042 cycle 3"
	if fakeGit.commitMessage != wantMsg {
		t.Errorf("expected commit message %q, got %q", wantMsg, fakeGit.commitMessage)
	}
}

// TestScenario_NoCommitOnValidationFail verifies that when validation fails,
// CommitAll is NOT called even when a worktree and gitOps are configured.
func TestScenario_NoCommitOnValidationFail(t *testing.T) {
	dir := t.TempDir()

	// Seed: a healthy worktree
	worktreePath := filepath.Join(dir, ".gromit-next", "worktrees", "wt-abc123")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}

	fakeGit := &validateScenarioFakeGitOps{}
	v := &fakeValidator{result: validator.FinalResult{
		Pass: false,
		AlwaysRun: validator.CheckResults{
			Results: []validator.CheckResult{{Name: "test", Pass: false, Output: "check failed"}},
		},
	}}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: "/tmp/work",
		RepoDir: dir,
	}, nil, nil, fakeGit)

	rs := runstore.NewRunState("spec-042", "proj-001")
	rs.WorktreePath = worktreePath
	rs.SpecID = "spec-042"
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}

	// Assert: CommitAll was NOT called
	if fakeGit.commitCalled {
		t.Fatal("expected CommitAll NOT to be called on validation failure")
	}
}

// TestScenario_CommitNotCalledWithoutWorktree verifies that when validation
// passes but no WorktreePath is set, CommitAll is NOT called.
func TestScenario_CommitNotCalledWithoutWorktree(t *testing.T) {
	fakeGit := &validateScenarioFakeGitOps{}
	v := &fakeValidator{result: validator.FinalResult{Pass: true}}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: "/tmp/work",
	}, nil, nil, fakeGit)

	rs := runstore.NewRunState("spec-042", "proj-001")
	// No WorktreePath set
	rs.SpecID = "spec-042"
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Assert: CommitAll was NOT called (no worktree)
	if fakeGit.commitCalled {
		t.Fatal("expected CommitAll NOT to be called when no WorktreePath is set")
	}
}
