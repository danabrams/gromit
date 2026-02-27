package specbranch

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/specmerge"
)

// GitOps provides git operations for spec branch lifecycle.
type GitOps struct {
	repoDir    string
	baseBranch string
}

// NewGitOps creates a GitOps instance for the given repository directory.
func NewGitOps(repoDir, baseBranch string) *GitOps {
	baseBranch = strings.TrimSpace(baseBranch)
	if baseBranch == "" {
		baseBranch = config.DefaultBaseBranch
	}
	return &GitOps{repoDir: repoDir, baseBranch: baseBranch}
}

// CreateOrCheckoutSpecBranch creates a new spec branch or checks out an existing one.
// If the branch already exists, it checks it out. Otherwise, it creates a new branch.
func (g *GitOps) CreateOrCheckoutSpecBranch(ctx context.Context, specBranchName string) error {
	if specBranchName == "" {
		return fmt.Errorf("spec branch name cannot be empty")
	}

	// Try to create the branch first
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", specBranchName)
	cmd.Dir = g.repoDir
	_, err := cmd.CombinedOutput()

	if err == nil {
		return nil
	}

	// If branch exists, just checkout
	cmd = exec.CommandContext(ctx, "git", "checkout", specBranchName)
	cmd.Dir = g.repoDir
	_, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create or checkout spec branch %s: %w", specBranchName, err)
	}

	return nil
}

// RebaseOnto rebases the branch onto the specified target branch.
// Returns a ConflictError if a rebase conflict occurs.
// Implements the GitOps interface from specmerge package.
func (g *GitOps) RebaseOnto(ctx context.Context, branch, onto string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if onto == "" {
		return fmt.Errorf("target branch cannot be empty")
	}

	cmd := exec.CommandContext(ctx, "git", "rebase", onto, branch)
	cmd.Dir = g.repoDir
	output, err := cmd.CombinedOutput()

	if err == nil {
		return nil
	}

	// Check if this is a rebase conflict
	if isRebaseConflict(string(output), err) {
		// Abort the rebase to clean up state
		abortCmd := exec.CommandContext(ctx, "git", "rebase", "--abort")
		abortCmd.Dir = g.repoDir
		_ = abortCmd.Run()

		return &specmerge.ConflictError{
			Operation: "rebase",
			Err:       err,
		}
	}

	return fmt.Errorf("failed to rebase branch %s onto %s: %w", branch, onto, err)
}

// RebaseSpecOntoMain rebases the spec branch onto the configured base branch.
// Returns a ConflictError if a rebase conflict occurs.
// Convenience method that calls RebaseOnto with the configured base branch as the target.
func (g *GitOps) RebaseSpecOntoMain(ctx context.Context, specBranchName string) error {
	return g.RebaseOnto(ctx, specBranchName, g.baseBranchOrDefault())
}

// FastForwardMerge merges the branch into the current branch using fast-forward only.
// Returns a ConflictError if the merge would result in a conflict.
// Implements the GitOps interface from specmerge package.
func (g *GitOps) FastForwardMerge(ctx context.Context, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Merge with --ff-only
	cmd := exec.CommandContext(ctx, "git", "merge", "--ff-only", branch)
	cmd.Dir = g.repoDir
	output, err := cmd.CombinedOutput()

	if err == nil {
		return nil
	}

	// Check if this is a merge conflict
	if isMergeConflict(string(output), err) {
		return &specmerge.ConflictError{
			Operation: "merge",
			Err:       err,
		}
	}

	return fmt.Errorf("failed to fast-forward merge branch %s: %w", branch, err)
}

// FastForwardMergeToMain merges the spec branch into the configured base branch using fast-forward only.
// Returns a ConflictError if the merge would result in a conflict.
// Convenience method that checks out the configured base branch and calls FastForwardMerge.
func (g *GitOps) FastForwardMergeToMain(ctx context.Context, specBranchName string) error {
	if specBranchName == "" {
		return fmt.Errorf("spec branch name cannot be empty")
	}

	// Check out main
	cmd := exec.CommandContext(ctx, "git", "checkout", g.baseBranchOrDefault())
	cmd.Dir = g.repoDir
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to checkout main: %w", err)
	}

	return g.FastForwardMerge(ctx, specBranchName)
}

// DeleteBranch deletes the specified branch.
// Implements the GitOps interface from specmerge package.
func (g *GitOps) DeleteBranch(ctx context.Context, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	cmd := exec.CommandContext(ctx, "git", "branch", "-d", branch)
	cmd.Dir = g.repoDir
	_, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branch, err)
	}

	return nil
}

// DeleteSpecBranch deletes the spec branch.
// Convenience method that calls DeleteBranch.
func (g *GitOps) DeleteSpecBranch(ctx context.Context, specBranchName string) error {
	return g.DeleteBranch(ctx, specBranchName)
}

func isRebaseConflict(output string, err error) bool {
	if err == nil {
		return false
	}
	// Check for typical rebase conflict markers in output
	return strings.Contains(output, "CONFLICT") || strings.Contains(output, "conflict")
}

func isMergeConflict(output string, err error) bool {
	if err == nil {
		return false
	}
	// Check for typical merge conflict markers and fast-forward failures
	return strings.Contains(output, "CONFLICT") ||
		strings.Contains(output, "conflict") ||
		strings.Contains(output, "Merge made by") ||
		strings.Contains(output, "Not possible to fast-forward")
}

func (g *GitOps) baseBranchOrDefault() string {
	if g == nil || g.baseBranch == "" {
		return config.DefaultBaseBranch
	}
	return g.baseBranch
}
