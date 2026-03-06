package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
