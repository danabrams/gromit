package worktree

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	interactiveBranchName   = "gromit/interactive"
	interactiveWorktreeSufx = "-gromit-interactive"
	branchPrefix            = "gromit/"
	maxSessionCreateRetries = 5
)

var sessionTimestampFn = func() int64 {
	return time.Now().UnixNano()
}

// GitRunFn is a function type for executing git commands.
// dir is the working directory, args are the git command arguments.
type GitRunFn func(dir string, args ...string) (string, error)

// SessionWorktree holds the result of creating a session-specific worktree.
type SessionWorktree struct {
	BranchName  string
	WorktreeDir string
}

// Manager manages git worktree lifecycle for interactive commands.
type Manager struct {
	MainDir     string   // path to main worktree (project root)
	WorktreeDir string   // path to interactive worktree
	gitRunFn    GitRunFn // function to run git commands (for testing)
}

// Option is a function that configures a Manager.
type Option func(*Manager)

// WithGitRunFn sets a custom git execution function for testing.
func WithGitRunFn(fn GitRunFn) Option {
	return func(m *Manager) {
		m.gitRunFn = fn
	}
}

// NewManager creates a new Manager for the given main directory.
// The worktree will be created at a sibling directory with "-gromit-interactive" suffix.
func NewManager(mainDir string, opts ...Option) (*Manager, error) {
	if mainDir == "" {
		return nil, errors.New("main directory cannot be empty")
	}

	m := &Manager{
		MainDir:     mainDir,
		WorktreeDir: mainDir + interactiveWorktreeSufx,
	}

	// Apply options
	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}

// EnsureWorktree creates the worktree if it doesn't exist, or verifies it's
// healthy if it does. Returns the worktree path.
func (m *Manager) EnsureWorktree() (string, error) {
	if m == nil {
		return "", errors.New("nil Manager receiver")
	}

	// Check if our specific worktree already exists by listing all worktrees
	// and checking if ours is in the list
	output, err := m.runGit(m.MainDir, "worktree", "list")
	if err == nil {
		// Parse the worktree list to see if our worktree exists
		// git worktree list outputs one worktree per line with the path as the first column
		if containsPath(output, m.WorktreeDir) {
			return m.WorktreeDir, nil
		}
	}

	// Create the worktree with a new branch
	_, err = m.runGit(m.MainDir, "worktree", "add", m.WorktreeDir, "-b", interactiveBranchName)
	if err != nil {
		// Branch may already exist from a previous worktree that was removed.
		// Retry without -b to checkout the existing branch.
		_, err = m.runGit(m.MainDir, "worktree", "add", m.WorktreeDir, interactiveBranchName)
		if err != nil {
			return "", fmt.Errorf("failed to create worktree: %w", err)
		}
	}

	return m.WorktreeDir, nil
}

// containsPath checks if the git worktree list output contains the given path.
// git worktree list format: each line starts with the worktree path followed by a space.
func containsPath(gitWorktreeListOutput, path string) bool {
	for _, line := range strings.Split(gitWorktreeListOutput, "\n") {
		if line == path || strings.HasPrefix(line, path+" ") {
			return true
		}
	}
	return false
}

// CreateBranch creates a new branch in the worktree for an interactive session.
// Branch name: gromit/<command>-<timestamp>
func (m *Manager) CreateBranch(command string) (string, error) {
	if m == nil {
		return "", errors.New("nil Manager receiver")
	}
	if command == "" {
		return "", errors.New("command cannot be empty")
	}

	// Generate branch name with timestamp
	timestamp := time.Now().Unix()
	branchName := sessionBranchName(command, timestamp)

	// Create and checkout the branch in the worktree directory
	_, err := m.runGit(m.WorktreeDir, "checkout", "-b", branchName)
	if err != nil {
		return "", fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}

	return branchName, nil
}

// CreateSessionWorktree creates a session-specific worktree with unique branch and path names.
// Returns a SessionWorktree containing the branch name and worktree directory path.
func (m *Manager) CreateSessionWorktree(command string) (*SessionWorktree, error) {
	if m == nil {
		return nil, errors.New("nil Manager receiver")
	}
	if command == "" {
		return nil, errors.New("command cannot be empty")
	}

	baseTimestamp := sessionTimestampFn()
	var lastErr error
	for attempt := int64(0); attempt < maxSessionCreateRetries; attempt++ {
		timestamp := baseTimestamp + attempt
		branchName := sessionBranchName(command, timestamp)
		worktreeDir := sessionWorktreeDir(m.MainDir, command, timestamp)

		_, err := m.runGit(m.MainDir, "worktree", "add", worktreeDir, "-b", branchName)
		if err == nil {
			return &SessionWorktree{
				BranchName:  branchName,
				WorktreeDir: worktreeDir,
			}, nil
		}
		lastErr = err
		if !isSessionContentionErr(err) {
			return nil, fmt.Errorf("failed to create session worktree: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to create session worktree after retries: %w", lastErr)
}

// PendingBranches returns branches created by interactive sessions
// that haven't been merged yet (branches matching gromit/* pattern).
func (m *Manager) PendingBranches() ([]string, error) {
	if m == nil {
		return nil, errors.New("nil Manager receiver")
	}

	// List all branches matching gromit/* pattern
	output, err := m.runGit(m.MainDir, "for-each-ref", "--format=%(refname)", "refs/heads/gromit/")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	// Parse the output
	if output == "" {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		// Convert refs/heads/gromit/retro-123 to gromit/retro-123
		if strings.HasPrefix(line, "refs/heads/") {
			branchName := strings.TrimPrefix(line, "refs/heads/")
			if strings.HasPrefix(branchName, branchPrefix) {
				if branchName == interactiveBranchName {
					continue
				}
				branches = append(branches, branchName)
			}
		}
	}

	return branches, nil
}

// MergeBack merges a completed interactive branch into the current
// branch of the main worktree. Attempts fast-forward first, falls back
// to merge commit if needed. Aborts on conflict and returns error.
// Deletes the branch on successful merge.
func (m *Manager) MergeBack(branch string) error {
	if m == nil {
		return errors.New("nil Manager receiver")
	}
	if branch == "" {
		return errors.New("branch name cannot be empty")
	}

	// Try fast-forward merge first
	_, err := m.runGit(m.MainDir, "merge", "--ff-only", branch)
	if err == nil {
		// Fast-forward successful, delete the branch
		_, _ = m.runGit(m.MainDir, "branch", "-d", branch)
		return nil
	}

	// Fast-forward failed, try regular merge
	_, err = m.runGit(m.MainDir, "merge", branch)
	if err != nil {
		// Merge failed (likely conflict), abort the merge
		_, _ = m.runGit(m.MainDir, "merge", "--abort")
		return fmt.Errorf("merge conflict for branch %s: %w", branch, err)
	}

	// Regular merge successful, delete the branch
	_, _ = m.runGit(m.MainDir, "branch", "-d", branch)
	return nil
}

// Cleanup removes the worktree and prunes stale branches.
// If the worktree doesn't exist, Cleanup succeeds without error.
func (m *Manager) Cleanup() error {
	if m == nil {
		return errors.New("nil Manager receiver")
	}

	// Remove the worktree. Ignore errors since worktree may not exist.
	// This is intentionally lenient - cleanup should not fail if there's
	// nothing to clean up.
	_, _ = m.runGit(m.MainDir, "worktree", "remove", m.WorktreeDir)

	return nil
}

// runGit executes a git command in the specified directory.
func (m *Manager) runGit(dir string, args ...string) (string, error) {
	if m.gitRunFn != nil {
		return m.gitRunFn(dir, args...)
	}

	// Default implementation: run real git command
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func sessionBranchName(command string, timestamp int64) string {
	return fmt.Sprintf("%s%s-%d", branchPrefix, command, timestamp)
}

func sessionWorktreeDir(mainDir, command string, timestamp int64) string {
	return fmt.Sprintf("%s-gromit-%s-%d", mainDir, command, timestamp)
}

func isSessionContentionErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "a branch named") && strings.Contains(msg, "already exists") {
		return true
	}
	if strings.Contains(msg, "cannot lock ref") &&
		strings.Contains(msg, "refs/heads/") &&
		strings.Contains(msg, "reference already exists") {
		return true
	}
	if strings.Contains(msg, "already checked out") {
		return true
	}
	if strings.Contains(msg, "already used by worktree") {
		return true
	}
	if strings.Contains(msg, "already registered as a worktree") {
		return true
	}
	return false
}
