package main

import (
	"fmt"
	"path/filepath"

	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/worktree"
)

type sessionWorktreeCreator interface {
	CreateSessionWorktree(command string) (*worktree.SessionWorktree, error)
}

type pendingBranchRecorder interface {
	AddPendingWorktreeBranch(branch string) error
}

var (
	_ sessionWorktreeCreator = (*worktree.Manager)(nil)
	_ pendingBranchRecorder  = (*state.InteractiveFile)(nil)
)

var interactiveWorktreeNewManagerFn = func(mainDir string) (sessionWorktreeCreator, error) {
	return worktree.NewManager(mainDir)
}

var interactiveWorktreeNewStateFileFn = func(gromitDir string) (pendingBranchRecorder, error) {
	return state.NewInteractiveFile(gromitDir)
}

// runWithSessionWorktree creates a session worktree, runs callback in that
// session directory context, and records the branch for downstream merge-back.
func runWithSessionWorktree(
	gromitDir string,
	command string,
	callback func(sessionDir string) error,
) (*worktree.SessionWorktree, error) {
	if callback == nil {
		return nil, fmt.Errorf("callback is nil")
	}

	mainDir := filepath.Dir(gromitDir)
	manager, err := interactiveWorktreeNewManagerFn(mainDir)
	if err != nil {
		return nil, fmt.Errorf("creating worktree manager: %w", err)
	}

	session, err := manager.CreateSessionWorktree(command)
	if err != nil {
		return nil, fmt.Errorf("creating session worktree: %w", err)
	}

	if err := callback(session.WorktreeDir); err != nil {
		return nil, fmt.Errorf("running session callback: %w", err)
	}

	stateFile, err := interactiveWorktreeNewStateFileFn(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("creating interactive state file: %w", err)
	}
	if err := stateFile.AddPendingWorktreeBranch(session.BranchName); err != nil {
		return nil, fmt.Errorf("recording pending worktree branch %q: %w", session.BranchName, err)
	}

	return session, nil
}
