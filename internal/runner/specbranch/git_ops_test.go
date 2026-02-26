package specbranch

import (
	"context"
	"os/exec"
	"testing"

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
