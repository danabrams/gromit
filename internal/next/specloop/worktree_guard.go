package specloop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// GitStatusFunc captures `git status --porcelain` output for a directory.
// Replaceable in tests to avoid real git operations.
type GitStatusFunc func(dir string) (string, error)

// DefaultGitStatus runs `git -C dir status --porcelain` and returns stdout.
func DefaultGitStatus(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git status in %s: %w", dir, err)
	}
	return string(out), nil
}

// WorktreeGuard wraps a Stage and checks that it doesn't modify files
// in the main repo when a worktree is active. It runs `git status --porcelain`
// in the main repo before and after the inner stage, and returns Blocked
// if new uncommitted changes appear.
type WorktreeGuard struct {
	Inner     Stage
	RepoDir   string              // main repo path — checked for unexpected modifications
	GitStatus GitStatusFunc       // nil defaults to DefaultGitStatus
	Baseline  map[string]struct{} // pre-captured baseline; if set, skip the pre-snapshot
}

func (g *WorktreeGuard) Name() string { return g.Inner.Name() }

func (g *WorktreeGuard) Run(ctx context.Context, rs *runstore.RunState) (NextAction, error) {
	// No worktree active — skip guard entirely.
	if rs.WorktreePath == "" {
		return g.Inner.Run(ctx, rs)
	}

	statusFn := g.GitStatus
	if statusFn == nil {
		statusFn = DefaultGitStatus
	}

	var beforeSet map[string]struct{}
	if g.Baseline != nil {
		beforeSet = g.Baseline
	} else {
		before, err := statusFn(g.RepoDir)
		if err != nil {
			return NextAction{}, fmt.Errorf("worktree guard: pre-snapshot: %w", err)
		}
		beforeSet = ParseStatusLines(before)
	}

	action, runErr := g.Inner.Run(ctx, rs)

	after, err := statusFn(g.RepoDir)
	if err != nil {
		return NextAction{}, fmt.Errorf("worktree guard: post-snapshot: %w", err)
	}
	afterSet := ParseStatusLines(after)

	newFiles := diffSets(beforeSet, afterSet)
	if len(newFiles) > 0 {
		sort.Strings(newFiles)
		if rs.WorktreePath != "" {
			// Only auto-move untracked files ("??") — moving tracked modified files
			// would overwrite the worktree's newer versions with stale main-repo content.
			untrackedAfter := ParseUntrackedStatusLines(after)
			untrackedNew := diffSets(beforeSet, untrackedAfter)
			if len(untrackedNew) > 0 {
				sort.Strings(untrackedNew)
				if err := moveFilesToWorktree(g.RepoDir, rs.WorktreePath, untrackedNew); err == nil {
					// Only skip blocking if ALL new files were untracked and moved.
					if len(untrackedNew) == len(newFiles) {
						return action, runErr
					}
				}
			}
		}
		msg := fmt.Sprintf("worktree guard: stage %q modified main repo files: %v", g.Inner.Name(), newFiles)
		return NextAction{
			Kind: Blocked,
			Context: &FailureContext{
				Failures: []string{msg},
			},
		}, nil
	}

	return action, runErr
}

// ParseStatusLines splits `git status --porcelain` output into a set of file paths.
// All statuses are included so that modifications to tracked files are also detected.
func ParseStatusLines(output string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, line := range strings.Split(output, "\n") {
		// Porcelain format: "XY filename" — status is first 2 chars, position [2] is
		// a space, and the filename starts at position 3. Do NOT trim leading spaces
		// since position 0 can be a space (e.g. " M file.go" = unstaged modification).
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 4 {
			continue
		}
		set[line[3:]] = struct{}{}
	}
	return set
}

// ParseUntrackedStatusLines returns only "??" (untracked) file paths from git status output.
// Use this when deciding what to auto-move; tracked files must not be moved because
// the worktree may have newer versions of those same files.
func ParseUntrackedStatusLines(output string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 4 {
			continue
		}
		if line[:2] != "??" {
			continue
		}
		set[line[3:]] = struct{}{}
	}
	return set
}

// diffSets returns keys present in after but not in before.
func diffSets(before, after map[string]struct{}) []string {
	var diff []string
	for k := range after {
		if _, ok := before[k]; !ok {
			diff = append(diff, k)
		}
	}
	return diff
}

// moveFilesToWorktree moves each file from repoDir to the equivalent path in
// worktreeDir, preserving directory structure. Returns an error if any move fails.
func moveFilesToWorktree(repoDir, worktreeDir string, files []string) error {
	for _, f := range files {
		src := filepath.Join(repoDir, f)
		dst := filepath.Join(worktreeDir, f)
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read stray file %s: %w", src, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", dst, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write to worktree %s: %w", dst, err)
		}
		if err := os.Remove(src); err != nil {
			return fmt.Errorf("remove stray file %s: %w", src, err)
		}
	}
	return nil
}
