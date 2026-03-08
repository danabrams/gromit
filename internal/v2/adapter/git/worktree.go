package git

import (
	"fmt"
	"strings"
)

// PreserveBranchMessage formats the log message used when a failed spec branch
// must be kept for inspection.
func PreserveBranchMessage(specID, worktree, reason string) string {
	msg := fmt.Sprintf("preserving failed spec worktree branch for spec %s at %s", specID, worktree)
	return withReason(msg, reason)
}

// DeleteBranchMessage formats the log message for deleting a managed spec branch
// after a successful run.
func DeleteBranchMessage(specID, worktree, reason string) string {
	msg := fmt.Sprintf("successfully deleted worktree and branch after successful presentation for spec %s at %s", specID, worktree)
	return withReason(msg, reason)
}

// RemoveFailedWorktreeMessage formats the log message when a failed worktree is
// being removed without preserving the branch.
func RemoveFailedWorktreeMessage(specID, worktree, reason string) string {
	msg := fmt.Sprintf("removing failed spec worktree for spec %s at %s (preserve_on_failure=false)", specID, worktree)
	return withReason(msg, reason)
}

// RemoveWorktreeMessage formats the log message when cleaning up a worktree
// without touching the branch directly.
func RemoveWorktreeMessage(specID, worktree, reason string) string {
	msg := fmt.Sprintf("removing worktree for spec %s at %s", specID, worktree)
	return withReason(msg, reason)
}

func withReason(message, reason string) string {
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		return fmt.Sprintf("%s (%s)", message, trimmed)
	}
	return message
}
