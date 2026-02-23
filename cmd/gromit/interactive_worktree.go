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
			"merge conflict persists for branch %q after %d agent attempt(s); checkout branch %q, resolve manually, and keep pending branch for follow-up",
			e.Branch,
			e.RetryCap,
			e.Branch,
		)
		if e.ResolverErr != nil {
			return fmt.Sprintf("%s (last resolver error: %v)", base, e.ResolverErr)
		}
		return base
	}

	return fmt.Sprintf(
		"merge conflict for branch %q; manual handoff required and pending branch retained. "+
			"checkout branch %q with `git checkout %s`, run `git status`, resolve files, then `git add -A` and `git commit`. "+
			"If merge metadata exists, run `git merge --abort` before retrying.",
		e.Branch,
		e.Branch,
		e.Branch,
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

	cleanedDuringMerge, err := attemptMergeWithConflictPolicy(manager, stateFile, mainDir, session, conflictSettings)
	if err != nil {
		return session, err
	}
	if !cleanedDuringMerge {
		if err := interactiveWorktreeCleanupSessionFn(mainDir, session.WorktreeDir); err != nil {
			return session, fmt.Errorf("removing session worktree for merged branch %q: %w", session.BranchName, err)
		}
	}

	return session, nil
}

func attemptMergeWithConflictPolicy(
	manager sessionWorktreeCreator,
	stateFile pendingBranchRecorder,
	mainDir string,
	session *worktree.SessionWorktree,
	conflictSettings sessionConflictSettings,
) (bool, error) {
	mergeErr, cleanedDuringMerge := mergeBackWithCheckedOutBranchRecovery(manager, mainDir, session)
	if mergeErr == nil {
		return cleanedDuringMerge, clearMergedState(session, stateFile)
	}

	policy, normalizeErr := normalizeConflictPolicy(conflictSettings.Policy)
	if normalizeErr != nil {
		return cleanedDuringMerge, normalizeErr
	}
	if policy != conflictPolicyAgent {
		return cleanedDuringMerge, newManualConflictHandoffError(session, mergeErr)
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
		retryErr, retryCleaned := mergeBackWithCheckedOutBranchRecovery(manager, mainDir, session)
		cleanedDuringMerge = cleanedDuringMerge || retryCleaned
		if retryErr != nil {
			lastMergeErr = retryErr
			continue
		}

		return cleanedDuringMerge, clearMergedState(session, stateFile)
	}

	return cleanedDuringMerge, newAgentConflictHandoffError(session, retryCap, lastMergeErr, lastResolverErr)
}

func mergeBackWithCheckedOutBranchRecovery(
	manager sessionWorktreeCreator,
	mainDir string,
	session *worktree.SessionWorktree,
) (error, bool) {
	mergeErr := manager.MergeBack(session.BranchName)
	if mergeErr == nil {
		return nil, false
	}

	if !isCheckedOutBranchDeleteError(mergeErr) {
		return mergeErr, false
	}

	if err := interactiveWorktreeCleanupSessionFn(mainDir, session.WorktreeDir); err != nil {
		return fmt.Errorf("removing session worktree for merged branch %q before merge retry: %w", session.BranchName, err), false
	}

	return manager.MergeBack(session.BranchName), true
}

func isCheckedOutBranchDeleteError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cannot delete branch") && strings.Contains(msg, "checked out at")
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

func clearMergedState(session *worktree.SessionWorktree, stateFile pendingBranchRecorder) error {
	if err := stateFile.RemovePendingWorktreeBranch(session.BranchName); err != nil {
		return fmt.Errorf("clearing merged pending worktree branch %q: %w", session.BranchName, err)
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
