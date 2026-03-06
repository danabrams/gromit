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

// NewExecGit returns an ExecGit that executes git commands from repoRoot.
func NewExecGit(repoRoot string) *ExecGit {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		repoRoot = "."
	}
	return &ExecGit{repoRoot: repoRoot}
}

// CreateWorktree creates a new git worktree rooted under the provided WorktreeRoot.
func (g *ExecGit) CreateWorktree(ctx context.Context, req CreateWorktreeRequest) (CreateWorktreeResponse, error) {
	specID := strings.TrimSpace(req.SpecID)
	if specID == "" {
		return CreateWorktreeResponse{}, fmt.Errorf("spec ID required")
	}
	worktreeRoot := strings.TrimSpace(req.WorktreeRoot)
	if worktreeRoot == "" {
		return CreateWorktreeResponse{}, fmt.Errorf("worktree root required")
	}

	worktree := filepath.Join(worktreeRoot, specID)
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		return CreateWorktreeResponse{}, fmt.Errorf("prepare worktree dir: %w", err)
	}

	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = "HEAD"
	}

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", worktree, reference)
	cmd.Dir = g.repoRoot

	if out, err := cmd.CombinedOutput(); err != nil {
		return CreateWorktreeResponse{}, fmt.Errorf("git worktree add: %s: %w", out, err)
	}

	return CreateWorktreeResponse{Worktree: worktree}, nil
}

// RemoveWorktree removes an existing git worktree.
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

// Commit stages all changes in the worktree and records a commit with the provided message.
func (g *ExecGit) Commit(ctx context.Context, req CommitRequest) (CommitResponse, error) {
	worktree := strings.TrimSpace(req.Worktree)
	if worktree == "" {
		return CommitResponse{}, fmt.Errorf("worktree required")
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		return CommitResponse{}, fmt.Errorf("commit message required")
	}

	if _, err := runGitCommand(ctx, worktree, "add", "-A"); err != nil {
		return CommitResponse{}, fmt.Errorf("git add: %w", err)
	}

	args := []string{"commit", "-m", message}
	if req.Amend {
		args = append(args, "--amend")
	}
	if _, err := runGitCommand(ctx, worktree, args...); err != nil {
		return CommitResponse{}, fmt.Errorf("git commit: %w", err)
	}

	out, err := runGitCommand(ctx, worktree, "rev-parse", "HEAD")
	if err != nil {
		return CommitResponse{}, fmt.Errorf("git rev-parse: %w", err)
	}

	return CommitResponse{CommitHash: strings.TrimSpace(string(out))}, nil
}

func runGitCommand(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
