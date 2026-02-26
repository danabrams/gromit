package specbranch

import (
	"context"
	"fmt"
	"os/exec"
)

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
