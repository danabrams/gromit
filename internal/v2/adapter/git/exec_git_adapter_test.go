package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGitBinary(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// initTestRepo creates a bare-minimum git repo with one commit and returns
// the repo root path. The caller's CWD is intentionally NOT changed, so
// tests verify that ExecGitAdapter uses repoRoot (cmd.Dir) rather than CWD.
func initTestRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	runGitBinary(t, repoDir, "init")
	runGitBinary(t, repoDir, "config", "user.email", "tester@example.com")
	runGitBinary(t, repoDir, "config", "user.name", "Test User")
	runGitBinary(t, repoDir, "commit", "--allow-empty", "-m", "initial")
	return repoDir
}

func TestExecGitAdapterCheckoutSetsDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	// Change CWD to a directory that is NOT a git repo, so if cmd.Dir is
	// not set the git command will fail.
	nonRepoDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(nonRepoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := adapter.Checkout(ctx, "spec-dir-test")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	expected := filepath.Join(worktreesDir, "spec-dir-test")
	if wtPath != expected {
		t.Fatalf("unexpected worktree path: got %q, want %q", wtPath, expected)
	}

	// Verify the worktree directory was actually created.
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}
}

func TestExecGitAdapterRemoveWorktreeSetsDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	// First create a worktree so we can remove it.
	wtPath, err := adapter.Checkout(ctx, "spec-remove-dir")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Change CWD away from the repo to prove RemoveWorktree uses repoRoot.
	nonRepoDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(nonRepoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	if err := adapter.RemoveWorktree(ctx, wtPath); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree removed, stat error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Diff tests
// ---------------------------------------------------------------------------

func TestExecGitAdapterDiff_EmptyWorktree(t *testing.T) {
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	_, err := adapter.Diff(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "worktree required") {
		t.Fatalf("expected 'worktree required' error, got: %v", err)
	}
}

func TestExecGitAdapterDiff_WhitespaceOnlyWorktree(t *testing.T) {
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	_, err := adapter.Diff(context.Background(), "   ")
	if err == nil || !strings.Contains(err.Error(), "worktree required") {
		t.Fatalf("expected 'worktree required' error, got: %v", err)
	}
}

func TestExecGitAdapterDiff_NoDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()
	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := adapter.Checkout(ctx, "spec-diff-nodiff")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	diff, err := adapter.Diff(ctx, wtPath)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff != "" {
		t.Fatalf("expected empty diff, got: %q", diff)
	}
}

func TestExecGitAdapterDiff_WithChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()
	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := adapter.Checkout(ctx, "spec-diff-changes")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Create a tracked file and commit, then modify it so diff HEAD shows something.
	if err := os.WriteFile(filepath.Join(wtPath, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitBinary(t, wtPath, "add", "-A")
	runGitBinary(t, wtPath, "commit", "-m", "add hello")

	if err := os.WriteFile(filepath.Join(wtPath, "hello.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	diff, err := adapter.Diff(ctx, wtPath)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if !strings.Contains(diff, "hello world") {
		t.Fatalf("expected diff to contain 'hello world', got: %q", diff)
	}
}

func TestExecGitAdapterDiff_InvalidDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	_, err := adapter.Diff(context.Background(), "/nonexistent-dir-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

// ---------------------------------------------------------------------------
// Commit tests
// ---------------------------------------------------------------------------

func TestExecGitAdapterCommit_EmptyWorktree(t *testing.T) {
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	_, err := adapter.Commit(context.Background(), "", "msg")
	if err == nil || !strings.Contains(err.Error(), "worktree required") {
		t.Fatalf("expected 'worktree required' error, got: %v", err)
	}
}

func TestExecGitAdapterCommit_EmptyMessage(t *testing.T) {
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	_, err := adapter.Commit(context.Background(), "/tmp", "")
	if err == nil || !strings.Contains(err.Error(), "commit message required") {
		t.Fatalf("expected 'commit message required' error, got: %v", err)
	}
}

func TestExecGitAdapterCommit_WhitespaceMessage(t *testing.T) {
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	_, err := adapter.Commit(context.Background(), "/tmp", "   ")
	if err == nil || !strings.Contains(err.Error(), "commit message required") {
		t.Fatalf("expected 'commit message required' error, got: %v", err)
	}
}

func TestExecGitAdapterCommit_Success(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()
	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := adapter.Checkout(ctx, "spec-commit-ok")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Create a file so there is something to commit.
	if err := os.WriteFile(filepath.Join(wtPath, "file.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	hash, err := adapter.Commit(ctx, wtPath, "test commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if len(hash) < 7 {
		t.Fatalf("expected a commit hash, got: %q", hash)
	}
}

func TestExecGitAdapterCommit_NothingToCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()
	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := adapter.Checkout(ctx, "spec-commit-empty")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Commit with nothing staged should fail.
	_, err = adapter.Commit(ctx, wtPath, "empty commit")
	if err == nil {
		t.Fatal("expected error for nothing-to-commit")
	}
	if !strings.Contains(err.Error(), "git commit") {
		t.Fatalf("expected 'git commit' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Status tests
// ---------------------------------------------------------------------------

func TestExecGitAdapterStatus_EmptyWorktree(t *testing.T) {
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	_, err := adapter.Status(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "worktree required") {
		t.Fatalf("expected 'worktree required' error, got: %v", err)
	}
}

func TestExecGitAdapterStatus_CleanWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()
	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := adapter.Checkout(ctx, "spec-status-clean")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	status, err := adapter.Status(ctx, wtPath)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("expected empty status for clean worktree, got: %q", status)
	}
}

func TestExecGitAdapterStatus_DirtyWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()
	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := adapter.Checkout(ctx, "spec-status-dirty")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Create an untracked file.
	if err := os.WriteFile(filepath.Join(wtPath, "untracked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	status, err := adapter.Status(ctx, wtPath)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !strings.Contains(status, "untracked.txt") {
		t.Fatalf("expected status to mention untracked.txt, got: %q", status)
	}
}

func TestExecGitAdapterStatus_InvalidDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	_, err := adapter.Status(context.Background(), "/nonexistent-dir-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

// ---------------------------------------------------------------------------
// RemoveWorktree validation tests
// ---------------------------------------------------------------------------

func TestExecGitAdapterRemoveWorktree_EmptyWorktree(t *testing.T) {
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	err := adapter.RemoveWorktree(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "worktree required") {
		t.Fatalf("expected 'worktree required' error, got: %v", err)
	}
}

func TestExecGitAdapterRemoveWorktree_WhitespaceWorktree(t *testing.T) {
	adapter := NewExecGitAdapter("/tmp", "/tmp")
	err := adapter.RemoveWorktree(context.Background(), "   ")
	if err == nil || !strings.Contains(err.Error(), "worktree required") {
		t.Fatalf("expected 'worktree required' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// NewExecGitAdapter constructor test
// ---------------------------------------------------------------------------

func TestNewExecGitAdapter(t *testing.T) {
	a := NewExecGitAdapter("/repo", "/worktrees")
	if a.repoRoot != "/repo" {
		t.Fatalf("expected repoRoot '/repo', got %q", a.repoRoot)
	}
	if a.worktreesDir != "/worktrees" {
		t.Fatalf("expected worktreesDir '/worktrees', got %q", a.worktreesDir)
	}
}

// ---------------------------------------------------------------------------
// Context cancellation test
// ---------------------------------------------------------------------------

func TestExecGitAdapterDiff_CancelledContext(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()
	adapter := NewExecGitAdapter(repoDir, worktreesDir)

	ctx, cancel := context.WithCancel(context.Background())
	wtPath, err := adapter.Checkout(ctx, "spec-ctx-cancel")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	cancel()
	_, err = adapter.Diff(ctx, wtPath)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
