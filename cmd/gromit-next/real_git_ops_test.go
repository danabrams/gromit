package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	env := []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("setup %v: %s\n%s", args, err, out)
		}
	}
	return dir
}

func initRepoWithFile(t *testing.T, filename, content string) string {
	t.Helper()
	dir := t.TempDir()

	env := []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}

	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "add file"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("setup %v: %s\n%s", args, err, out)
		}
	}
	return dir
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("get branch: %s", err)
	}
	return strings.TrimSpace(string(out))
}

func TestRealGitOps_CreateWorktree(t *testing.T) {
	repoDir := initBareRepo(t)
	ops := &realGitOps{}

	wtPath, err := ops.CreateWorktree(repoDir, "test-branch")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer ops.RemoveWorktree(wtPath)

	// Verify directory exists
	info, err := os.Stat(wtPath)
	if err != nil {
		t.Fatalf("worktree dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("worktree path should be a directory")
	}

	// Verify correct branch
	branch := currentBranch(t, wtPath)
	if branch != "test-branch" {
		t.Errorf("expected branch test-branch, got %s", branch)
	}
}

func TestRealGitOps_CreateWorktree_InceptionGuard(t *testing.T) {
	ops := &realGitOps{}
	// A repoDir that contains .gromit-next/worktrees/ in its path simulates
	// a nested invocation inside an existing gromit-next worktree.
	nestedRepoDir := "/some/project/.gromit-next/worktrees/wt-12345/cmd/gromit-next"
	_, err := ops.CreateWorktree(nestedRepoDir, "test-branch")
	if err == nil {
		t.Fatal("expected inception guard error, got nil")
	}
	if !strings.Contains(err.Error(), "inception guard") {
		t.Errorf("expected 'inception guard' in error, got: %v", err)
	}
}

func TestRealGitOps_RemoveWorktree(t *testing.T) {
	repoDir := initBareRepo(t)
	ops := &realGitOps{}

	wtPath, err := ops.CreateWorktree(repoDir, "remove-branch")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	if err := ops.RemoveWorktree(wtPath); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree dir should not exist after removal, err=%v", err)
	}
}

func TestRealGitOps_CommitAll(t *testing.T) {
	repoDir := initBareRepo(t)
	ops := &realGitOps{}

	wtPath, err := ops.CreateWorktree(repoDir, "commit-branch")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer ops.RemoveWorktree(wtPath)

	// Create a new file in the worktree
	if err := os.WriteFile(filepath.Join(wtPath, "new.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// CommitAll should stage and commit
	if err := ops.CommitAll(wtPath, "test: add new file"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	// Verify: git status should be clean
	cmd := exec.Command("git", "-C", wtPath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected clean working tree, got: %q", string(out))
	}

	// Verify: commit message is correct
	cmd = exec.Command("git", "-C", wtPath, "log", "-1", "--format=%s")
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if strings.TrimSpace(string(out)) != "test: add new file" {
		t.Errorf("expected commit message %q, got %q", "test: add new file", strings.TrimSpace(string(out)))
	}
}

func TestRealGitOps_CommitAll_NothingToCommit(t *testing.T) {
	repoDir := initBareRepo(t)
	ops := &realGitOps{}

	wtPath, err := ops.CreateWorktree(repoDir, "empty-commit-branch")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer ops.RemoveWorktree(wtPath)

	// CommitAll with nothing to commit should return nil (not error)
	if err := ops.CommitAll(wtPath, "empty"); err != nil {
		t.Fatalf("CommitAll on clean tree should return nil, got: %v", err)
	}
}

func TestRealGitOps_CommitAll_ExcludesGromitNext(t *testing.T) {
	repoDir := initBareRepo(t)
	ops := &realGitOps{}

	wtPath, err := ops.CreateWorktree(repoDir, "exclude-branch")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer ops.RemoveWorktree(wtPath)

	// Create files: one normal, one in .gromit-next/
	if err := os.WriteFile(filepath.Join(wtPath, "code.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gromitDir := filepath.Join(wtPath, ".gromit-next")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "run.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ops.CommitAll(wtPath, "test: exclude gromit-next"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	// Verify: code.go was committed
	cmd := exec.Command("git", "-C", wtPath, "show", "--name-only", "--format=", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git show: %v", err)
	}
	if !strings.Contains(string(out), "code.go") {
		t.Errorf("expected code.go in commit, got: %q", string(out))
	}

	// Verify: .gromit-next/ was NOT committed
	if strings.Contains(string(out), ".gromit-next") {
		t.Errorf("expected .gromit-next excluded from commit, got: %q", string(out))
	}
}

func TestRealGitOps_WorktreeHasRepoContents(t *testing.T) {
	repoDir := initRepoWithFile(t, "hello.txt", "hello world\n")
	ops := &realGitOps{}

	wtPath, err := ops.CreateWorktree(repoDir, "content-branch")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer ops.RemoveWorktree(wtPath)

	// Verify file is present in worktree
	data, err := os.ReadFile(filepath.Join(wtPath, "hello.txt"))
	if err != nil {
		t.Fatalf("file should exist in worktree: %v", err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}
