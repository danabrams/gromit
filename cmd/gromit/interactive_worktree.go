package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/worktree"
)

type sessionWorktreeCreator interface {
	CreateSessionWorktree(command string) (*worktree.SessionWorktree, error)
	MergeBack(branch string) error
}

type pendingBranchRecorder interface {
	AddPendingWorktreeBranch(branch string) error
	RemovePendingWorktreeBranch(branch string) error
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

var interactiveWorktreeCleanupSessionFn = func(mainDir, sessionDir string) error {
	cmd := exec.Command("git", "worktree", "remove", sessionDir)
	cmd.Dir = mainDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return fmt.Errorf("removing session worktree %q: %w", sessionDir, err)
		}
		return fmt.Errorf("removing session worktree %q: %w: %s", sessionDir, err, message)
	}
	return nil
}

const (
	conflictPolicyManual = "manual"
	conflictPolicyAgent  = "agent"
	defaultRetryCap      = 3
)

type sessionConflictSettings struct {
	Policy                string
	RetryCap              int
	AgentConflictResolver func(sessionDir, branch string, attempt int) error
}

type mergeConflictHandoffError struct {
	Policy      string
	Branch      string
	SessionDir  string
	RetryCap    int
	MergeErr    error
	ResolverErr error
}

func (e *mergeConflictHandoffError) Error() string {
	if e == nil {
		return ""
	}
	policy := strings.ToLower(strings.TrimSpace(e.Policy))
	if policy == conflictPolicyAgent {
		base := fmt.Sprintf(
			"merge conflict persists for branch %q after %d agent attempt(s); resolve manually in %q and keep pending branch for follow-up",
			e.Branch,
			e.RetryCap,
			e.SessionDir,
		)
		if e.ResolverErr != nil {
			return fmt.Sprintf("%s (last resolver error: %v)", base, e.ResolverErr)
		}
		return base
	}

	return fmt.Sprintf(
		"merge conflict for branch %q; manual handoff required in %q and pending branch retained. "+
			"Run `git -C %s status`, resolve files, then `git -C %s add -A` and `git -C %s commit`. "+
			"If merge metadata exists, run `git -C %s merge --abort` (`git merge --abort`) before retrying.",
		e.Branch,
		e.SessionDir,
		e.SessionDir,
		e.SessionDir,
		e.SessionDir,
		e.SessionDir,
	)
}

func (e *mergeConflictHandoffError) Unwrap() error {
	if e == nil {
		return nil
	}
	if e.ResolverErr != nil {
		return e.ResolverErr
	}
	return e.MergeErr
}

// runWithSessionWorktree creates a session worktree, runs callback in that
// session directory context, and records the branch for downstream merge-back.
func runWithSessionWorktree(
	gromitDir string,
	command string,
	callback func(sessionDir string) error,
) (*worktree.SessionWorktree, error) {
	return runWithSessionWorktreeWithConflictSettings(gromitDir, command, sessionConflictSettings{}, callback)
}

func runWithSessionWorktreeWithConflictSettings(
	gromitDir string,
	command string,
	conflictSettings sessionConflictSettings,
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

	if err := attemptMergeWithConflictPolicy(manager, stateFile, mainDir, session, conflictSettings); err != nil {
		return session, err
	}

	return session, nil
}

func attemptMergeWithConflictPolicy(
	manager sessionWorktreeCreator,
	stateFile pendingBranchRecorder,
	mainDir string,
	session *worktree.SessionWorktree,
	conflictSettings sessionConflictSettings,
) error {
	mergeErr := manager.MergeBack(session.BranchName)
	if mergeErr == nil {
		return clearMergedState(mainDir, session, stateFile)
	}

	policy, normalizeErr := normalizeConflictPolicy(conflictSettings.Policy)
	if normalizeErr != nil {
		return normalizeErr
	}
	if policy != conflictPolicyAgent {
		return newManualConflictHandoffError(session, mergeErr)
	}

	retryCap := normalizeRetryCap(conflictSettings.RetryCap)
	lastMergeErr := mergeErr
	var lastResolverErr error

	for attempt := 1; attempt <= retryCap; attempt++ {
		if conflictSettings.AgentConflictResolver != nil {
			resolveErr := conflictSettings.AgentConflictResolver(session.WorktreeDir, session.BranchName, attempt)
			if resolveErr != nil {
				lastResolverErr = resolveErr
				continue
			}
		}

		lastResolverErr = nil
		mergeErr = manager.MergeBack(session.BranchName)
		if mergeErr != nil {
			lastMergeErr = mergeErr
			continue
		}

		return clearMergedState(mainDir, session, stateFile)
	}

	return newAgentConflictHandoffError(session, retryCap, lastMergeErr, lastResolverErr)
}

func newManualConflictHandoffError(session *worktree.SessionWorktree, mergeErr error) *mergeConflictHandoffError {
	return &mergeConflictHandoffError{
		Policy:     conflictPolicyManual,
		Branch:     session.BranchName,
		SessionDir: session.WorktreeDir,
		MergeErr:   mergeErr,
	}
}

func newAgentConflictHandoffError(
	session *worktree.SessionWorktree,
	retryCap int,
	mergeErr error,
	resolverErr error,
) *mergeConflictHandoffError {
	return &mergeConflictHandoffError{
		Policy:      conflictPolicyAgent,
		Branch:      session.BranchName,
		SessionDir:  session.WorktreeDir,
		RetryCap:    retryCap,
		MergeErr:    mergeErr,
		ResolverErr: resolverErr,
	}
}

func clearMergedState(mainDir string, session *worktree.SessionWorktree, stateFile pendingBranchRecorder) error {
	if err := stateFile.RemovePendingWorktreeBranch(session.BranchName); err != nil {
		return fmt.Errorf("clearing merged pending worktree branch %q: %w", session.BranchName, err)
	}
	if err := interactiveWorktreeCleanupSessionFn(mainDir, session.WorktreeDir); err != nil {
		return err
	}
	return nil
}

func normalizeConflictPolicy(policy string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(policy))
	switch normalized {
	case "", "abort", conflictPolicyManual:
		return conflictPolicyManual, nil
	case conflictPolicyAgent:
		return conflictPolicyAgent, nil
	default:
		return "", fmt.Errorf("invalid conflict resolution policy %q", policy)
	}
}

func normalizeRetryCap(retryCap int) int {
	if retryCap <= 0 {
		return defaultRetryCap
	}
	return retryCap
}

func isMergeConflictHandoffError(err error) bool {
	var handoffErr *mergeConflictHandoffError
	return errors.As(err, &handoffErr)
}

func sessionConflictSettingsFromConfig(cfg *config.Config) sessionConflictSettings {
	if cfg == nil {
		return sessionConflictSettings{}
	}
	return sessionConflictSettings{
		Policy:   cfg.Worktree.ConflictResolution,
		RetryCap: cfg.Worktree.RetryCap,
	}
}
