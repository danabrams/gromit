package git

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// PreserveBranch keeps the worktree branch intact after a failure.
// It logs the preservation decision and returns any errors from the branch check.
func (a *ExecGitAdapter) PreserveBranch(ctx context.Context, specID string) error {
	if strings.TrimSpace(specID) == "" {
		return fmt.Errorf("spec ID required")
	}

	branchName := "gromit/spec/" + specID

	// Verify the branch exists
	_, err := runGitCommand(ctx, a.repoRoot, "rev-parse", "--verify", branchName)
	if err != nil {
		log.Printf("WARNING: branch %s not found, preserve skipped", branchName)
		return nil
	}

	log.Printf("Preserved worktree branch %s for spec %s", branchName, specID)
	return nil
}

// DeleteBranch removes the worktree branch after successful completion.
// It logs the deletion decision and returns any errors from the deletion.
func (a *ExecGitAdapter) DeleteBranch(ctx context.Context, specID string) error {
	if strings.TrimSpace(specID) == "" {
		return fmt.Errorf("spec ID required")
	}

	branchName := "gromit/spec/" + specID

	// Verify the branch exists before deleting
	_, err := runGitCommand(ctx, a.repoRoot, "rev-parse", "--verify", branchName)
	if err != nil {
		log.Printf("branch %s not found during deletion attempt", branchName)
		return nil
	}

	// Delete the branch
	if out, err := runGitCommand(ctx, a.repoRoot, "branch", "-D", branchName); err != nil {
		return fmt.Errorf("delete branch %s: %s: %w", branchName, out, err)
	}

	log.Printf("Deleted worktree branch %s for spec %s", branchName, specID)
	return nil
}
