package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/worktree"
)

type sessionWorktreeCreator interface {
	CreateSessionWorktree(ctx context.Context, command string) (*worktree.SessionWorktree, error)
	MergeBack(ctx context.Context, branch string) error
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
	sessionQueueLane               = "code_lane"
	sessionQueueReadyState         = integrationqueue.StateReady
	sessionQueueBlockedState       = integrationqueue.StateConflict
	sessionQueueCommittedReason    = "session_committed"
	sessionQueueCommitFailedReason = "session_commit_failed"
	sessionQueueCommitFailedCode   = "session_commit_failed"
	sessionAutoCommitMessage       = "gromit session commit"
)

type sessionQueueStore interface {
	Save(entry integrationqueue.Entry) error
	Delete(branch string) error
}

var (
	_ sessionQueueStore = (*integrationqueue.Store)(nil)
)

var interactiveWorktreeNewQueueStoreFn = func(gromitDir string) (sessionQueueStore, error) {
	return integrationqueue.NewStore(gromitDir)
}

var interactiveWorktreeGitRunFn = func(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err
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

	session, err := manager.CreateSessionWorktree(context.TODO(), command)
	if err != nil {
		return nil, fmt.Errorf("creating session worktree: %w", err)
	}

	// Create a draft queue entry for the session at startup
	if err := enqueueDraftBranch(gromitDir, command, session); err != nil {
		return nil, fmt.Errorf("creating draft queue entry: %w", err)
	}

	if err := callback(session.WorktreeDir); err != nil {
		return nil, fmt.Errorf("running session callback: %w", err)
	}

	stateFile, err := interactiveWorktreeNewStateFileFn(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("creating interactive state file: %w", err)
	}

	commitMeta, commitErr := autoCommitSessionWorktree(session.WorktreeDir, session.BranchName)
	if commitErr != nil {
		blockedMeta := gatherWorkingTreeSnapshot(session.WorktreeDir)
		queueErr := enqueueBlockedBranch(gromitDir, command, session, blockedMeta, commitErr)
		if err := stateFile.AddPendingWorktreeBranch(session.BranchName); err != nil {
			return session, fmt.Errorf("recording pending worktree branch %q: %w", session.BranchName, err)
		}
		if err := interactiveWorktreeCleanupSessionFn(mainDir, session.WorktreeDir); err != nil {
			return session, fmt.Errorf("removing session worktree for blocked branch %q: %w", session.BranchName, err)
		}
		finalErr := fmt.Errorf("auto-commit failed for branch %q: %w", session.BranchName, commitErr)
		if queueErr != nil {
			finalErr = fmt.Errorf("%w (queue record error: %v)", finalErr, queueErr)
		}
		return session, finalErr
	}
	if commitMeta == nil {
		if err := removeQueueBranch(gromitDir, session.BranchName); err != nil {
			return session, fmt.Errorf("removing draft queue entry for no-op branch %q: %w", session.BranchName, err)
		}
		if err := interactiveWorktreeCleanupSessionFn(mainDir, session.WorktreeDir); err != nil {
			return session, fmt.Errorf("removing session worktree for no-op branch %q: %w", session.BranchName, err)
		}
		return session, nil
	}

	if err := enqueueReadyBranch(gromitDir, command, session, commitMeta); err != nil {
		return session, fmt.Errorf("queueing session branch %q: %w", session.BranchName, err)
	}
	if err := stateFile.AddPendingWorktreeBranch(session.BranchName); err != nil {
		return session, fmt.Errorf("recording pending worktree branch %q: %w", session.BranchName, err)
	}

	if err := interactiveWorktreeCleanupSessionFn(mainDir, session.WorktreeDir); err != nil {
		return session, fmt.Errorf("removing session worktree for merged branch %q: %w", session.BranchName, err)
	}

	// Single-writer policy: session enqueues branch for coordinator-mediated integration.
	// Do not attempt merge in session path. Branch remains pending for orchestrator processing.
	return session, nil
}

func attemptMergeWithConflictPolicy(
	manager sessionWorktreeCreator,
	stateFile pendingBranchRecorder,
	session *worktree.SessionWorktree,
	conflictSettings sessionConflictSettings,
) error {
	mergeErr := manager.MergeBack(context.TODO(), session.BranchName)
	if mergeErr == nil {
		return clearMergedState(session, stateFile)
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
		retryErr := manager.MergeBack(context.TODO(), session.BranchName)
		if retryErr != nil {
			lastMergeErr = retryErr
			continue
		}

		return clearMergedState(session, stateFile)
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

type sessionCommitMetadata struct {
	headSHA          string
	baseRef          string
	changedFiles     []string
	changedFilesHash string
}

func baseRefFromMeta(meta *sessionCommitMetadata) string {
	if meta == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(meta.baseRef); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(meta.headSHA)
}

func autoCommitSessionWorktree(sessionDir, branch string) (*sessionCommitMetadata, error) {
	if _, err := interactiveWorktreeGitRunFn(sessionDir, "add", "-A"); err != nil {
		return nil, fmt.Errorf("staging session changes: %w", err)
	}
	hasChanges, err := sessionWorktreeHasStagedChanges(sessionDir)
	if err != nil {
		return nil, err
	}
	if !hasChanges {
		return nil, nil
	}

	commitMsg := fmt.Sprintf("%s %s", sessionAutoCommitMessage, branch)
	if _, err := interactiveWorktreeGitRunFn(sessionDir, "commit", "--allow-empty", "-m", commitMsg); err != nil {
		return nil, fmt.Errorf("committing session branch %s: %w", branch, err)
	}

	return describeSessionCommit(sessionDir)
}

func sessionWorktreeHasStagedChanges(sessionDir string) (bool, error) {
	_, err := interactiveWorktreeGitRunFn(sessionDir, "diff", "--cached", "--quiet")
	if err != nil {
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("checking staged changes: %w", err)
	}

	porcelain, statusErr := interactiveWorktreeGitRunFn(sessionDir, "status", "--porcelain")
	if statusErr != nil {
		return false, fmt.Errorf("checking staged changes via status: %w", statusErr)
	}
	return strings.TrimSpace(porcelain) != "", nil
}

func describeSessionCommit(sessionDir string) (*sessionCommitMetadata, error) {
	head, err := runGitTrim(sessionDir, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("reading HEAD commit: %w", err)
	}
	parent, err := runGitTrim(sessionDir, "rev-parse", "HEAD^")
	if err != nil {
		return nil, fmt.Errorf("reading parent commit: %w", err)
	}
	changed, err := runGitTrim(sessionDir, "diff", "--name-only", "HEAD^", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("listing changed files: %w", err)
	}
	files := parseChangedFiles(changed)
	return &sessionCommitMetadata{
		headSHA:          head,
		baseRef:          parent,
		changedFiles:     files,
		changedFilesHash: computeChangedFilesHash(files),
	}, nil
}

func enqueueDraftBranch(gromitDir, command string, session *worktree.SessionWorktree) error {
	store, err := interactiveWorktreeNewQueueStoreFn(gromitDir)
	if err != nil {
		return fmt.Errorf("creating integration queue store: %w", err)
	}

	// Get current HEAD as the base reference for the draft entry
	baseRef, err := runGitTrim(session.WorktreeDir, "rev-parse", "HEAD")
	if err != nil {
		// If we can't get HEAD yet (new worktree), use empty string and let validation handle it
		baseRef = ""
	}

	entry := integrationqueue.Entry{
		Branch:        session.BranchName,
		SessionID:     session.BranchName,
		OriginCommand: command,
		State:         integrationqueue.StateDraft,
		Lane:          sessionQueueLane,
		BaseRef:       baseRef,
		HeadSHA:       baseRef, // Same as base for draft; will be updated to actual HEAD after commit
	}
	return store.Save(entry)
}

func enqueueReadyBranch(gromitDir, command string, session *worktree.SessionWorktree, meta *sessionCommitMetadata) error {
	store, err := interactiveWorktreeNewQueueStoreFn(gromitDir)
	if err != nil {
		return fmt.Errorf("creating integration queue store: %w", err)
	}
	entry := integrationqueue.Entry{
		Branch:               session.BranchName,
		SessionID:            session.BranchName,
		OriginCommand:        command,
		State:                sessionQueueReadyState,
		Lane:                 sessionQueueLane,
		AttemptCount:         0,
		RetryCount:           0,
		BaseRef:              baseRefFromMeta(meta),
		HeadSHA:              meta.headSHA,
		ChangedFiles:         meta.changedFiles,
		ChangedFilesHash:     meta.changedFilesHash,
		LastTransitionReason: sessionQueueCommittedReason,
	}
	return store.Save(entry)
}

func enqueueBlockedBranch(gromitDir, command string, session *worktree.SessionWorktree, meta *sessionCommitMetadata, commitErr error) error {
	if meta == nil {
		meta = &sessionCommitMetadata{}
	}
	store, err := interactiveWorktreeNewQueueStoreFn(gromitDir)
	if err != nil {
		return fmt.Errorf("creating integration queue store: %w", err)
	}
	guidance := fmt.Sprintf("Auto-commit failed for branch %q (%v); checkout %s, resolve issues, run `git add -A` and `git commit`, then requeue.", session.BranchName, commitErr, session.BranchName)
	entry := integrationqueue.Entry{
		Branch:               session.BranchName,
		SessionID:            session.BranchName,
		OriginCommand:        command,
		State:                sessionQueueBlockedState,
		Lane:                 sessionQueueLane,
		AttemptCount:         0,
		RetryCount:           0,
		BaseRef:              baseRefFromMeta(meta),
		HeadSHA:              meta.headSHA,
		ChangedFiles:         meta.changedFiles,
		ChangedFilesHash:     meta.changedFilesHash,
		LastTransitionReason: sessionQueueCommitFailedReason,
		LastErrorCode:        sessionQueueCommitFailedCode,
		LastErrorMessage:     guidance,
	}
	return store.Save(entry)
}

func removeQueueBranch(gromitDir, branch string) error {
	store, err := interactiveWorktreeNewQueueStoreFn(gromitDir)
	if err != nil {
		return fmt.Errorf("creating integration queue store: %w", err)
	}
	return store.Delete(branch)
}

func gatherWorkingTreeSnapshot(sessionDir string) *sessionCommitMetadata {
	meta := &sessionCommitMetadata{}
	if head, err := runGitTrim(sessionDir, "rev-parse", "HEAD"); err == nil {
		meta.headSHA = head
		meta.baseRef = head
	}
	meta.changedFiles = collectWorkingTreeChanges(sessionDir)
	meta.changedFilesHash = computeChangedFilesHash(meta.changedFiles)
	return meta
}

func collectWorkingTreeChanges(sessionDir string) []string {
	output, err := runGitTrim(sessionDir, "status", "--porcelain")
	if err != nil {
		return nil
	}
	return parseStatusFiles(output)
}

func parseStatusFiles(output string) []string {
	var files []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		path := fields[len(fields)-1]
		if idx := strings.Index(path, "->"); idx != -1 {
			path = strings.TrimSpace(path[idx+2:])
		}
		if path != "" {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files
}

func parseChangedFiles(output string) []string {
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			files = append(files, trimmed)
		}
	}
	sort.Strings(files)
	return files
}

func computeChangedFilesHash(files []string) string {
	hasher := sha256.New()
	sorted := append([]string{}, files...)
	sort.Strings(sorted)
	for _, file := range sorted {
		hasher.Write([]byte(file))
		hasher.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func runGitTrim(dir string, args ...string) (string, error) {
	output, err := interactiveWorktreeGitRunFn(dir, args...)
	return strings.TrimSpace(output), err
}
