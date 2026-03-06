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
	runGitBinary(t, repoDir, "init")
	runGitBinary(t, repoDir, "config", "user.email", "tester@example.com")
	runGitBinary(t, repoDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("writing base file: %v", err)
	}
	runGitBinary(t, repoDir, "add", "base.txt")
	runGitBinary(t, repoDir, "commit", "-m", "initial")

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

func TestExecGitRemoveWorktree(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	repoDir := t.TempDir()
	runGitBinary(t, repoDir, "init")
	runGitBinary(t, repoDir, "config", "user.email", "tester@example.com")
	runGitBinary(t, repoDir, "config", "user.name", "Test User")
	runGitBinary(t, repoDir, "commit", "--allow-empty", "-m", "initial")

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

func TestExecGitCommit(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	repoDir := t.TempDir()
	runGitBinary(t, repoDir, "init")
	runGitBinary(t, repoDir, "config", "user.email", "tester@example.com")
	runGitBinary(t, repoDir, "config", "user.name", "Test User")
	runGitBinary(t, repoDir, "commit", "--allow-empty", "-m", "initial")

	worktreesRoot := t.TempDir()
	adapter := NewExecGit(repoDir)

	resp, err := adapter.CreateWorktree(ctx, CreateWorktreeRequest{
		SpecID:       "spec-commit",
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
		t.Fatalf("expected non-empty commit hash")
	}

	got := runGitBinaryOutput(t, resp.Worktree, "rev-parse", "HEAD")
	if commitResp.CommitHash != got {
		t.Fatalf("commit hash mismatch: %q vs %q", commitResp.CommitHash, got)
	}
}

func TestExecGitDiff(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	repoDir := t.TempDir()
	runGitBinary(t, repoDir, "init")
	runGitBinary(t, repoDir, "config", "user.email", "tester@example.com")
	runGitBinary(t, repoDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoDir, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("writing base file: %v", err)
	}
	runGitBinary(t, repoDir, "add", "tracked.txt")
	runGitBinary(t, repoDir, "commit", "-m", "initial")

	worktreesRoot := t.TempDir()
	adapter := NewExecGit(repoDir)

	resp, err := adapter.CreateWorktree(ctx, CreateWorktreeRequest{
		SpecID:       "spec-diff",
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
		t.Fatalf("summary missing tracked file: %q", diffResp.Summary)
	}
}

func runGitBinary(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func runGitBinaryOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
