package present

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/danabrams/gromit/internal/v2/presentation"
)

func cleanupMergedWorktreeBranch(ctx context.Context, worktree, specID, integrationBranch string) error {
	if strings.TrimSpace(worktree) == "" {
		return nil
	}

	specBranch := presentation.SpecBranchName(specID)
	if specBranch == "" {
		return nil
	}

	baseBranch := strings.TrimSpace(integrationBranch)
	if baseBranch == "" {
		baseBranch = presentation.DefaultIntegrationBranch()
	}

	specExists, err := localBranchExists(ctx, worktree, specBranch)
	if err != nil || !specExists {
		return err
	}

	baseExists, err := localBranchExists(ctx, worktree, baseBranch)
	if err != nil {
		return err
	}
	if !baseExists {
		return nil
	}

	merged, err := branchMergedInto(ctx, worktree, specBranch, baseBranch)
	if err != nil {
		return err
	}
	if !merged {
		return nil
	}

	currentBranch, err := currentWorktreeBranch(ctx, worktree)
	if err != nil {
		return err
	}
	if currentBranch == specBranch {
		if err := runGitInWorktree(ctx, worktree, "checkout", "--ignore-other-worktrees", baseBranch); err != nil {
			return fmt.Errorf("checkout integration branch %s: %w", baseBranch, err)
		}
	}
	if err := runGitInWorktree(ctx, worktree, "branch", "-D", specBranch); err != nil {
		return fmt.Errorf("delete merged worktree branch %s: %w", specBranch, err)
	}
	return nil
}

func localBranchExists(ctx context.Context, worktree, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktree, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git show-ref refs/heads/%s: %s: %w", branch, strings.TrimSpace(string(out)), err)
}

func branchMergedInto(ctx context.Context, worktree, branch, target string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktree, "merge-base", "--is-ancestor", branch, target)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %s: %w", branch, target, strings.TrimSpace(string(out)), err)
}
