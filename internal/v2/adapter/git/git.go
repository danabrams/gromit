package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/danabrams/gromit/internal/v2/adapter"
)

var _ adapter.GitAdapter = (*ExecGitAdapter)(nil)

// ExecGitAdapter implements GitAdapter using exec.Command("git", ...).
type ExecGitAdapter struct {
	worktreesDir string
}

// NewExecGitAdapter returns an ExecGitAdapter that creates worktrees under worktreesDir.
func NewExecGitAdapter(worktreesDir string) *ExecGitAdapter {
	return &ExecGitAdapter{worktreesDir: worktreesDir}
}

// Checkout creates a git worktree for the given specID and returns its path.
func (a *ExecGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
	wtPath := filepath.Join(a.worktreesDir, specID)
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return "", fmt.Errorf("creating worktrees dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", wtPath, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", out, err)
	}

	return wtPath, nil
}
