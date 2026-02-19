package main

import (
	"path/filepath"

	"github.com/danabrams/gromit/internal/worktree"
)

// interactiveLaunchDir returns the interactive worktree path when the run loop
// is active, and empty string otherwise. Run-loop activity is determined by
// worktree.IsRunLoopActive using .gromit/status.json PID-backed liveness checks.
func interactiveLaunchDir(gromitDir string) string {
	if !worktree.IsRunLoopActive(gromitDir) {
		return ""
	}
	mainDir := filepath.Dir(gromitDir)
	mgr, err := worktree.NewManager(mainDir)
	if err != nil {
		return ""
	}
	return mgr.WorktreeDir
}
