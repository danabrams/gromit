package specbranch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/procutil"
	"github.com/danabrams/gromit/internal/runner/specmerge"
)

var waitForProcessCapacityFn = procutil.WaitForProcessCapacity
var removeStaleWorktreeFn = removeStaleWorktree

const defaultProcessCapacityWaitTime = 1500 * time.Millisecond

type gitCommandOutput struct {
	stdout string
	stderr string
}

var nonBlockingDirtyWorktreePaths = map[string]struct{}{
	".gromit/integration-queue.json":  {},
	".gromit/LEARNINGS.md":            {},
	".beads/backup/backup_state.json": {},
	".beads/dolt-monitor.pid":         {},
}

// DirtyWorktreeError is returned when the repository has uncommitted changes that block branch switching.
type DirtyWorktreeError struct {
	RepoDir string
	Status  string
}

const dirtyWorktreeGuidance = "Please commit, stash, or clean your working tree before switching branches."

func (e *DirtyWorktreeError) Error() string {
	if e == nil {
		return ""
	}
	status := strings.TrimSpace(e.Status)

	message := fmt.Sprintf("dirty worktree %s", e.RepoDir)
	if status != "" {
		message = fmt.Sprintf("%s: %s", message, status)
	}

	return fmt.Sprintf("%s. %s", message, dirtyWorktreeGuidance)
}

// runGitCommand executes a git command with process capacity waiting and lifecycle handling.
func runGitCommand(ctx context.Context, repoDir string, args ...string) (string, error) {
	output, err := runGitCommandWithOutput(ctx, repoDir, args...)
	if err != nil {
		return strings.TrimSuffix(output.stdout+"\n"+output.stderr, "\n"), err
	}

	return output.stdout, nil
}

func runGitCommandWithOutput(ctx context.Context, repoDir string, args ...string) (gitCommandOutput, error) {
	if err := waitForProcessCapacityFn(ctx, defaultProcessCapacityWaitTime); err != nil {
		return gitCommandOutput{}, fmt.Errorf("waiting for process capacity: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir

	procutil.SetProcessGroupKill(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return gitCommandOutput{}, err
	}

	procutil.KillDescendantsOnCancel(ctx, cmd)
	defer procutil.ReapProcessTree(cmd)

	if err := cmd.Wait(); err != nil {
		return gitCommandOutput{stdout: stdout.String(), stderr: stderr.String()}, err
	}

	return gitCommandOutput{stdout: stdout.String(), stderr: stderr.String()}, nil
}

// GitOps provides git operations for spec branch lifecycle.
type GitOps struct {
	repoDir    string
	baseBranch string
}

// NewGitOps creates a GitOps instance for the given repository directory.
func NewGitOps(repoDir, baseBranch string) *GitOps {
	baseBranch = strings.TrimSpace(baseBranch)
	if baseBranch == "" {
		baseBranch = config.DefaultBaseBranch
	}
	return &GitOps{repoDir: repoDir, baseBranch: baseBranch}
}

// parseWorktreeConflictPath extracts the worktree path from a git error message
// of the form: "fatal: '<branch>' is already used by worktree at '<path>'"
// Returns the path and true if found, or empty string and false otherwise.
func parseWorktreeConflictPath(gitOutput string) (string, bool) {
	const marker = "already used by worktree at '"
	idx := strings.Index(gitOutput, marker)
	if idx < 0 {
		return "", false
	}
	rest := gitOutput[idx+len(marker):]
	endIdx := strings.Index(rest, "'")
	if endIdx < 0 {
		return "", false
	}
	return rest[:endIdx], true
}

// recoverStaleSessionWorktree extracts the conflicting worktree path, validates it,
// and attempts removal if it looks like a stale gromit run session.
func recoverStaleSessionWorktree(ctx context.Context, repoDir, gitOutput string) (bool, string, error) {
	worktreePath, ok := parseWorktreeConflictPath(gitOutput)
	if !ok || !isStaleGromitWorktree(worktreePath) {
		return false, "", nil
	}
	attempted, err := removeStaleWorktreeFn(ctx, repoDir, worktreePath)
	return attempted, worktreePath, err
}

// isStaleGromitWorktree returns true if the worktree path looks like it was
// created by a previous gromit run session (contains "-gromit-run-").
// Excludes interactive worktrees ("-gromit-interactive") which may be active.
func isStaleGromitWorktree(worktreePath string) bool {
	return strings.Contains(worktreePath, "-gromit-run-")
}

// removeStaleWorktree attempts to forcibly remove a stale gromit run worktree
// so the branch it holds can be checked out elsewhere. Only targets worktrees
// matching the gromit-run session pattern. Returns true if removal was
// attempted, along with any error from the removal.
//
// When `git worktree remove --force` fails (e.g., permission denied on orphaned
// worktrees, exit status 255), falls back to manual directory removal followed
// by `git worktree prune` to clean the registry.
func removeStaleWorktree(ctx context.Context, repoDir, worktreePath string) (attempted bool, err error) {
	if !isStaleGromitWorktree(worktreePath) {
		return false, nil
	}
	_, err = runGitCommandWithOutput(ctx, repoDir, "worktree", "remove", "--force", worktreePath)
	if err == nil {
		return true, nil
	}

	// Fallback: manually remove the directory and prune the worktree registry.
	// This handles orphaned worktrees where git's internal removal fails.
	// Go module caches inside worktrees have read-only permissions, so fix
	// permissions before removal.
	_ = chmodWritableRecursive(worktreePath)
	removeErr := os.RemoveAll(worktreePath)
	if removeErr != nil {
		return true, fmt.Errorf("git worktree remove failed: %w; manual removal also failed: %v", err, removeErr)
	}

	// Clean up the now-orphaned registry entry.
	_, pruneErr := runGitCommandWithOutput(ctx, repoDir, "worktree", "prune")
	if pruneErr != nil {
		return true, fmt.Errorf("directory removed but git worktree prune failed: %w", pruneErr)
	}

	return true, nil
}

// chmodWritableRecursive makes all files and directories under root writable by
// the owner. This is needed because Go module caches (and similar) use read-only
// permissions that prevent os.RemoveAll from succeeding.
func chmodWritableRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // best-effort: skip inaccessible entries
		}
		if info.Mode().Perm()&0200 == 0 {
			_ = os.Chmod(path, info.Mode()|0200)
		}
		return nil
	})
}

// CreateOrCheckoutSpecBranch creates a new spec branch or checks out an existing one.
// If the branch already exists, it checks it out. Otherwise, it creates a new branch.
func (g *GitOps) CreateOrCheckoutSpecBranch(ctx context.Context, specBranchName string) error {
	if specBranchName == "" {
		return fmt.Errorf("spec branch name cannot be empty")
	}

	currentBranch, err := g.currentBranch(ctx)
	if err != nil {
		return fmt.Errorf("determine current branch before spec checkout: %w", err)
	}
	if currentBranch == specBranchName {
		return nil
	}

	if err := g.ensureWorktreeClean(ctx); err != nil {
		return fmt.Errorf("spec branch %s blocked by dirty worktree precondition: %w", specBranchName, err)
	}

	// Try to create the branch first
	createOutput, err := runGitCommandWithOutput(ctx, g.repoDir, "checkout", "-b", specBranchName)

	if err == nil {
		return nil
	}

	// Only fall back to plain checkout if the error is "branch already exists".
	// Other failures (dirty working tree, locked index, context cancellation) should
	// be returned immediately so the root cause is not masked.
	combinedOutput := createOutput.stdout + "\n" + createOutput.stderr
	if !strings.Contains(combinedOutput, "already exists") {
		return fmt.Errorf("failed to create spec branch %s: %w (output: %s)", specBranchName, err, formatGitCommandOutput(createOutput))
	}

	// Branch already exists, just checkout
	checkoutOutput, checkoutErr := runGitCommandWithOutput(ctx, g.repoDir, "checkout", specBranchName)
	if checkoutErr == nil {
		return nil
	}

	checkoutCombined := checkoutOutput.stdout + "\n" + checkoutOutput.stderr

	// If checkout fails because non-blocking dirty files (e.g. .gromit/) would be
	// overwritten, stash them, retry checkout, and restore. ensureWorktreeClean already
	// passed, so only non-blocking paths are dirty.
	if strings.Contains(checkoutCombined, "would be overwritten") {
		if _, stashErr := runGitCommandWithOutput(ctx, g.repoDir, "stash", "push", "-m", "gromit-spec-checkout-auto"); stashErr == nil {
			retryOutput, retryErr := runGitCommandWithOutput(ctx, g.repoDir, "checkout", specBranchName)
			// Best-effort restore of stashed changes; conflicts in non-blocking
			// files are acceptable and won't block the build.
			_, _ = runGitCommandWithOutput(ctx, g.repoDir, "stash", "pop")
			if retryErr == nil {
				return nil
			}
			// Update error context for the final error message.
			checkoutOutput = retryOutput
			checkoutErr = retryErr
			checkoutCombined = retryOutput.stdout + "\n" + retryOutput.stderr
		}
	}

	// If the branch is held by a stale gromit worktree, try to remove it and retry.
	if worktreePath, ok := parseWorktreeConflictPath(checkoutCombined); ok {
		if attempted, removeErr := removeStaleWorktree(ctx, g.repoDir, worktreePath); attempted {
			if removeErr != nil {
				return fmt.Errorf(
					"failed to checkout spec branch %s: branch held by stale worktree %s, removal failed: %w",
					specBranchName, worktreePath, removeErr,
				)
			}
			// Worktree removed successfully — retry checkout once.
			retryOutput, retryErr := runGitCommandWithOutput(ctx, g.repoDir, "checkout", specBranchName)
			if retryErr != nil {
				return fmt.Errorf(
					"failed to checkout spec branch %s after removing stale worktree %s: %w (output: %s)",
					specBranchName, worktreePath, retryErr, formatGitCommandOutput(retryOutput),
				)
			}
			return nil
		}
	}

	return fmt.Errorf(
		"failed to create or checkout spec branch %s: create attempt failed: %v (output: %s); checkout attempt failed: %w (output: %s)",
		specBranchName,
		err,
		formatGitCommandOutput(createOutput),
		checkoutErr,
		formatGitCommandOutput(checkoutOutput),
	)
}

func (g *GitOps) currentBranch(ctx context.Context) (string, error) {
	output, err := runGitCommandWithOutput(ctx, g.repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve current branch: %w", err)
	}
	return strings.TrimSpace(output.stdout), nil
}

func formatGitCommandOutput(output gitCommandOutput) string {
	stdout := strings.TrimSpace(output.stdout)
	stderr := strings.TrimSpace(output.stderr)

	if stdout == "" {
		stdout = "<empty>"
	}
	if stderr == "" {
		stderr = "<empty>"
	}

	return fmt.Sprintf("stdout: %s | stderr: %s", stdout, stderr)
}

// WorktreeStatus returns the porcelain status of the repository worktree.
func WorktreeStatus(ctx context.Context, repoDir string) (string, error) {
	output, err := runGitCommandWithOutput(ctx, repoDir, "status", "--porcelain=1", "--untracked-files=all")
	if err != nil {
		return "", fmt.Errorf("determine worktree status: %w", err)
	}

	return strings.TrimRight(output.stdout, "\n"), nil
}

// EnsureWorktreeClean checks that the repository worktree has no blocking
// uncommitted changes. Returns a *DirtyWorktreeError if dirty.
func (g *GitOps) EnsureWorktreeClean(ctx context.Context) error {
	return g.ensureWorktreeClean(ctx)
}

func (g *GitOps) ensureWorktreeClean(ctx context.Context) error {
	status, err := WorktreeStatus(ctx, g.repoDir)
	if err != nil {
		return err
	}
	blockingStatus := filterBlockingWorktreeStatus(status)
	if blockingStatus == "" {
		return nil
	}
	return &DirtyWorktreeError{
		RepoDir: g.repoDir,
		Status:  blockingStatus,
	}
}

func filterBlockingWorktreeStatus(status string) string {
	if strings.TrimSpace(status) == "" {
		return ""
	}

	lines := strings.Split(status, "\n")
	blocking := make([]string, 0, len(lines))
	for _, line := range lines {
		raw := strings.TrimRight(line, "\r")
		if strings.TrimSpace(raw) == "" {
			continue
		}

		path := parsePorcelainStatusPath(raw)
		if path != "" {
			if isNonBlockingDirtyWorktreePath(path) {
				continue
			}
		}

		blocking = append(blocking, strings.TrimSpace(raw))
	}

	return strings.Join(blocking, "\n")
}

func isNonBlockingDirtyWorktreePath(path string) bool {
	if _, ok := nonBlockingDirtyWorktreePaths[path]; ok {
		return true
	}

	// The .gromit/ directory contains infrastructure artifacts (plans, specs, reports,
	// templates, experiments, etc.) that may be edited in parallel with the runner.
	// None of these are source code, so they should not block branch checkout.
	if strings.HasPrefix(path, ".gromit/") {
		return true
	}
	if strings.Contains(path, "/.gromit/") {
		return true
	}

	return false
}

func parsePorcelainStatusPath(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}

	path := ""
	switch {
	case len(line) >= 3 && line[2] == ' ':
		path = strings.TrimSpace(line[3:])
	case len(trimmed) >= 2 && trimmed[1] == ' ':
		path = strings.TrimSpace(trimmed[2:])
	default:
		fields := strings.Fields(trimmed)
		if len(fields) > 1 {
			path = fields[len(fields)-1]
		}
	}

	if path == "" {
		return ""
	}

	if idx := strings.Index(path, " -> "); idx >= 0 {
		path = strings.TrimSpace(path[idx+4:])
	}

	path = strings.Trim(path, "\"")
	return path
}

// RebaseOnto rebases the branch onto the specified target branch.
// Returns a ConflictError if a rebase conflict occurs.
// Implements the GitOps interface from specmerge package.
func (g *GitOps) RebaseOnto(ctx context.Context, branch, onto string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if onto == "" {
		return fmt.Errorf("target branch cannot be empty")
	}

	output, err := runGitCommand(ctx, g.repoDir, "rebase", onto, branch)

	if err == nil {
		return nil
	}

	// Check if this is a rebase conflict
	if isRebaseConflict(output, err) {
		// Abort the rebase to clean up state.
		// Use context.Background() so cleanup completes even if the parent context is cancelled.
		_, _ = runGitCommand(context.Background(), g.repoDir, "rebase", "--abort")

		return &specmerge.ConflictError{
			Operation: "rebase",
			Err:       err,
		}
	}

	return fmt.Errorf("failed to rebase branch %s onto %s: %w", branch, onto, err)
}

// RebaseSpecOntoMain rebases the spec branch onto the configured base branch.
// Returns a ConflictError if a rebase conflict occurs.
// Convenience method that calls RebaseOnto with the configured base branch as the target.
func (g *GitOps) RebaseSpecOntoMain(ctx context.Context, specBranchName string) error {
	return g.RebaseOnto(ctx, specBranchName, g.baseBranchOrDefault())
}

// FastForwardMerge merges the branch into the current branch using fast-forward only.
// Returns a ConflictError if the merge would result in a conflict.
// Implements the GitOps interface from specmerge package.
func (g *GitOps) FastForwardMerge(ctx context.Context, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Merge with --ff-only
	output, err := runGitCommand(ctx, g.repoDir, "merge", "--ff-only", branch)

	if err == nil {
		return nil
	}

	// Check if this is a merge conflict
	if isMergeConflict(output, err) {
		return &specmerge.ConflictError{
			Operation: "merge",
			Err:       err,
		}
	}

	return fmt.Errorf("failed to fast-forward merge branch %s: %w", branch, err)
}

// FastForwardMergeToMain merges the spec branch into the configured base branch using fast-forward only.
// Returns a ConflictError if the merge would result in a conflict.
// Convenience method that checks out the configured base branch and calls FastForwardMerge.
func (g *GitOps) FastForwardMergeToMain(ctx context.Context, specBranchName string) error {
	if specBranchName == "" {
		return fmt.Errorf("spec branch name cannot be empty")
	}

	// Check out main
	_, err := runGitCommand(ctx, g.repoDir, "checkout", g.baseBranchOrDefault())
	if err != nil {
		return fmt.Errorf("failed to checkout main: %w", err)
	}

	return g.FastForwardMerge(ctx, specBranchName)
}

// DeleteBranch deletes the specified branch.
// Implements the GitOps interface from specmerge package.
func (g *GitOps) DeleteBranch(ctx context.Context, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	_, err := runGitCommand(ctx, g.repoDir, "branch", "-d", branch)

	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branch, err)
	}

	return nil
}

// DeleteSpecBranch deletes the spec branch.
// Convenience method that calls DeleteBranch.
func (g *GitOps) DeleteSpecBranch(ctx context.Context, specBranchName string) error {
	return g.DeleteBranch(ctx, specBranchName)
}

// FinalizeSpecBranch rebases the spec branch onto main, merges it, and deletes it.
func (g *GitOps) FinalizeSpecBranch(ctx context.Context, specBranchName string) error {
	if specBranchName == "" {
		return fmt.Errorf("spec branch name cannot be empty")
	}

	if err := g.RebaseSpecOntoMain(ctx, specBranchName); err != nil {
		return err
	}
	if err := g.FastForwardMergeToMain(ctx, specBranchName); err != nil {
		return err
	}
	if err := g.DeleteSpecBranch(ctx, specBranchName); err != nil {
		return err
	}

	return nil
}

func isRebaseConflict(output string, err error) bool {
	if err == nil {
		return false
	}
	// Check for typical rebase conflict markers in output
	return strings.Contains(output, "CONFLICT")
}

func isMergeConflict(output string, err error) bool {
	if err == nil {
		return false
	}
	// Check for typical merge conflict markers and fast-forward failures
	return strings.Contains(output, "CONFLICT") ||
		strings.Contains(output, "Not possible to fast-forward")
}

// RevertAndReturnToBase reverts all uncommitted changes and checks out the base branch.
// It is a no-op if the worktree is already clean and on the base branch.
func (g *GitOps) RevertAndReturnToBase(ctx context.Context) error {
	// Check current state: is the worktree clean and are we on the base branch?
	status, err := WorktreeStatus(ctx, g.repoDir)
	if err != nil {
		return fmt.Errorf("check worktree status before revert: %w", err)
	}

	currentBranch, err := g.currentBranch(ctx)
	if err != nil {
		return fmt.Errorf("determine current branch before revert: %w", err)
	}

	baseBranch := g.baseBranchOrDefault()

	// No-op if already clean and on the base branch.
	if status == "" && currentBranch == baseBranch {
		return nil
	}

	// Revert all uncommitted tracked file changes.
	if _, err := runGitCommand(ctx, g.repoDir, "checkout", "--", "."); err != nil {
		return fmt.Errorf("revert uncommitted changes: %w", err)
	}

	// Remove untracked files and directories.
	if _, err := runGitCommand(ctx, g.repoDir, "clean", "-fd"); err != nil {
		return fmt.Errorf("remove untracked files: %w", err)
	}

	// Checkout the base branch (skip if already on it).
	if currentBranch != baseBranch {
		if _, err := runGitCommand(ctx, g.repoDir, "checkout", baseBranch); err != nil {
			return fmt.Errorf("checkout base branch %s: %w", baseBranch, err)
		}
	}

	return nil
}

func (g *GitOps) baseBranchOrDefault() string {
	if g == nil || g.baseBranch == "" {
		return config.DefaultBaseBranch
	}
	return g.baseBranch
}
