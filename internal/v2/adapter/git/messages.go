package git

import "fmt"

// PreserveBranchMessage reports that a failing worktree will be preserved.
func PreserveBranchMessage(specID, worktree, reason string) string {
	return fmt.Sprintf("preserving worktree %s for spec %s: %s", worktree, specID, reason)
}

// RemoveFailedWorktreeMessage reports that a failed worktree will be removed.
func RemoveFailedWorktreeMessage(specID, worktree, reason string) string {
	return fmt.Sprintf("removing failed worktree %s for spec %s: %s", worktree, specID, reason)
}

// DeleteBranchMessage announces that a worktree branch is being deleted after success.
func DeleteBranchMessage(specID, worktree, reason string) string {
	return fmt.Sprintf("deleting branch/worktree %s for spec %s: %s", worktree, specID, reason)
}

// RemoveWorktreeMessage reports that a worktree is being removed for a successful spec.
func RemoveWorktreeMessage(specID, worktree, reason string) string {
	return fmt.Sprintf("removing worktree %s for spec %s: %s", worktree, specID, reason)
}
