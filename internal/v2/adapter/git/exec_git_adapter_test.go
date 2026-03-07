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
	t.Parallel()
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

func TestExecGitAdapterCheckoutIdempotent(t *testing.T) {
	t.Parallel()
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	// First checkout should succeed.
	wtPath1, err := adapter.Checkout(ctx, "spec-idempotent")
	if err != nil {
		t.Fatalf("first Checkout failed: %v", err)
	}

	expected := filepath.Join(worktreesDir, "spec-idempotent")
	if wtPath1 != expected {
		t.Fatalf("unexpected worktree path: got %q, want %q", wtPath1, expected)
	}

	// Second checkout with the same specID should also succeed (not fail
	// with "already exists"), simulating a retry after a preserved failure.
	wtPath2, err := adapter.Checkout(ctx, "spec-idempotent")
	if err != nil {
		t.Fatalf("second Checkout failed: %v", err)
	}

	if wtPath2 != expected {
		t.Fatalf("unexpected worktree path on second call: got %q, want %q", wtPath2, expected)
	}

	// Verify the worktree directory exists.
	if _, err := os.Stat(wtPath2); err != nil {
		t.Fatalf("worktree dir not created after second Checkout: %v", err)
	}
}

func TestExecGitAdapterCheckoutCreatesBranch(t *testing.T) {
	t.Parallel()
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	adapter := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := adapter.Checkout(ctx, "my-spec")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Verify the branch exists in the repo.
	listCmd := exec.Command("git", "branch", "--list", "gromit/spec/my-spec")
	listCmd.Dir = repoDir
	listOut, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list failed: %v\n%s", err, listOut)
	}
	if len(listOut) == 0 {
		t.Fatalf("expected branch gromit/spec/my-spec to exist, but git branch --list returned empty")
	}

	// Verify the worktree HEAD is on the named branch.
	headCmd := exec.Command("git", "-C", wtPath, "rev-parse", "--abbrev-ref", "HEAD")
	headOut, err := headCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse --abbrev-ref HEAD failed: %v\n%s", err, headOut)
	}
	got := string(headOut)
	got = got[:len(got)-1] // trim trailing newline
	if got != "gromit/spec/my-spec" {
		t.Fatalf("worktree HEAD branch: got %q, want %q", got, "gromit/spec/my-spec")
	}
}

func TestExecGitAdapterLogReturnsMostRecentFirst(t *testing.T) {
	t.Parallel()
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	a := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	// Create a worktree and add two commits.
	wtPath, err := a.Checkout(ctx, "spec-log-test")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	// Write a file and commit.
	if err := os.WriteFile(filepath.Join(wtPath, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile a.txt: %v", err)
	}
	runGitBinary(t, wtPath, "add", "-A")
	runGitBinary(t, wtPath, "commit", "-m", "first commit")

	if err := os.WriteFile(filepath.Join(wtPath, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile b.txt: %v", err)
	}
	runGitBinary(t, wtPath, "add", "-A")
	runGitBinary(t, wtPath, "commit", "-m", "second commit")

	entries, err := a.Log(ctx, wtPath, 2)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}
	if entries[0].Message != "second commit" {
		t.Fatalf("expected most recent commit first, got %q", entries[0].Message)
	}
	if entries[1].Message != "first commit" {
		t.Fatalf("expected first commit second, got %q", entries[1].Message)
	}
	if entries[0].Hash == "" {
		t.Fatal("expected non-empty hash for first entry")
	}
}

func TestExecGitAdapterShowReturnsDiff(t *testing.T) {
	t.Parallel()
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	a := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := a.Checkout(ctx, "spec-show-test")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	// Add a file and commit it.
	if err := os.WriteFile(filepath.Join(wtPath, "show.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGitBinary(t, wtPath, "add", "-A")
	runGitBinary(t, wtPath, "commit", "-m", "show commit")

	// Get the commit hash.
	entries, err := a.Log(ctx, wtPath, 1)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	diff, err := a.Show(ctx, wtPath, entries[0].Hash)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff from Show")
	}
	if !strings.Contains(diff, "show.txt") {
		t.Fatalf("expected diff to mention show.txt, got: %q", diff)
	}
}

func TestExecGitAdapterSquashCommitsCollapsesCommits(t *testing.T) {
	t.Parallel()
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	a := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := a.Checkout(ctx, "spec-squash-test")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	// Add three commits.
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(wtPath, name), []byte(name), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
		runGitBinary(t, wtPath, "add", "-A")
		runGitBinary(t, wtPath, "commit", "-m", "add "+name)
	}

	// Squash last 3 commits into 1.
	if err := a.SquashCommits(ctx, wtPath, 3); err != nil {
		t.Fatalf("SquashCommits: %v", err)
	}

	// After squash, git log should show only the initial commit (from initTestRepo) left.
	entries, err := a.Log(ctx, wtPath, 10)
	if err != nil {
		t.Fatalf("Log after squash: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 commit after squash, got %d: %v", len(entries), entries)
	}
}

func TestExecGitAdapterLogRejectsNonPositiveN(t *testing.T) {
	t.Parallel()
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	a := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := a.Checkout(ctx, "spec-log-validate")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	for _, n := range []int{0, -1, -100} {
		_, err := a.Log(ctx, wtPath, n)
		if err == nil {
			t.Errorf("Log(n=%d) should return error, got nil", n)
		}
		if err != nil && !strings.Contains(err.Error(), "n must be positive") {
			t.Errorf("Log(n=%d) error = %q, want it to contain %q", n, err.Error(), "n must be positive")
		}
	}
}

func TestExecGitAdapterSquashCommitsRejectsNonPositiveCount(t *testing.T) {
	t.Parallel()
	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	a := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()

	wtPath, err := a.Checkout(ctx, "spec-squash-validate")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	for _, count := range []int{0, -1, -100} {
		err := a.SquashCommits(ctx, wtPath, count)
		if err == nil {
			t.Errorf("SquashCommits(count=%d) should return error, got nil", count)
		}
		if err != nil && !strings.Contains(err.Error(), "count must be positive") {
			t.Errorf("SquashCommits(count=%d) error = %q, want it to contain %q", count, err.Error(), "count must be positive")
		}
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
