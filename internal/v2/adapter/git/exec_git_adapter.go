package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/v2/adapter"
)

var _ adapter.GitAdapter = (*ExecGitAdapter)(nil)

// ExecGitAdapter implements GitAdapter using exec.Command("git", ...).
type ExecGitAdapter struct {
	repoRoot     string
	worktreesDir string
}

// NewExecGitAdapter returns an ExecGitAdapter that creates worktrees under worktreesDir.
// repoRoot is the path to the git repository root; git commands that operate on the
// repository (e.g. worktree add/remove) run with Dir set to repoRoot.
func NewExecGitAdapter(repoRoot, worktreesDir string) *ExecGitAdapter {
	return &ExecGitAdapter{repoRoot: repoRoot, worktreesDir: worktreesDir}
}

// Checkout creates a git worktree for the given specID and returns its
// absolute path. The returned path is always absolute regardless of whether
// repoRoot or worktreesDir were specified as relative paths, ensuring that
// downstream consumers (Claude CLI, stage committers, etc.) always target
// the worktree and never accidentally operate on the main repo.
func (a *ExecGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
	wtPath := filepath.Join(a.worktreesDir, specID)
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return "", fmt.Errorf("creating worktrees dir: %w", err)
	}

	// If a worktree already exists (e.g. preserved from a previous failed run),
	// remove it before creating a fresh one.
	if _, err := os.Stat(wtPath); err == nil {
		if removeErr := a.removeExistingWorktree(ctx, wtPath); removeErr != nil {
			return "", removeErr
		}
	}

	branchName := "gromit/spec/" + specID
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-B", branchName, wtPath, "HEAD")
	cmd.Dir = a.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", out, err)
	}

	// Always return an absolute path so that all downstream git operations
	// (cmd.Dir, stage commits, Claude CLI working directory) unambiguously
	// target the worktree regardless of CWD changes.
	absPath, err := filepath.Abs(wtPath)
	if err != nil {
		return "", fmt.Errorf("resolving absolute worktree path: %w", err)
	}

	return absPath, nil
}

// Diff returns the current diff for the provided worktree.
func (a *ExecGitAdapter) Diff(ctx context.Context, worktree string) (string, error) {
	if strings.TrimSpace(worktree) == "" {
		return "", fmt.Errorf("worktree required")
	}
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD")
	cmd.Dir = worktree

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff: %s: %w", out, err)
	}
	return string(out), nil
}

func (a *ExecGitAdapter) Commit(ctx context.Context, worktree, message string) (string, error) {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return "", fmt.Errorf("worktree required")
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", fmt.Errorf("commit message required")
	}
	if out, err := runGitCommand(ctx, trimmed, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %s: %w", out, err)
	}
	if out, err := runGitCommand(ctx, trimmed, "commit", "-m", msg); err != nil {
		return "", fmt.Errorf("git commit: %s: %w", out, err)
	}
	out, err := runGitCommand(ctx, trimmed, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %s: %w", out, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (a *ExecGitAdapter) RemoveWorktree(ctx context.Context, worktree string) error {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return fmt.Errorf("worktree required")
	}
	return a.removeExistingWorktree(ctx, trimmed)
}

func (a *ExecGitAdapter) Log(ctx context.Context, worktree string, n int) ([]adapter.LogEntry, error) {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return nil, fmt.Errorf("worktree required")
	}
	if n <= 0 {
		return nil, fmt.Errorf("n must be positive, got %d", n)
	}
	out, err := runGitCommand(ctx, trimmed, "log", "--format=%H%x00%s", "-"+strconv.Itoa(n))
	if err != nil {
		return nil, fmt.Errorf("git log: %s: %w", out, err)
	}
	var entries []adapter.LogEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		entries = append(entries, adapter.LogEntry{Hash: parts[0], Message: parts[1]})
	}
	return entries, nil
}

func (a *ExecGitAdapter) SquashCommits(ctx context.Context, worktree string, count int) error {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return fmt.Errorf("worktree required")
	}
	if count <= 0 {
		return fmt.Errorf("count must be positive, got %d", count)
	}
	ref := "HEAD~" + strconv.Itoa(count)
	out, err := runGitCommand(ctx, trimmed, "reset", "--soft", ref)
	if err != nil {
		return fmt.Errorf("git reset --soft: %s: %w", out, err)
	}
	return nil
}

func (a *ExecGitAdapter) Show(ctx context.Context, worktree, hash string) (string, error) {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return "", fmt.Errorf("worktree required")
	}
	if strings.TrimSpace(hash) == "" {
		return "", fmt.Errorf("hash required")
	}
	out, err := runGitCommand(ctx, trimmed, "show", hash)
	if err != nil {
		return "", fmt.Errorf("git show: %s: %w", out, err)
	}
	return string(out), nil
}

func (a *ExecGitAdapter) Status(ctx context.Context, worktree string) (string, error) {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return "", fmt.Errorf("worktree required")
	}
	out, err := runGitCommand(ctx, trimmed, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git status --porcelain: %s: %w", out, err)
	}
	return string(out), nil
}

// removeExistingWorktree removes a pre-existing worktree directory, falling back
// to manual removal + prune when git worktree remove fails (e.g. orphaned
// worktrees with read-only Go module cache files).
func (a *ExecGitAdapter) removeExistingWorktree(ctx context.Context, wtPath string) error {
	rmCmd := exec.CommandContext(ctx, "git", "worktree", "remove", wtPath)
	rmCmd.Dir = a.repoRoot
	if _, err := rmCmd.CombinedOutput(); err == nil {
		return nil
	}

	forceCmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", wtPath)
	forceCmd.Dir = a.repoRoot
	if _, err := forceCmd.CombinedOutput(); err == nil {
		return nil
	}

	// Fallback: manually remove the directory and prune the worktree registry.
	// Go module caches inside worktrees have read-only permissions that prevent
	// git's internal removal from succeeding.
	_ = filepath.Walk(wtPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // best-effort: skip inaccessible entries
		}
		if info.Mode().Perm()&0200 == 0 {
			_ = os.Chmod(path, info.Mode()|0200)
		}
		return nil
	})
	if err := os.RemoveAll(wtPath); err != nil {
		return fmt.Errorf("failed to remove orphaned worktree %s: %w", wtPath, err)
	}

	pruneCmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	pruneCmd.Dir = a.repoRoot
	if out, err := pruneCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("directory removed but git worktree prune failed: %s: %w", out, err)
	}
	return nil
}
