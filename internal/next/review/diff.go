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

// Diff stages all changes (including untracked files) and then runs
// git diff --cached against the given base branch. Staging first ensures that
// newly created files appear in the diff output. The worktree is ephemeral,
// so staging has no side effects on the main repo.
func (g *GitDiffProvider) Diff(baseBranch string) (string, error) {
	// Stage all changes (including new files) so they appear in the diff.
	// The worktree is ephemeral, so staging has no side effects on the main repo.
	addCmd := exec.Command("git", "add", "--", ".", ":!.gromit-next")
	addCmd.Dir = g.WorkDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		// git add exits non-zero when it encounters .gitignore'd paths even
		// with a pathspec exclusion. Ignore the error when the only issue is
		// ignored files — the exclusion already prevents staging them.
		if !strings.Contains(string(out), "ignored by one of your .gitignore") {
			return "", fmt.Errorf("git add: %s: %w", string(out), err)
		}
	}

	cmd := exec.Command("git", "diff", "--cached", baseBranch)
	cmd.Dir = g.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --cached %s: %w", baseBranch, err)
	}
	return strings.TrimSpace(string(out)), nil
}
