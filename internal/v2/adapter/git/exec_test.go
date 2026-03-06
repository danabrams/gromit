package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecGitCreateWorktree(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init")
	runGitCommand(t, repoDir, "config", "user.email", "tester@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoDir, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("writing base file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "tracked.txt")
	runGitCommand(t, repoDir, "commit", "-m", "initial")

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

func TestExecGitCommit(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init")
	runGitCommand(t, repoDir, "config", "user.email", "tester@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "Test User")
	runGitCommand(t, repoDir, "commit", "--allow-empty", "-m", "initial")

	worktreesRoot := t.TempDir()
	adapter := NewExecGit(repoDir)

	resp, err := adapter.CreateWorktree(ctx, CreateWorktreeRequest{
		SpecID:       "spec-commit",
		Reference:    "HEAD",
		WorktreeRoot: worktreesRoot,
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(resp.Worktree, "file.txt"), []byte("work"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	commitResp, err := adapter.Commit(ctx, CommitRequest{
		Worktree: resp.Worktree,
		Message:  "spec commit",
	})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if strings.TrimSpace(commitResp.CommitHash) == "" {
		t.Fatalf("expected commit hash")
	}

	got := runGitCommandOutput(t, resp.Worktree, "rev-parse", "HEAD")
	if commitResp.CommitHash != got {
		t.Fatalf("commit hash mismatch: %q vs %q", commitResp.CommitHash, got)
	}
}

func TestExecGitDiff(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init")
	runGitCommand(t, repoDir, "config", "user.email", "tester@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoDir, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("writing base file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "tracked.txt")
	runGitCommand(t, repoDir, "commit", "-m", "initial")

	worktreesRoot := t.TempDir()
	adapter := NewExecGit(repoDir)

	resp, err := adapter.CreateWorktree(ctx, CreateWorktreeRequest{
		SpecID:       "spec-diff",
		Reference:    "HEAD",
		WorktreeRoot: worktreesRoot,
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(resp.Worktree, "tracked.txt"), []byte("updated\n"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	diffResp, err := adapter.Diff(ctx, DiffRequest{
		Worktree: resp.Worktree,
		Base:     "HEAD",
	})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if !strings.Contains(diffResp.Diff, "tracked.txt") {
		t.Fatalf("diff missing file context: %q", diffResp.Diff)
	}

	if !strings.Contains(diffResp.Summary, "tracked.txt") {
		t.Fatalf("summary missing file stat: %q", diffResp.Summary)
	}
}

func runGitCommandOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
