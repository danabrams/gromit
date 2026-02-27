package specbranch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/test/helpers"
)

// TestCreateOrCheckoutSpecBranch_CreatesNewBranch verifies that CreateOrCheckoutSpecBranch
// creates a new spec branch when it doesn't exist.
func TestCreateOrCheckoutSpecBranch_CreatesNewBranch(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir)

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

// TestRebaseSpecOntoMain_Rebases verifies that RebaseSpecOntoMain rebases
// the spec branch onto main without conflicts.
func TestRebaseSpecOntoMain_Rebases(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir)

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
	ops := NewGitOps(fixture.Dir)

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

// TestDeleteSpecBranch_DeletesSuccessfully verifies that DeleteSpecBranch
// successfully deletes the spec branch.
func TestDeleteSpecBranch_DeletesSuccessfully(t *testing.T) {
	fixture := helpers.NewDeterministicGitConflictFixture(t)
	ops := NewGitOps(fixture.Dir)

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
	ops := NewGitOps(fixture.Dir)

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
	ops := NewGitOps(fixture.Dir)

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
	ops := NewGitOps(fixture.Dir)

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
