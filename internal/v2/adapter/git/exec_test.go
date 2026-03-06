package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExecGitCreateWorktree(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init")
	runGitCommand(t, repoDir, "config", "user.email", "tester@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "Test User")
	runGitCommand(t, repoDir, "commit", "--allow-empty", "-m", "initial")

	worktreesRoot := t.TempDir()
	adapter := NewExecGit(repoDir)

	resp, err := adapter.CreateWorktree(ctx, CreateWorktreeRequest{
		SpecID:       "spec-create",
		Reference:    "HEAD",
		WorktreeRoot: worktreesRoot,
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	expected := filepath.Join(worktreesRoot, "spec-create")
	if resp.Worktree != expected {
		t.Fatalf("unexpected worktree path: %q", resp.Worktree)
	}
}

func TestExecGitRemoveWorktree(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init")
	runGitCommand(t, repoDir, "config", "user.email", "tester@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "Test User")
	runGitCommand(t, repoDir, "commit", "--allow-empty", "-m", "initial")

	worktreesRoot := t.TempDir()
	adapter := NewExecGit(repoDir)

	resp, err := adapter.CreateWorktree(ctx, CreateWorktreeRequest{
		SpecID:       "spec-remove",
		Reference:    "HEAD",
		WorktreeRoot: worktreesRoot,
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if _, err := adapter.RemoveWorktree(ctx, RemoveWorktreeRequest{Worktree: resp.Worktree}); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	if _, err := os.Stat(resp.Worktree); !os.IsNotExist(err) {
		t.Fatalf("expected worktree to be removed, stat error: %v", err)
	}
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
