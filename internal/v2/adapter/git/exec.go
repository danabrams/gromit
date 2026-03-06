package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecGit implements Git using git(1) commands.
type ExecGit struct {
	repoRoot string
}

// NewExecGit returns an ExecGit targeting repoRoot.
func NewExecGit(repoRoot string) *ExecGit {
	return &ExecGit{repoRoot: repoRoot}
}

// CreateWorktree creates a new git worktree rooted under the provided WorktreeRoot.
func (g *ExecGit) CreateWorktree(ctx context.Context, req CreateWorktreeRequest) (CreateWorktreeResponse, error) {
	worktree := filepath.Join(req.WorktreeRoot, req.SpecID)
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		return CreateWorktreeResponse{}, fmt.Errorf("create worktree dir: %w", err)
	}

	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = "HEAD"
	}

	args := []string{"worktree", "add", worktree, reference}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoRoot

	if out, err := cmd.CombinedOutput(); err != nil {
		return CreateWorktreeResponse{}, fmt.Errorf("git worktree add: %s: %w", out, err)
	}

	return CreateWorktreeResponse{Worktree: worktree}, nil
}

func (g *ExecGit) RemoveWorktree(ctx context.Context, req RemoveWorktreeRequest) (RemoveWorktreeResponse, error) {
	worktree := strings.TrimSpace(req.Worktree)
	if worktree == "" {
		return RemoveWorktreeResponse{}, fmt.Errorf("worktree required")
	}

	args := []string{"worktree", "remove"}
	if req.Force {
		args = append(args, "--force")
	}
	args = append(args, worktree)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoRoot

	if out, err := cmd.CombinedOutput(); err != nil {
		return RemoveWorktreeResponse{}, fmt.Errorf("git worktree remove: %s: %w", out, err)
	}

	return RemoveWorktreeResponse{Removed: true}, nil
}

func (g *ExecGit) Commit(ctx context.Context, req CommitRequest) (CommitResponse, error) {
	worktree := strings.TrimSpace(req.Worktree)
	if worktree == "" {
		return CommitResponse{}, fmt.Errorf("worktree required")
	}

	if strings.TrimSpace(req.Message) == "" {
		return CommitResponse{}, fmt.Errorf("commit message required")
	}

	addCmd := exec.CommandContext(ctx, "git", "-C", worktree, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return CommitResponse{}, fmt.Errorf("git add: %s: %w", out, err)
	}

	commitArgs := []string{"-C", worktree, "commit", "-m", req.Message}
	if req.Amend {
		commitArgs = append(commitArgs, "--amend")
	}
	commitCmd := exec.CommandContext(ctx, "git", commitArgs...)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return CommitResponse{}, fmt.Errorf("git commit: %s: %w", out, err)
	}

	revCmd := exec.CommandContext(ctx, "git", "-C", worktree, "rev-parse", "HEAD")
	out, err := revCmd.CombinedOutput()
	if err != nil {
		return CommitResponse{}, fmt.Errorf("git rev-parse: %s: %w", out, err)
	}

	return CommitResponse{CommitHash: strings.TrimSpace(string(out))}, nil
}
