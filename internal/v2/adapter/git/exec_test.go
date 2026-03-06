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
    if err := os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
        t.Fatalf("writing base file: %v", err)
    }
    runGitCommand(t, repoDir, "add", "base.txt")
    runGitCommand(t, repoDir, "commit", "-m", "initial")

    worktreesRoot := t.TempDir()
    adapter := NewExecGit(repoDir)

    resp, err := adapter.CreateWorktree(ctx, CreateWorktreeRequest{
        SpecID:       "spec-1",
        WorktreeRoot: worktreesRoot,
    })
    if err != nil {
        t.Fatalf("CreateWorktree failed: %v", err)
    }

    expected := filepath.Join(worktreesRoot, "spec-1")
    if resp.Worktree != expected {
        t.Fatalf("unexpected worktree path: %s", resp.Worktree)
    }
}

func runGitCommand(t *testing.T, dir string, args ...string) {
    t.Helper()
    cmd := exec.Command("git", args...)
    cmd.Dir = dir
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("git %v failed: %v\n%s", args, err, out)
    }
}

func TestExecGitRemoveWorktree(t *testing.T) {
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
		SpecID:       "spec-remove",
		WorktreeRoot: worktreesRoot,
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if _, err := adapter.RemoveWorktree(ctx, RemoveWorktreeRequest{Worktree: resp.Worktree}); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	if _, err := os.Stat(resp.Worktree); !os.IsNotExist(err) {
		t.Fatalf("expected worktree removed, stat error %v", err)
	}
}
