package specbranch

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ConflictError represents a git operation that failed due to a conflict.
type ConflictError struct {
	Operation string
	Err       error
}

// Error implements the error interface.
func (e *ConflictError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s conflict: %v", e.Operation, e.Err)
}

// Unwrap returns the underlying error.
func (e *ConflictError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// GitOps provides git operations for spec branch lifecycle.
type GitOps struct {
	repoDir string
}

// NewGitOps creates a GitOps instance for the given repository directory.
func NewGitOps(repoDir string) *GitOps {
	return &GitOps{repoDir: repoDir}
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

// RebaseSpecOntoMain rebases the spec branch onto the main branch.
// Returns a ConflictError if a rebase conflict occurs.
func (g *GitOps) RebaseSpecOntoMain(ctx context.Context, specBranchName string) error {
	if specBranchName == "" {
		return fmt.Errorf("spec branch name cannot be empty")
	}

	cmd := exec.CommandContext(ctx, "git", "rebase", "main", specBranchName)
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

		return &ConflictError{
			Operation: "rebase",
			Err:       err,
		}
	}

	return fmt.Errorf("failed to rebase spec branch %s onto main: %w", specBranchName, err)
}

// FastForwardMergeToMain merges the spec branch into main using fast-forward only.
// Returns a ConflictError if the merge would result in a conflict.
func (g *GitOps) FastForwardMergeToMain(ctx context.Context, specBranchName string) error {
	if specBranchName == "" {
		return fmt.Errorf("spec branch name cannot be empty")
	}

	// Check out main
	cmd := exec.CommandContext(ctx, "git", "checkout", "main")
	cmd.Dir = g.repoDir
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to checkout main: %w", err)
	}

	// Merge with --ff-only
	cmd = exec.CommandContext(ctx, "git", "merge", "--ff-only", specBranchName)
	cmd.Dir = g.repoDir
	output, err := cmd.CombinedOutput()

	if err == nil {
		return nil
	}

	// Check if this is a merge conflict
	if isMergeConflict(string(output), err) {
		return &ConflictError{
			Operation: "merge",
			Err:       err,
		}
	}

	return fmt.Errorf("failed to fast-forward merge spec branch %s to main: %w", specBranchName, err)
}

func isRebaseConflict(output string, err error) bool {
	if err == nil {
		return false
	}
	// Check for typical rebase conflict markers in output
	return strings.Contains(output, "CONFLICT") || strings.Contains(output, "conflict")
}

// DeleteSpecBranch deletes the spec branch.
func (g *GitOps) DeleteSpecBranch(ctx context.Context, specBranchName string) error {
	if specBranchName == "" {
		return fmt.Errorf("spec branch name cannot be empty")
	}

	cmd := exec.CommandContext(ctx, "git", "branch", "-d", specBranchName)
	cmd.Dir = g.repoDir
	_, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("failed to delete spec branch %s: %w", specBranchName, err)
	}

	return nil
}

func isMergeConflict(output string, err error) bool {
	if err == nil {
		return false
	}
	// Check for typical merge conflict markers in output
	return strings.Contains(output, "CONFLICT") || strings.Contains(output, "conflict") || strings.Contains(output, "Merge made by")
}
