package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/danabrams/gromit/internal/worktree"
)

type runWorktreeManager interface {
	CreateSessionWorktree(ctx context.Context, command string) (*worktree.SessionWorktree, error)
	PruneStaleSessionWorktrees(ctx context.Context) (int, error)
}

var (
	_ runWorktreeManager = (*worktree.Manager)(nil)
)

var runWorktreeNewManagerFn = func(mainDir string) (runWorktreeManager, error) {
	return worktree.NewManager(mainDir)
}

var runWorktreeCleanupFn = func(mainDir, worktreeDir, branchName string) {
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreeDir)
	cmd.Dir = mainDir
	_ = cmd.Run()

	cmd = exec.Command("git", "branch", "-D", branchName)
	cmd.Dir = mainDir
	_ = cmd.Run()
}

// runInDedicatedWorktree creates a temporary worktree for the run loop,
// executes fn inside it, and cleans up afterward. This isolates all
// branch switching from the main worktree.
func runInDedicatedWorktree(ctx context.Context, mainDir string, fn func() error) error {
	if mainDir == "" {
		return fmt.Errorf("mainDir cannot be empty")
	}
	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	manager, err := runWorktreeNewManagerFn(mainDir)
	if err != nil {
		return fmt.Errorf("creating worktree manager: %w", err)
	}

	// Prune stale session worktrees from previous killed runs before creating a new one.
	// When a run process is killed (SIGKILL, OOM, terminal close), the defer cleanup
	// doesn't run and stale worktrees persist. These can hold spec branches checked out,
	// blocking subsequent runs from checking out the same branches.
	if pruned, pruneErr := manager.PruneStaleSessionWorktrees(ctx); pruneErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: stale worktree pruning: %v\n", pruneErr)
	} else if pruned > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Pruned %d stale session worktree(s)\n", pruned)
	}

	session, err := manager.CreateSessionWorktree(ctx, "run")
	if err != nil {
		return fmt.Errorf("creating run worktree: %w", err)
	}

	// Use context.Background() for cleanup so a cancelled ctx doesn't block it
	defer runWorktreeCleanupFn(mainDir, session.WorktreeDir, session.BranchName)
	defer func() {
		_ = os.Chdir(origDir)
	}()

	if err := os.Chdir(session.WorktreeDir); err != nil {
		return fmt.Errorf("changing to worktree dir %s: %w", session.WorktreeDir, err)
	}

	return fn()
}
