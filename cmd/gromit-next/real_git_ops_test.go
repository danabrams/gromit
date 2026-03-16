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
