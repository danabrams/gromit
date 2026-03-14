package review

import (
	"fmt"
	"os/exec"
	"strings"
)

// DiffProvider computes diffs against a base branch.
type DiffProvider interface {
	Diff(baseBranch string) (string, error)
}

// GitDiffProvider runs git diff against a base branch in a worktree directory.
type GitDiffProvider struct {
	WorkDir string
}

// Diff runs git diff against the given base branch.
// Uses "git diff <base>" (not "git diff <base>...HEAD") so that uncommitted
// working-tree changes are included. This is essential when the worktree is a
// cp -a copy of the repo (noopGitOps) where no commits are made — the three-dot
// form would always produce empty output because HEAD equals the base branch.
func (g *GitDiffProvider) Diff(baseBranch string) (string, error) {
	cmd := exec.Command("git", "diff", baseBranch)
	cmd.Dir = g.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff %s: %w", baseBranch, err)
	}
	return strings.TrimSpace(string(out)), nil
}
