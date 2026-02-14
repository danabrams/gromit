package worktree

import (
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// GitRunFn is a function type for executing git commands.
// dir is the working directory, args are the git command arguments.
type GitRunFn func(dir string, args ...string) (string, error)

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
		WorktreeDir: mainDir + "-gromit-interactive",
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

	// Check if worktree already exists
	output, err := m.runGit(m.MainDir, "worktree", "list")
	if err == nil && output != "" {
		// Worktree exists, return the path
		return m.WorktreeDir, nil
	}

	// Create the worktree
	_, err = m.runGit(m.MainDir, "worktree", "add", m.WorktreeDir, "-b", "gromit/interactive")
	if err != nil {
		return "", fmt.Errorf("failed to create worktree: %w", err)
	}

	return m.WorktreeDir, nil
}

// CreateBranch creates a new branch in the worktree for an interactive session.
// Branch name: gromit/<command>-<timestamp>
func (m *Manager) CreateBranch(command string) (string, error) {
	if m == nil {
		return "", errors.New("nil Manager receiver")
	}

	// Generate branch name with timestamp
	timestamp := time.Now().Unix()
	branchName := fmt.Sprintf("gromit/%s-%d", command, timestamp)

	// Create and checkout the branch in the worktree directory
	_, err := m.runGit(m.WorktreeDir, "checkout", "-b", branchName)
	if err != nil {
		return "", fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}

	return branchName, nil
}

// Cleanup removes the worktree and prunes stale branches.
func (m *Manager) Cleanup() error {
	if m == nil {
		return errors.New("nil Manager receiver")
	}

	// Remove the worktree
	_, err := m.runGit(m.MainDir, "worktree", "remove", m.WorktreeDir)
	if err != nil {
		// If worktree doesn't exist, that's fine
		return nil
	}

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
