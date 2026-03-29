package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// realGitOps implements stages.GitOps using real git worktree commands.
type realGitOps struct{}

// CreateWorktree creates a new git worktree on the given branch.
// It uses os.MkdirTemp to generate a unique path, removes the directory
// (git worktree add requires the target not to exist), then runs
// git worktree add -B <branch> <path>.
func (r *realGitOps) CreateWorktree(repoDir, branch string) (string, error) {
	// Inception guard: refuse to create a worktree inside an existing worktree.
	// This prevents recursive gromit-next invocations from nesting worktrees
	// indefinitely and filling the disk.
	abs, err := filepath.Abs(repoDir)
	if err == nil && isInsideWorktree(abs) {
		return "", fmt.Errorf("inception guard: repoDir %q is inside a .gromit-next/worktrees path; refusing to create nested worktree", repoDir)
	}
	// Use a stable directory under the repo instead of OS temp dir,
	// which macOS can partially clean (deleting .git but leaving subdirs).
	wtBase := filepath.Join(repoDir, ".gromit-next", "worktrees")
	if err := os.MkdirAll(wtBase, 0o755); err != nil {
		return "", fmt.Errorf("create worktree base dir: %w", err)
	}
	tmp, err := os.MkdirTemp(wtBase, "wt-*")
	if err != nil {
		return "", fmt.Errorf("mkdirtemp: %w", err)
	}
	// git worktree add needs the target directory to not exist
	if err := os.Remove(tmp); err != nil {
		return "", fmt.Errorf("remove temp dir: %w", err)
	}

	cmd := exec.Command("git", "-C", repoDir, "worktree", "add", "-B", branch, tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, out)
	}

	// Verify worktree is not in detached HEAD state.
	headCmd := exec.Command("git", "-C", tmp, "symbolic-ref", "--short", "HEAD")
	if _, err := headCmd.Output(); err != nil {
		// Detached HEAD — force-reset the branch to current HEAD and check it out.
		// Use -B (not -b) because the branch may already exist (e.g. after -B reset above).
		fixCmd := exec.Command("git", "-C", tmp, "checkout", "-B", branch)
		if fixOut, fixErr := fixCmd.CombinedOutput(); fixErr != nil {
			return "", fmt.Errorf("fix detached HEAD: %w\n%s", fixErr, fixOut)
		}
	}

	return tmp, nil
}

// RecoverWorktree creates a git worktree for an existing branch.
// It uses os.MkdirTemp to generate a unique path, removes the directory
// (git worktree add requires the target not to exist), then runs
// git worktree add <path> <branch> (without -b flag, since the branch exists).
func (r *realGitOps) RecoverWorktree(repoDir, branch string) (string, error) {
	wtBase := filepath.Join(repoDir, ".gromit-next", "worktrees")
	if err := os.MkdirAll(wtBase, 0o755); err != nil {
		return "", fmt.Errorf("create worktree base dir: %w", err)
	}
	tmp, err := os.MkdirTemp(wtBase, "wt-*")
	if err != nil {
		return "", fmt.Errorf("mkdirtemp: %w", err)
	}
	// git worktree add needs the target directory to not exist
	if err := os.Remove(tmp); err != nil {
		return "", fmt.Errorf("remove temp dir: %w", err)
	}

	cmd := exec.Command("git", "-C", repoDir, "worktree", "add", tmp, branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, out)
	}
	return tmp, nil
}

// forceRemoveAll removes a directory tree, first making all entries writable.
// This handles Go module cache files which are read-only by default.
func forceRemoveAll(path string) error {
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		_ = os.Chmod(p, 0o700)
		return nil
	})
	return os.RemoveAll(path)
}

// CommitAll stages all changes (excluding .gromit-next/) and commits them.
// Returns nil if there is nothing to commit (clean working tree).
func (r *realGitOps) CommitAll(workDir, message string) error {
	// Stage all changes, excluding .gromit-next/
	addCmd := exec.Command("git", "-C", workDir, "add", "--", ".", ":!.gromit-next")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s: %w", string(out), err)
	}

	// Check if there's anything staged
	diffCmd := exec.Command("git", "-C", workDir, "diff", "--cached", "--quiet")
	if err := diffCmd.Run(); err == nil {
		// Exit 0 means no staged changes — nothing to commit
		return nil
	}

	// Commit with git env vars to avoid needing user config
	commitCmd := exec.Command("git", "-C", workDir, "commit", "-m", message)
	commitCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Gromit",
		"GIT_AUTHOR_EMAIL=gromit@noreply",
		"GIT_COMMITTER_NAME=Gromit",
		"GIT_COMMITTER_EMAIL=gromit@noreply",
	)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s: %w", string(out), err)
	}
	return nil
}

// RemoveWorktree removes a git worktree. It first attempts git worktree remove --force,
// falling back to forceRemoveAll if the git command fails.
func (r *realGitOps) RemoveWorktree(path string) error {
	// Find the main repo from the worktree's .git file
	gitFilePath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitFilePath); err != nil {
		// Not a valid worktree; just remove the directory
		return forceRemoveAll(path)
	}

	// Read the .git file to find the main repo's gitdir
	data, err := os.ReadFile(gitFilePath)
	if err != nil {
		return forceRemoveAll(path)
	}

	// Parse "gitdir: /path/to/.git/worktrees/<name>"
	// Walk up to find the repo root
	line := string(data)
	if len(line) < 8 {
		return forceRemoveAll(path)
	}
	// Extract the gitdir path — format is "gitdir: <path>\n"
	var gitdir string
	if n, _ := fmt.Sscanf(line, "gitdir: %s", &gitdir); n != 1 {
		return forceRemoveAll(path)
	}

	// The gitdir points to .git/worktrees/<name>, so repo .git is two levels up
	repoGitDir := filepath.Dir(filepath.Dir(gitdir))
	// The repo working dir is one level above .git
	repoDir := filepath.Dir(repoGitDir)

	cmd := exec.Command("git", "-C", repoDir, "worktree", "remove", "--force", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback: just remove the directory
		_ = out
		return forceRemoveAll(path)
	}
	return nil
}

// isInsideWorktree returns true if path contains a ".gromit-next/worktrees/"
// component, indicating it is already inside a gromit-next managed worktree.
func isInsideWorktree(path string) bool {
	return filepath.IsAbs(path) && containsPathComponent(path, ".gromit-next", "worktrees")
}

// containsPathComponent returns true if path contains the sequence parent/child
// as consecutive path components anywhere in the path.
func containsPathComponent(path, parent, child string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == parent && parts[i+1] == child {
			return true
		}
	}
	return false
}
