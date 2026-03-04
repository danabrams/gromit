package specbranch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/test/helpers"
)

// TestCreateOrCheckoutSpecBranch_CreatesNewBranch verifies that CreateOrCheckoutSpecBranch
// creates a new spec branch when it doesn't exist.
func TestCreateOrCheckoutSpecBranch_CreatesNewBranch(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-test-feature"

	// CreateOrCheckoutSpecBranch should create the branch
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v, want nil", err)
	}

	// Verify the branch was created by checking it exists
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+specBranchName)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Errorf("branch %s was not created", specBranchName)
	}

	// Verify we're on the new branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = fixture.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if string(output) != specBranchName+"\n" {
		t.Errorf("not on spec branch: got %q, want %q", string(output), specBranchName+"\n")
	}
}

func TestCreateOrCheckoutSpecBranch_IncludesGitContextOnFailure(t *testing.T) {
	t.Parallel()

	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	err := ops.CreateOrCheckoutSpecBranch(context.Background(), "bad branch name")
	if err == nil {
		t.Fatal("CreateOrCheckoutSpecBranch() expected error for invalid branch name")
	}

	msg := err.Error()
	if !strings.Contains(msg, "failed to create spec branch bad branch name") {
		t.Fatalf("error %q missing create failure context", msg)
	}
	if !strings.Contains(msg, "output:") {
		t.Fatalf("error %q missing git command output context", msg)
	}
	if !strings.Contains(msg, "stdout:") {
		t.Fatalf("error %q missing stdout context", msg)
	}
	if !strings.Contains(msg, "stderr:") {
		t.Fatalf("error %q missing stderr context", msg)
	}
	if !strings.Contains(msg, "fatal: 'bad branch name' is not a valid branch name") {
		t.Fatalf("error %q missing actionable git stderr content", msg)
	}
}

func TestCreateOrCheckoutSpecBranch_DirtyWorktreeReturnsTypedError(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	dirtyFile := filepath.Join(fixture.Dir, fixture.ConflictingFile)
	if err := os.WriteFile(dirtyFile, []byte("dirty change"), 0o644); err != nil {
		t.Fatalf("failed to dirty worktree: %v", err)
	}

	specBranchName := fixture.OurBranch
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err == nil {
		t.Fatal("expected dirty worktree error")
	}

	var dirtyErr *DirtyWorktreeError
	if !errors.As(err, &dirtyErr) {
		t.Fatalf("expected DirtyWorktreeError, got %T: %v", err, err)
	}

	if !strings.Contains(dirtyErr.Status, fixture.ConflictingFile) {
		t.Fatalf("dirty worktree status missing file context: %q", dirtyErr.Status)
	}
}

func TestCreateOrCheckoutSpecBranch_DirtyWorktreeIncludesGuidance(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	dirtyFile := filepath.Join(fixture.Dir, fixture.ConflictingFile)
	if err := os.WriteFile(dirtyFile, []byte("dirty change"), 0o644); err != nil {
		t.Fatalf("failed to dirty worktree: %v", err)
	}

	specBranchName := fixture.OurBranch
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err == nil {
		t.Fatal("expected dirty worktree error")
	}

	var dirtyErr *DirtyWorktreeError
	if !errors.As(err, &dirtyErr) {
		t.Fatalf("expected DirtyWorktreeError, got %T: %v", err, err)
	}

	msg := dirtyErr.Error()
	guidance := "commit, stash, or clean your working tree"
	if !strings.Contains(msg, guidance) {
		t.Fatalf("error %q missing guidance %q", msg, guidance)
	}
}

func TestCreateOrCheckoutSpecBranch_DirtyWorktreeErrorMessageIsActionable(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	dirtyFile := filepath.Join(fixture.Dir, fixture.ConflictingFile)
	if err := os.WriteFile(dirtyFile, []byte("dirty change"), 0o644); err != nil {
		t.Fatalf("failed to dirty worktree: %v", err)
	}

	specBranchName := fixture.OurBranch
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err == nil {
		t.Fatal("expected dirty worktree error")
	}

	var dirtyErr *DirtyWorktreeError
	if !errors.As(err, &dirtyErr) {
		t.Fatalf("expected DirtyWorktreeError, got %T: %v", err, err)
	}

	msg := err.Error()
	if !strings.Contains(msg, specBranchName) {
		t.Fatalf("error %q missing branch context %q", msg, specBranchName)
	}
	if !strings.Contains(msg, dirtyWorktreeGuidance) {
		t.Fatalf("error %q missing guidance %q", msg, dirtyWorktreeGuidance)
	}
	if !strings.Contains(msg, dirtyErr.Status) {
		t.Fatalf("error %q missing worktree status %q", msg, dirtyErr.Status)
	}
}

func TestCreateOrCheckoutSpecBranch_DirtyWorktreeFixtureBlocksCheckout(t *testing.T) {
	fixture := helpers.NewDirtyWorktreeFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-dirty-block"
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err == nil {
		t.Fatal("expected dirty worktree error")
	}

	var dirtyErr *DirtyWorktreeError
	if !errors.As(err, &dirtyErr) {
		t.Fatalf("expected DirtyWorktreeError, got %T: %v", err, err)
	}

	if !strings.Contains(dirtyErr.Status, fixture.DirtyFile) {
		t.Fatalf("dirty worktree status missing file context: %q", dirtyErr.Status)
	}

	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+specBranchName)
	cmd.Dir = fixture.Dir
	if cmd.Run() == nil {
		t.Fatalf("expected spec branch %s not to exist when checkout is blocked", specBranchName)
	}
}

func TestCreateOrCheckoutSpecBranch_AllowsDirtyWorktreeWhenAlreadyOnTargetBranch(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-already-on-target"
	if err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName); err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() initial create error = %v", err)
	}

	dirtyFile := filepath.Join(fixture.Dir, fixture.ConflictingFile)
	if err := os.WriteFile(dirtyFile, []byte("dirty change"), 0o644); err != nil {
		t.Fatalf("failed to dirty worktree: %v", err)
	}

	if err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName); err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() when already on target branch error = %v, want nil", err)
	}
}

func TestCreateOrCheckoutSpecBranch_IgnoresOperationalQueueStateChanges(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	queueFile := filepath.Join(fixture.Dir, ".gromit", "integration-queue.json")
	if err := os.MkdirAll(filepath.Dir(queueFile), 0o755); err != nil {
		t.Fatalf("failed to create queue directory: %v", err)
	}
	if err := os.WriteFile(queueFile, []byte("{\"schema_version\":1}\n"), 0o644); err != nil {
		t.Fatalf("failed to write queue file: %v", err)
	}
	cmd := exec.Command("git", "add", ".gromit/integration-queue.json")
	cmd.Dir = fixture.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to add queue file: %v (%s)", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "add queue file")
	cmd.Dir = fixture.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to commit queue file: %v (%s)", err, out)
	}

	specBranchName := "gromit/spec-queue-ops"
	if err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName); err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() initial create error = %v", err)
	}
	cmd = exec.Command("git", "checkout", fixture.BaseBranch)
	cmd.Dir = fixture.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to checkout base branch: %v (%s)", err, out)
	}

	if err := os.WriteFile(queueFile, []byte("{\"schema_version\":1,\"updated_at\":\"now\"}\n"), 0o644); err != nil {
		t.Fatalf("failed to dirty queue file: %v", err)
	}

	if err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName); err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() with operational queue dirtiness error = %v, want nil", err)
	}
}

func TestFilterBlockingWorktreeStatus_IgnoresOnlyOperationalPaths(t *testing.T) {
	status := strings.Join([]string{
		"M .gromit/integration-queue.json",
		" M .beads/backup/backup_state.json",
		"?? .beads/dolt-monitor.pid",
		"?? internal/runner/.gromit/tmp/go-build-cache/ab/cached-file-a",
		"?? .gromit/tmp/go-build-cache/ab/cached-file-b",
		" M .gromit/plans/tui-pipeline-view.md",
		" M .gromit/plans/validation-auto-fix-integration.md",
		" M .gromit/specs/remove-tdd-fresh-context.md",
		" M .gromit/reports/debug-20260303-104035.md",
		" M .gromit/templates/PROMPT_build.md",
		" M .gromit/experiments/current.md",
		"?? .gromit/epics/new-epic.md",
		"?? internal/runner/.gromit/reports/nested-report.md",
		" M internal/runner/specbranch/git_ops.go",
	}, "\n")

	filtered := filterBlockingWorktreeStatus(status)

	nonBlockingPaths := []string{
		".gromit/integration-queue.json",
		".beads/backup/backup_state.json",
		".beads/dolt-monitor.pid",
		"internal/runner/.gromit/tmp/go-build-cache/ab/cached-file-a",
		".gromit/tmp/go-build-cache/ab/cached-file-b",
		".gromit/plans/tui-pipeline-view.md",
		".gromit/plans/validation-auto-fix-integration.md",
		".gromit/specs/remove-tdd-fresh-context.md",
		".gromit/reports/debug-20260303-104035.md",
		".gromit/templates/PROMPT_build.md",
		".gromit/experiments/current.md",
		".gromit/epics/new-epic.md",
		"internal/runner/.gromit/reports/nested-report.md",
	}
	for _, p := range nonBlockingPaths {
		if strings.Contains(filtered, p) {
			t.Fatalf("filtered status should not include non-blocking path %q: %q", p, filtered)
		}
	}
	if !strings.Contains(filtered, "internal/runner/specbranch/git_ops.go") {
		t.Fatalf("filtered status should retain source changes: %q", filtered)
	}
}

// TestRebaseSpecOntoMain_Rebases verifies that RebaseSpecOntoMain rebases
// the spec branch onto main without conflicts.
func TestRebaseSpecOntoMain_Rebases(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-rebase-test"

	// Create spec branch and make a commit
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v", err)
	}

	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "spec commit")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create spec commit: %v", err)
	}

	// Make a commit on main
	cmd = exec.Command("git", "checkout", fixture.BaseBranch)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "main commit")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create main commit: %v", err)
	}

	// Rebase spec branch onto main
	err = ops.RebaseSpecOntoMain(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("RebaseSpecOntoMain() error = %v, want nil", err)
	}

	// Verify we're still on the spec branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = fixture.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if string(output) != specBranchName+"\n" {
		t.Errorf("not on spec branch after rebase: got %q", string(output))
	}

	// Verify the rebase succeeded (spec branch should be ahead of main)
	cmd = exec.Command("git", "log", "--oneline", fixture.BaseBranch+".."+specBranchName)
	cmd.Dir = fixture.Dir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to check log: %v", err)
	}
	if len(output) == 0 {
		t.Error("spec branch should have commits ahead of main after rebase")
	}
}

// TestFastForwardMergeToMain_MergesSuccessfully verifies that FastForwardMergeToMain
// successfully merges the spec branch into main.
func TestFastForwardMergeToMain_MergesSuccessfully(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-merge-test"

	// Create spec branch and make a commit
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v", err)
	}

	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "spec commit")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create spec commit: %v", err)
	}

	// Rebase onto main to ensure fast-forward is possible
	err = ops.RebaseSpecOntoMain(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("RebaseSpecOntoMain() error = %v", err)
	}

	// Merge spec branch into main
	err = ops.FastForwardMergeToMain(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("FastForwardMergeToMain() error = %v, want nil", err)
	}

	// Verify main is now pointing to the same commit as spec branch
	cmd = exec.Command("git", "rev-parse", "main")
	cmd.Dir = fixture.Dir
	mainRef, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get main ref: %v", err)
	}

	cmd = exec.Command("git", "rev-parse", specBranchName)
	cmd.Dir = fixture.Dir
	specRef, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get spec branch ref: %v", err)
	}

	if string(mainRef) != string(specRef) {
		t.Errorf("main and spec branch should point to same commit after merge; got main=%q, spec=%q", string(mainRef), string(specRef))
	}
}

// TestFinalizeSpecBranch_PerformsRebaseMergeAndDeletion ensures the new finalize
// helper rebases the spec branch onto main, fast-forward merges it, and deletes it.
func TestFinalizeSpecBranch_PerformsRebaseMergeAndDeletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-finalize-complete"

	if err := ops.CreateOrCheckoutSpecBranch(ctx, specBranchName); err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v", err)
	}

	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "spec finalize commit")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create spec commit: %v", err)
	}

	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = fixture.Dir
	commitOutput, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse HEAD failed: %v", err)
	}
	specCommit := strings.TrimSpace(string(commitOutput))

	if err := ops.FinalizeSpecBranch(ctx, specBranchName); err != nil {
		t.Fatalf("FinalizeSpecBranch() error = %v", err)
	}

	cmd = exec.Command("git", "rev-parse", fixture.BaseBranch)
	cmd.Dir = fixture.Dir
	mainCommitBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse base branch failed: %v", err)
	}
	mainCommit := strings.TrimSpace(string(mainCommitBytes))
	if mainCommit != specCommit {
		t.Fatalf("main commit mismatch: got %q, want %q", mainCommit, specCommit)
	}

	cmd = exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+specBranchName)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err == nil {
		t.Errorf("spec branch %s should have been deleted", specBranchName)
	}
}

// TestDeleteSpecBranch_DeletesSuccessfully verifies that DeleteSpecBranch
// successfully deletes the spec branch.
func TestDeleteSpecBranch_DeletesSuccessfully(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-delete-test"

	// Create spec branch
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v", err)
	}

	// Verify the branch exists
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+specBranchName)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("branch %s does not exist before deletion", specBranchName)
	}

	// Checkout a different branch before deleting
	cmd = exec.Command("git", "checkout", fixture.BaseBranch)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout base branch before deletion: %v", err)
	}

	// Delete the branch
	err = ops.DeleteSpecBranch(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("DeleteSpecBranch() error = %v, want nil", err)
	}

	// Verify the branch no longer exists
	cmd = exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+specBranchName)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err == nil {
		t.Error("branch should have been deleted but still exists")
	}
}

func TestRebaseSpecOntoMain_UsesConfiguredBaseBranch(t *testing.T) {
	t.Parallel()

	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, "nonexistent")

	specBranchName := "gromit/spec-base-branch"

	if err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName); err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v", err)
	}

	err := ops.RebaseSpecOntoMain(context.Background(), specBranchName)
	if err == nil {
		t.Fatalf("RebaseSpecOntoMain() expected error when base branch missing")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("RebaseSpecOntoMain() error = %v, want mention of nonexistent", err)
	}
}

// TestRebaseSpecOntoMain_ReturnsConflictError verifies that RebaseSpecOntoMain
// returns a ConflictError when rebase conflicts occur.
func TestRebaseSpecOntoMain_ReturnsConflictError(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	// Make a conflicting spec branch
	specBranchName := "gromit/spec-conflict-test"
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v", err)
	}

	// Make a change on spec branch
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "spec change")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create spec commit: %v", err)
	}

	// Make a conflicting change on main
	cmd = exec.Command("git", "checkout", fixture.BaseBranch)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	// Create a conflict by modifying the same file on main
	conflictFile := "rebase_conflict_test.txt"
	if err := os.WriteFile(filepath.Join(fixture.Dir, conflictFile), []byte("main change"), 0o644); err != nil {
		t.Fatalf("failed to write conflict file: %v", err)
	}

	cmd = exec.Command("git", "add", conflictFile)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add conflict file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "main conflict change")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on main: %v", err)
	}

	// Make same file change on spec branch
	cmd = exec.Command("git", "checkout", specBranchName)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout spec branch: %v", err)
	}

	if err := os.WriteFile(filepath.Join(fixture.Dir, conflictFile), []byte("spec change"), 0o644); err != nil {
		t.Fatalf("failed to write conflict file on spec: %v", err)
	}

	cmd = exec.Command("git", "add", conflictFile)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add conflict file on spec: %v", err)
	}

	cmd = exec.Command("git", "commit", "--amend", "-m", "spec with conflict")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on spec: %v", err)
	}

	// Attempt rebase which should conflict
	err = ops.RebaseSpecOntoMain(context.Background(), specBranchName)

	// Verify it's a ConflictError
	var conflictErr *specmerge.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ConflictError, got %T: %v", err, err)
	}

	if conflictErr.Operation != "rebase" {
		t.Errorf("ConflictError.Operation = %q, want %q", conflictErr.Operation, "rebase")
	}
}

// TestFastForwardMergeToMain_ReturnsConflictError verifies that FastForwardMergeToMain
// returns a ConflictError when the branch cannot be fast-forward merged.
func TestFastForwardMergeToMain_ReturnsConflictError(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	// Create spec branch with changes
	specBranchName := "gromit/spec-merge-conflict-test"
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v", err)
	}

	// Make a change on spec branch
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "spec commit")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create spec commit: %v", err)
	}

	// Make another change on main that diverges
	cmd = exec.Command("git", "checkout", fixture.BaseBranch)
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "main commit")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create main commit: %v", err)
	}

	// Now spec branch cannot be fast-forward merged
	err = ops.FastForwardMergeToMain(context.Background(), specBranchName)

	// Should return a ConflictError since fast-forward is not possible
	var conflictErr *specmerge.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ConflictError or regular error, got %T: %v", err, err)
	}
}

// TestGitOpsImplementsSpecmergeInterface verifies that GitOps implements
// the specmerge.GitOps interface.
func TestGitOpsImplementsSpecmergeInterface(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	// Verify that GitOps implements specmerge.GitOps interface
	var _ specmerge.GitOps = ops

	// Verify all required methods exist and are callable
	ctx := context.Background()

	// RebaseOnto should exist
	if err := ops.RebaseOnto(ctx, fixture.OurBranch, fixture.TheirBranch); err == nil {
		// Error expected, but just verifying the method exists
	}

	// FastForwardMerge should exist
	if err := ops.FastForwardMerge(ctx, fixture.OurBranch); err == nil {
		// Error expected, but just verifying the method exists
	}

	// DeleteBranch should exist
	if err := ops.DeleteBranch(ctx, fixture.OurBranch); err == nil {
		// Error expected, but just verifying the method exists
	}
}

// TestCreateOrCheckoutSpecBranch_WaitForProcessCapacity verifies that CreateOrCheckoutSpecBranch
// calls WaitForProcessCapacity before forking subprocesses.
func TestCreateOrCheckoutSpecBranch_WaitForProcessCapacity(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-capacity-test"

	// Track if waitForProcessCapacityFn was called
	var called bool
	oldFn := waitForProcessCapacityFn
	t.Cleanup(func() { waitForProcessCapacityFn = oldFn })
	waitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		called = true
		return nil
	}

	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v, want nil", err)
	}

	if !called {
		t.Error("WaitForProcessCapacity was not called")
	}
}

// TestRebaseOnto_WaitForProcessCapacity verifies that RebaseOnto
// calls WaitForProcessCapacity before forking subprocesses.
func TestRebaseOnto_WaitForProcessCapacity(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-rebase-cap-test"
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v", err)
	}

	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "spec commit")
	cmd.Dir = fixture.Dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create spec commit: %v", err)
	}

	// Track if waitForProcessCapacityFn was called
	var called bool
	oldFn := waitForProcessCapacityFn
	t.Cleanup(func() { waitForProcessCapacityFn = oldFn })
	waitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		called = true
		return nil
	}

	err = ops.RebaseOnto(context.Background(), specBranchName, fixture.BaseBranch)
	if err != nil {
		t.Fatalf("RebaseOnto() error = %v, want nil", err)
	}

	if !called {
		t.Error("WaitForProcessCapacity was not called in RebaseOnto")
	}
}

// --- Worktree-conflict recovery tests ---

func TestParseWorktreeConflictPath_ExtractsPath(t *testing.T) {
	t.Parallel()
	input := "fatal: 'gromit/spec-branch' is already used by worktree at '/tmp/repo-gromit-run-12345'"
	path, ok := parseWorktreeConflictPath(input)
	if !ok {
		t.Fatal("parseWorktreeConflictPath() returned false, want true")
	}
	if path != "/tmp/repo-gromit-run-12345" {
		t.Fatalf("parseWorktreeConflictPath() path = %q, want %q", path, "/tmp/repo-gromit-run-12345")
	}
}

func TestParseWorktreeConflictPath_ReturnsFalseWhenNoConflict(t *testing.T) {
	t.Parallel()
	input := "fatal: 'bad branch name' is not a valid branch name"
	_, ok := parseWorktreeConflictPath(input)
	if ok {
		t.Fatal("parseWorktreeConflictPath() returned true for non-worktree error, want false")
	}
}

func TestIsStaleGromitWorktree_TrueForGromitRunPaths(t *testing.T) {
	t.Parallel()
	if !isStaleGromitWorktree("/tmp/myrepo-gromit-run-1772611822677537000") {
		t.Fatal("isStaleGromitWorktree() = false for -gromit-run- path, want true")
	}
}

func TestIsStaleGromitWorktree_FalseForNonRunGromitPaths(t *testing.T) {
	t.Parallel()
	// Interactive and debug worktrees should NOT be treated as stale run sessions
	if isStaleGromitWorktree("/tmp/myrepo-gromit-interactive") {
		t.Fatal("isStaleGromitWorktree() = true for -gromit-interactive path, want false")
	}
	if isStaleGromitWorktree("/tmp/myrepo-gromit-debug-abc123") {
		t.Fatal("isStaleGromitWorktree() = true for -gromit-debug- path, want false")
	}
}

func TestIsStaleGromitWorktree_FalseForNonGromitPaths(t *testing.T) {
	t.Parallel()
	if isStaleGromitWorktree("/tmp/myrepo-feature-branch") {
		t.Fatal("isStaleGromitWorktree() = true for non-gromit path, want false")
	}
}

func TestRecoverStaleSessionWorktree_RemovesStaleWhenRunInactive(t *testing.T) {
	ctx := context.Background()
	var removedPath string
	oldRemove := removeStaleWorktreeFn
	removeStaleWorktreeFn = func(_ context.Context, _ string, path string) (bool, error) {
		removedPath = path
		return true, nil
	}
	t.Cleanup(func() { removeStaleWorktreeFn = oldRemove })

	output := "fatal: 'gromit/spec-branch' is already used by worktree at '/tmp/repo-gromit-run-12345'"
	attempted, path, err := recoverStaleSessionWorktree(ctx, "/tmp/repo", output)
	if err != nil {
		t.Fatalf("recoverStaleSessionWorktree() error = %v, want nil", err)
	}
	if !attempted {
		t.Fatal("recoverStaleSessionWorktree() did not attempt removal")
	}
	if path != "/tmp/repo-gromit-run-12345" {
		t.Fatalf("recoverStaleSessionWorktree() path = %q, want %q", path, "/tmp/repo-gromit-run-12345")
	}
	if removedPath != path {
		t.Fatalf("removeStaleWorktreeFn called with %q, want %q", removedPath, path)
	}
}

func TestRecoverStaleSessionWorktree_SkipsWhenRunActive(t *testing.T) {
	ctx := context.Background()
	calledRemove := false
	oldRemove := removeStaleWorktreeFn
	removeStaleWorktreeFn = func(context.Context, string, string) (bool, error) {
		calledRemove = true
		return true, nil
	}
	t.Cleanup(func() { removeStaleWorktreeFn = oldRemove })

	oldRunLoop := runLoopActiveFn
	runLoopActiveFn = func(string) bool { return true }
	t.Cleanup(func() { runLoopActiveFn = oldRunLoop })

	attempted, path, err := recoverStaleSessionWorktree(ctx, "/tmp/repo", "fatal: 'gromit/spec-branch' is already used by worktree at '/tmp/repo-gromit-run-12345'")
	if err != nil {
		t.Fatalf("recoverStaleSessionWorktree() error = %v, want nil", err)
	}
	if attempted {
		t.Fatal("recoverStaleSessionWorktree() should not attempt removal when run active")
	}
	if path != "" {
		t.Fatalf("recoverStaleSessionWorktree() path = %q, want empty", path)
	}
	if calledRemove {
		t.Fatal("removeStaleWorktreeFn should not be called when run active")
	}
}

func TestRecoverStaleSessionWorktree_ParsesDoubleQuotedOutput(t *testing.T) {
	ctx := context.Background()
	var removedPath string
	oldRemove := removeStaleWorktreeFn
	removeStaleWorktreeFn = func(_ context.Context, _ string, path string) (bool, error) {
		removedPath = path
		return true, nil
	}
	t.Cleanup(func() { removeStaleWorktreeFn = oldRemove })

	oldRunLoop := runLoopActiveFn
	runLoopActiveFn = func(string) bool { return false }
	t.Cleanup(func() { runLoopActiveFn = oldRunLoop })

	output := "fatal: 'gromit/spec-branch' is already used by worktree at \"/tmp/repo-gromit-run-12345\""
	attempted, path, err := recoverStaleSessionWorktree(ctx, "/tmp/repo", output)
	if err != nil {
		t.Fatalf("recoverStaleSessionWorktree() error = %v, want nil", err)
	}
	if !attempted {
		t.Fatal("recoverStaleSessionWorktree() did not attempt removal for double-quoted output")
	}
	if path != "/tmp/repo-gromit-run-12345" {
		t.Fatalf("recoverStaleSessionWorktree() path = %q, want %q", path, "/tmp/repo-gromit-run-12345")
	}
	if removedPath != path {
		t.Fatalf("removeStaleWorktreeFn called with %q, want %q", removedPath, path)
	}
}

func TestCreateOrCheckoutSpecBranch_RecoversStalWorktreeConflict(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-worktree-recovery"

	// Create the spec branch so it exists, then switch back to base.
	cmd := exec.Command("git", "branch", specBranchName)
	cmd.Dir = fixture.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create branch: %v (%s)", err, out)
	}

	// Create a worktree that holds the branch, using a gromit-like path.
	worktreeDir := filepath.Join(t.TempDir(), ".-gromit-run-stale-1234")
	cmd = exec.Command("git", "worktree", "add", worktreeDir, specBranchName)
	cmd.Dir = fixture.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v (%s)", err, out)
	}

	calledRunLoop := false
	oldRunLoop := runLoopActiveFn
	runLoopActiveFn = func(string) bool {
		calledRunLoop = true
		return false
	}
	t.Cleanup(func() { runLoopActiveFn = oldRunLoop })

	// Now CreateOrCheckoutSpecBranch should detect the stale worktree, remove it, and succeed.
	if err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName); err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() after stale worktree error = %v, want nil", err)
	}
	if !calledRunLoop {
		t.Fatal("expected runLoopActiveFn to be called during stale worktree recovery")
	}

	// Verify we're on the spec branch.
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = fixture.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if strings.TrimSpace(string(output)) != specBranchName {
		t.Fatalf("expected to be on %s, got %s", specBranchName, strings.TrimSpace(string(output)))
	}
}

// --- RevertAndReturnToBase tests ---

// TestRevertAndReturnToBase_RevertsAndCheckoutsBase verifies that dirty tracked and
// untracked files are cleaned and the base branch is checked out.
func TestRevertAndReturnToBase_RevertsAndCheckoutsBase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	// Create and switch to a spec branch.
	specBranch := "gromit/spec-revert-test"
	if err := ops.CreateOrCheckoutSpecBranch(ctx, specBranch); err != nil {
		t.Fatalf("CreateOrCheckoutSpecBranch() error = %v", err)
	}

	// Dirty a tracked file.
	trackedFile := filepath.Join(fixture.Dir, fixture.ConflictingFile)
	if err := os.WriteFile(trackedFile, []byte("dirty tracked change"), 0o644); err != nil {
		t.Fatalf("failed to dirty tracked file: %v", err)
	}

	// Create an untracked file.
	untrackedFile := filepath.Join(fixture.Dir, "untracked_build_artifact.txt")
	if err := os.WriteFile(untrackedFile, []byte("build output"), 0o644); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	// Verify worktree is dirty before the call.
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = fixture.Dir
	statusBefore, _ := cmd.CombinedOutput()
	if len(strings.TrimSpace(string(statusBefore))) == 0 {
		t.Fatal("expected dirty worktree before RevertAndReturnToBase")
	}

	// Call RevertAndReturnToBase.
	if err := ops.RevertAndReturnToBase(ctx); err != nil {
		t.Fatalf("RevertAndReturnToBase() error = %v", err)
	}

	// Verify worktree is clean.
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = fixture.Dir
	statusAfter, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	if strings.TrimSpace(string(statusAfter)) != "" {
		t.Fatalf("worktree not clean after RevertAndReturnToBase: %s", statusAfter)
	}

	// Verify we're on the base branch.
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = fixture.Dir
	branchOutput, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if strings.TrimSpace(string(branchOutput)) != fixture.BaseBranch {
		t.Fatalf("expected branch %s, got %s", fixture.BaseBranch, strings.TrimSpace(string(branchOutput)))
	}

	// Verify untracked file was removed.
	if _, err := os.Stat(untrackedFile); !os.IsNotExist(err) {
		t.Fatalf("untracked file should have been removed, but still exists")
	}
}

// TestRevertAndReturnToBase_NoOpWhenCleanAndOnBase verifies that the method is a no-op
// when the worktree is already clean and on the base branch.
func TestRevertAndReturnToBase_NoOpWhenCleanAndOnBase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	// We should already be on the base branch with a clean worktree.
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = fixture.Dir
	branchOutput, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if strings.TrimSpace(string(branchOutput)) != fixture.BaseBranch {
		t.Fatalf("fixture not on base branch: %s", strings.TrimSpace(string(branchOutput)))
	}

	// Should succeed as a no-op.
	if err := ops.RevertAndReturnToBase(ctx); err != nil {
		t.Fatalf("RevertAndReturnToBase() error = %v, want nil (no-op)", err)
	}
}

// TestRevertAndReturnToBase_ErrorNotInGitRepo verifies that the method returns an
// error when called outside a git repository.
func TestRevertAndReturnToBase_ErrorNotInGitRepo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	nonGitDir := t.TempDir()
	ops := NewGitOps(nonGitDir, "main")

	err := ops.RevertAndReturnToBase(ctx)
	if err == nil {
		t.Fatal("RevertAndReturnToBase() expected error outside git repo, got nil")
	}
}

func TestCreateOrCheckoutSpecBranch_ReturnsErrorForNonGromitWorktree(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir, fixture.BaseBranch)

	specBranchName := "gromit/spec-worktree-nongromit"

	// Create the spec branch without checking it out.
	cmd := exec.Command("git", "branch", specBranchName)
	cmd.Dir = fixture.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create branch: %v (%s)", err, out)
	}

	// Create a worktree with a non-gromit path holding the branch.
	worktreeDir := filepath.Join(t.TempDir(), "user-feature-worktree")
	cmd = exec.Command("git", "worktree", "add", worktreeDir, specBranchName)
	cmd.Dir = fixture.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v (%s)", err, out)
	}
	t.Cleanup(func() {
		cmd := exec.Command("git", "worktree", "remove", "--force", worktreeDir)
		cmd.Dir = fixture.Dir
		_ = cmd.Run()
	})

	// Should fail because the worktree is not a gromit worktree (not recoverable).
	err := ops.CreateOrCheckoutSpecBranch(context.Background(), specBranchName)
	if err == nil {
		t.Fatal("CreateOrCheckoutSpecBranch() expected error for non-gromit worktree conflict, got nil")
	}
	if !strings.Contains(err.Error(), "checkout") {
		t.Fatalf("error %q should mention checkout failure", err.Error())
	}
}
