package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPreserveBranchLogsWhenBranchExists verifies that PreserveBranch
// logs the preservation and returns no error when the branch exists.
func TestPreserveBranchLogsWhenBranchExists(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot := t.TempDir()
	initTestGitRepo(t, repoRoot)

	specID := "test-spec-preserve"
	worktreesDir := filepath.Join(repoRoot, ".gromit", "worktrees")
	adapter := NewExecGitAdapter(repoRoot, worktreesDir)

	// Create a worktree (which creates the branch)
	_, err := adapter.Checkout(context.Background(), specID)
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	// PreserveBranch should not error
	err = adapter.PreserveBranch(context.Background(), specID)
	if err != nil {
		t.Fatalf("PreserveBranch: %v", err)
	}

	// Verify the branch still exists
	branchName := "gromit/spec/" + specID
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("branch check failed: %s: %v", out, err)
	}
}

// TestPreserveBranchHandlesMissingBranch verifies that PreserveBranch
// returns no error when the branch doesn't exist (graceful).
func TestPreserveBranchHandlesMissingBranch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot := t.TempDir()
	initTestGitRepo(t, repoRoot)

	adapter := NewExecGitAdapter(repoRoot, filepath.Join(repoRoot, ".gromit", "worktrees"))

	// PreserveBranch for a non-existent spec should not error
	err := adapter.PreserveBranch(context.Background(), "nonexistent-spec")
	if err != nil {
		t.Fatalf("PreserveBranch should not error for non-existent spec, got: %v", err)
	}
}

// TestPreserveBranchRequiresSpecID verifies that PreserveBranch
// returns an error when spec ID is empty.
func TestPreserveBranchRequiresSpecID(t *testing.T) {
	t.Parallel()

	adapter := NewExecGitAdapter("/tmp", "/tmp")

	err := adapter.PreserveBranch(context.Background(), "")
	if err == nil {
		t.Fatal("PreserveBranch should error for empty spec ID")
	}
	if !strings.Contains(err.Error(), "spec ID required") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "spec ID required")
	}
}

// TestDeleteBranchRemovesBranch verifies that DeleteBranch removes
// the worktree branch when it exists.
func TestDeleteBranchRemovesBranch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot := t.TempDir()
	initTestGitRepo(t, repoRoot)

	specID := "test-spec-delete"
	worktreesDir := filepath.Join(repoRoot, ".gromit", "worktrees")
	adapter := NewExecGitAdapter(repoRoot, worktreesDir)

	// Create a worktree (which creates the branch)
	wtPath, err := adapter.Checkout(context.Background(), specID)
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	// Verify branch exists before delete
	branchName := "gromit/spec/" + specID
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("branch should exist before delete: %s: %v", out, err)
	}

	// Remove the worktree first (branch can't be deleted while worktree uses it)
	err = adapter.RemoveWorktree(context.Background(), wtPath)
	if err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	// DeleteBranch should not error
	err = adapter.DeleteBranch(context.Background(), specID)
	if err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	// Verify branch is deleted
	cmd = exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("branch should not exist after delete, but did: %s", out)
	}
}

// TestDeleteBranchHandlesMissingBranch verifies that DeleteBranch
// returns no error when the branch doesn't exist (graceful).
func TestDeleteBranchHandlesMissingBranch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot := t.TempDir()
	initTestGitRepo(t, repoRoot)

	adapter := NewExecGitAdapter(repoRoot, filepath.Join(repoRoot, ".gromit", "worktrees"))

	// DeleteBranch for a non-existent spec should not error
	err := adapter.DeleteBranch(context.Background(), "nonexistent-spec")
	if err != nil {
		t.Fatalf("DeleteBranch should not error for non-existent spec, got: %v", err)
	}
}

// TestDeleteBranchRequiresSpecID verifies that DeleteBranch
// returns an error when spec ID is empty.
func TestDeleteBranchRequiresSpecID(t *testing.T) {
	t.Parallel()

	adapter := NewExecGitAdapter("/tmp", "/tmp")

	err := adapter.DeleteBranch(context.Background(), "")
	if err == nil {
		t.Fatal("DeleteBranch should error for empty spec ID")
	}
	if !strings.Contains(err.Error(), "spec ID required") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "spec ID required")
	}
}

// initTestGitRepo creates a minimal git repo with one commit.
func initTestGitRepo(t *testing.T, repoRoot string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.email: %s: %v", out, err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.name: %s: %v", out, err)
	}

	// Create initial commit
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("initial"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %s: %v", out, err)
	}

	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %s: %v", out, err)
	}
}
