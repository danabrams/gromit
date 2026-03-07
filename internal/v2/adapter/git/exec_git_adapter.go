package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// Checkout creates a git worktree for the given specID and returns its path.
func (a *ExecGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
	wtPath := filepath.Join(a.worktreesDir, specID)
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return "", fmt.Errorf("creating worktrees dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", wtPath, "HEAD")
	cmd.Dir = a.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", out, err)
	}

	return wtPath, nil
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
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", trimmed)
	cmd.Dir = a.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", out, err)
	}
	return nil
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
