package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// realGitOps implements stages.GitOps using real git worktree commands.
type realGitOps struct{}

// CreateWorktree creates a new git worktree on the given branch.
// It uses os.MkdirTemp to generate a unique path, removes the directory
// (git worktree add requires the target not to exist), then runs
// git worktree add -b <branch> <path>.
func (r *realGitOps) CreateWorktree(repoDir, branch string) (string, error) {
	tmp, err := os.MkdirTemp("", "gromit-wt-*")
	if err != nil {
		return "", fmt.Errorf("mkdirtemp: %w", err)
	}
	// git worktree add needs the target directory to not exist
	if err := os.Remove(tmp); err != nil {
		return "", fmt.Errorf("remove temp dir: %w", err)
	}

	cmd := exec.Command("git", "-C", repoDir, "worktree", "add", "-b", branch, tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, out)
	}
	return tmp, nil
}

// RemoveWorktree removes a git worktree. It first attempts git worktree remove --force,
// falling back to os.RemoveAll if the git command fails.
func (r *realGitOps) RemoveWorktree(path string) error {
	// Find the main repo from the worktree's .git file
	gitFilePath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitFilePath); err != nil {
		// Not a valid worktree; just remove the directory
		return os.RemoveAll(path)
	}

	// Read the .git file to find the main repo's gitdir
	data, err := os.ReadFile(gitFilePath)
	if err != nil {
		return os.RemoveAll(path)
	}

	// Parse "gitdir: /path/to/.git/worktrees/<name>"
	// Walk up to find the repo root
	line := string(data)
	if len(line) < 8 {
		return os.RemoveAll(path)
	}
	// Extract the gitdir path — format is "gitdir: <path>\n"
	var gitdir string
	if n, _ := fmt.Sscanf(line, "gitdir: %s", &gitdir); n != 1 {
		return os.RemoveAll(path)
	}

	// The gitdir points to .git/worktrees/<name>, so repo .git is two levels up
	repoGitDir := filepath.Dir(filepath.Dir(gitdir))
	// The repo working dir is one level above .git
	repoDir := filepath.Dir(repoGitDir)

	cmd := exec.Command("git", "-C", repoDir, "worktree", "remove", "--force", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback: just remove the directory
		_ = out
		return os.RemoveAll(path)
	}
	return nil
}
