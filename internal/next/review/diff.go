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
func (g *GitDiffProvider) Diff(baseBranch string) (string, error) {
	cmd := exec.Command("git", "diff", baseBranch+"...HEAD")
	cmd.Dir = g.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff %s...HEAD: %w", baseBranch, err)
	}
	return strings.TrimSpace(string(out)), nil
}
