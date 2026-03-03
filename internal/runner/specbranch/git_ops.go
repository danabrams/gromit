package specbranch

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/procutil"
	"github.com/danabrams/gromit/internal/runner/specmerge"
)

var waitForProcessCapacityFn = procutil.WaitForProcessCapacity

const defaultProcessCapacityWaitTime = 1500 * time.Millisecond

type gitCommandOutput struct {
	stdout string
	stderr string
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

// CreateOrCheckoutSpecBranch creates a new spec branch or checks out an existing one.
// If the branch already exists, it checks it out. Otherwise, it creates a new branch.
func (g *GitOps) CreateOrCheckoutSpecBranch(ctx context.Context, specBranchName string) error {
	if specBranchName == "" {
		return fmt.Errorf("spec branch name cannot be empty")
	}

	// Try to create the branch first
	createOutput, err := runGitCommandWithOutput(ctx, g.repoDir, "checkout", "-b", specBranchName)

	if err == nil {
		return nil
	}

	// If branch exists, just checkout
	checkoutOutput, checkoutErr := runGitCommandWithOutput(ctx, g.repoDir, "checkout", specBranchName)
	if checkoutErr != nil {
		return fmt.Errorf(
			"failed to create or checkout spec branch %s: create attempt failed: %v (output: %s); checkout attempt failed: %w (output: %s)",
			specBranchName,
			err,
			formatGitCommandOutput(createOutput),
			checkoutErr,
			formatGitCommandOutput(checkoutOutput),
		)
	}

	return nil
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
		// Abort the rebase to clean up state
		_, _ = runGitCommand(ctx, g.repoDir, "rebase", "--abort")

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
	return strings.Contains(output, "CONFLICT") || strings.Contains(output, "conflict")
}

func isMergeConflict(output string, err error) bool {
	if err == nil {
		return false
	}
	// Check for typical merge conflict markers and fast-forward failures
	return strings.Contains(output, "CONFLICT") ||
		strings.Contains(output, "conflict") ||
		strings.Contains(output, "Merge made by") ||
		strings.Contains(output, "Not possible to fast-forward")
}

func (g *GitOps) baseBranchOrDefault() string {
	if g == nil || g.baseBranch == "" {
		return config.DefaultBaseBranch
	}
	return g.baseBranch
}
