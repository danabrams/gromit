package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/procutil"
)

const (
	interactiveBranchName   = "gromit/interactive"
	interactiveWorktreeSufx = "-gromit-interactive"
	branchPrefix            = "gromit/"
	maxSessionCreateRetries = 5
	worktreeProcessCapacity = 1500 * time.Millisecond
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
	ctx         context.Context
}

// Option is a function that configures a Manager.
type Option func(*Manager)

// WithGitRunFn sets a custom git execution function for testing.
func WithGitRunFn(fn GitRunFn) Option {
	return func(m *Manager) {
		m.gitRunFn = fn
	}
}

// WithContext sets the context used by git commands run by the Manager.
// If nil, context.Background() is used.
func WithContext(ctx context.Context) Option {
	return func(m *Manager) {
		if ctx != nil {
			m.ctx = ctx
		}
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

func (m *Manager) gitContext() context.Context {
	if m == nil || m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

func (m *Manager) contextFor(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return m.gitContext()
}

// EnsureWorktree creates the worktree if it doesn't exist, or verifies it's
// healthy if it does. Returns the worktree path.
func (m *Manager) EnsureWorktree(ctx context.Context) (string, error) {
	if m == nil {
		return "", errors.New("nil Manager receiver")
	}

	ctx = m.contextFor(ctx)

	// Check if our specific worktree already exists by listing all worktrees
	// and checking if ours is in the list
	output, err := m.runGit(ctx, m.MainDir, "worktree", "list")
	if err == nil {
		// Parse the worktree list to see if our worktree exists
		// git worktree list outputs one worktree per line with the path as the first column
		if containsPath(output, m.WorktreeDir) {
			return m.WorktreeDir, nil
		}
	}

	// Create the worktree with a new branch
	_, err = m.runGit(ctx, m.MainDir, "worktree", "add", m.WorktreeDir, "-b", interactiveBranchName)
	if err != nil {
		// Branch may already exist from a previous worktree that was removed.
		// Retry without -b to checkout the existing branch.
		_, err = m.runGit(ctx, m.MainDir, "worktree", "add", m.WorktreeDir, interactiveBranchName)
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
func (m *Manager) CreateBranch(ctx context.Context, command string) (string, error) {
	if m == nil {
		return "", errors.New("nil Manager receiver")
	}
	if command == "" {
		return "", errors.New("command cannot be empty")
	}

	ctx = m.contextFor(ctx)

	// Generate branch name with timestamp
	timestamp := time.Now().Unix()
	branchName := sessionBranchName(command, timestamp)

	// Create and checkout the branch in the worktree directory
	_, err := m.runGit(ctx, m.WorktreeDir, "checkout", "-b", branchName)
	if err != nil {
		return "", fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}

	return branchName, nil
}

// CreateSessionWorktree creates a session-specific worktree with unique branch and path names.
// Returns a SessionWorktree containing the branch name and worktree directory path.
func (m *Manager) CreateSessionWorktree(ctx context.Context, command string) (*SessionWorktree, error) {
	if m == nil {
		return nil, errors.New("nil Manager receiver")
	}
	if command == "" {
		return nil, errors.New("command cannot be empty")
	}

	ctx = m.contextFor(ctx)

	baseTimestamp := sessionTimestampFn()
	var lastErr error
	for attempt := int64(0); attempt < maxSessionCreateRetries; attempt++ {
		timestamp := baseTimestamp + attempt
		branchName := sessionBranchName(command, timestamp)
		worktreeDir := sessionWorktreeDir(m.MainDir, command, timestamp)

		output, err := m.runGit(ctx, m.MainDir, "worktree", "add", worktreeDir, "-b", branchName)
		if err == nil {
			return &SessionWorktree{
				BranchName:  branchName,
				WorktreeDir: worktreeDir,
			}, nil
		}
		lastErr = err

		decision := decideSessionCreateRetry(sessionCreateRetryInput{
			FailureClass: classifySessionCreateFailure(output, err),
			ProbeBranchExists: func() bool {
				return m.sessionBranchExists(ctx, branchName)
			},
			ProbeWorktreeRegistered: func() bool {
				return m.sessionWorktreeRegistered(ctx, worktreeDir)
			},
		})
		if decision.Retry {
			continue
		}
		if decision.TerminalReason != "" {
			return nil, fmt.Errorf("failed to create session worktree (%s): %w", decision.TerminalReason, err)
		}
		return nil, fmt.Errorf("failed to create session worktree: %w", err)
	}

	return nil, fmt.Errorf("failed to create session worktree after retries: %w", lastErr)
}

// PendingBranches returns branches created by interactive sessions
// that haven't been merged yet (branches matching gromit/* pattern).
func (m *Manager) PendingBranches(ctx context.Context) ([]string, error) {
	if m == nil {
		return nil, errors.New("nil Manager receiver")
	}

	ctx = m.contextFor(ctx)
	checkedOut := m.checkedOutBranchesByWorktree(ctx)

	// List all branches matching gromit/* pattern
	output, err := m.runGit(ctx, m.MainDir, "for-each-ref", "--format=%(refname)", "refs/heads/gromit/")
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
				if _, isCheckedOut := checkedOut[branchName]; isCheckedOut {
					continue
				}
				branches = append(branches, branchName)
			}
		}
	}

	return branches, nil
}

func (m *Manager) checkedOutBranchesByWorktree(ctx context.Context) map[string]struct{} {
	ctx = m.contextFor(ctx)
	output, err := m.runGit(ctx, m.MainDir, "worktree", "list", "--porcelain")
	if err != nil || strings.TrimSpace(output) == "" {
		return map[string]struct{}{}
	}

	checkedOut := make(map[string]struct{})
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "branch refs/heads/") {
			continue
		}
		branch := strings.TrimPrefix(trimmed, "branch refs/heads/")
		if branch == "" {
			continue
		}
		checkedOut[branch] = struct{}{}
	}
	return checkedOut
}

// MergeBack merges a completed interactive branch into the current
// branch of the main worktree. Attempts fast-forward first, falls back
// to merge commit if needed. Aborts on conflict and returns error.
// Deletes the branch on successful merge.
func (m *Manager) MergeBack(ctx context.Context, branch string) error {
	if m == nil {
		return errors.New("nil Manager receiver")
	}
	if branch == "" {
		return errors.New("branch name cannot be empty")
	}

	ctx = m.contextFor(ctx)

	// Try fast-forward merge first
	_, err := m.runGit(ctx, m.MainDir, "merge", "--ff-only", branch)
	if err == nil {
		// Fast-forward successful, delete the branch
		if _, deleteErr := m.runGit(ctx, m.MainDir, "branch", "-d", branch); deleteErr != nil {
			return fmt.Errorf("delete merged branch %s: %w", branch, deleteErr)
		}
		return nil
	}

	// Fast-forward failed, try regular merge
	mergeInProgressBefore := m.mergeInProgress(ctx)
	output, err := m.runGit(ctx, m.MainDir, "merge", branch)
	if err != nil {
		mergeInProgressAfter := m.mergeInProgress(ctx)
		decision := classifyMergeFailure(mergeFailureInput{
			Output:          output,
			Err:             err,
			MergeInProgress: mergeInProgressAfter && !mergeInProgressBefore,
		})
		if decision.Class == mergeFailureConflict {
			// Merge failed with conflict, abort merge state.
			_, _ = m.runGit(ctx, m.MainDir, "merge", "--abort")
			return fmt.Errorf("merge conflict for branch %s: %w", branch, err)
		}
		if decision.ExitCodeKnown {
			return fmt.Errorf("merge failed for branch %s (exit %d): %w", branch, decision.ExitCode, err)
		}
		return fmt.Errorf("merge failed for branch %s: %w", branch, err)
	}

	// Regular merge successful, delete the branch
	if _, deleteErr := m.runGit(ctx, m.MainDir, "branch", "-d", branch); deleteErr != nil {
		return fmt.Errorf("delete merged branch %s: %w", branch, deleteErr)
	}
	return nil
}

// Cleanup removes the worktree and prunes stale branches.
// If the worktree doesn't exist, Cleanup succeeds without error.
func (m *Manager) Cleanup() error {
	if m == nil {
		return errors.New("nil Manager receiver")
	}

	ctx := m.gitContext()

	// Remove the worktree. Ignore errors since worktree may not exist.
	// This is intentionally lenient - cleanup should not fail if there's
	// nothing to clean up.
	_, _ = m.runGit(ctx, m.MainDir, "worktree", "remove", m.WorktreeDir)

	return nil
}

// RemoveByPath removes a session worktree by explicit path.
// Verifies the path is registered using git worktree list --porcelain
// before issuing git worktree remove.
func (m *Manager) RemoveByPath(path string) error {
	if m == nil {
		return errors.New("nil Manager receiver")
	}
	if path == "" {
		return errors.New("path cannot be empty")
	}

	ctx := m.gitContext()

	// Verify the path is registered in the worktree list
	output, err := m.runGit(ctx, m.MainDir, "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	if !strings.Contains(output, "\nworktree "+path+"\n") &&
		!strings.HasPrefix(output, "worktree "+path+"\n") {
		return fmt.Errorf("worktree path not found in registry: %s", path)
	}

	// Remove the worktree
	_, err = m.runGit(ctx, m.MainDir, "worktree", "remove", path)
	if err != nil {
		return fmt.Errorf("failed to remove worktree at %s: %w", path, err)
	}

	return nil
}

// runGit executes a git command in the specified directory.
func (m *Manager) runGit(ctx context.Context, dir string, args ...string) (string, error) {
	if m.gitRunFn != nil {
		return m.gitRunFn(dir, args...)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// Default implementation: run real git command
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	procutil.SetProcessGroupKill(cmd)
	cmd.Env = procutil.SubprocessEnv()

	if waitErr := procutil.WaitForProcessCapacity(ctx, worktreeProcessCapacity); waitErr != nil {
		return "", fmt.Errorf("waiting for process capacity: %w", waitErr)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	defer procutil.ReapProcessTree(cmd)
	if err := cmd.Wait(); err != nil {
		out := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		if out == "" {
			return "", err
		}
		return out, err
	}
	return stdout.String(), nil
}

func sessionBranchName(command string, timestamp int64) string {
	return fmt.Sprintf("%s%s-%d", branchPrefix, command, timestamp)
}

func sessionWorktreeDir(mainDir, command string, timestamp int64) string {
	return fmt.Sprintf("%s-gromit-%s-%d", mainDir, command, timestamp)
}

func (m *Manager) sessionBranchExists(ctx context.Context, branchName string) bool {
	ref := "refs/heads/" + branchName
	ctx = m.contextFor(ctx)
	_, err := m.runGit(ctx, m.MainDir, "show-ref", "--verify", "--quiet", ref, "--")
	return err == nil
}

func (m *Manager) sessionWorktreeRegistered(ctx context.Context, worktreeDir string) bool {
	ctx = m.contextFor(ctx)
	output, err := m.runGit(ctx, m.MainDir, "worktree", "list", "--porcelain")
	if err != nil {
		return false
	}
	return strings.Contains(output, "\nworktree "+worktreeDir+"\n") ||
		strings.HasPrefix(output, "worktree "+worktreeDir+"\n")
}

func (m *Manager) mergeInProgress(ctx context.Context) bool {
	ctx = m.contextFor(ctx)
	output, err := m.runGit(ctx, m.MainDir, "rev-parse", "--verify", "MERGE_HEAD")
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) != ""
}
