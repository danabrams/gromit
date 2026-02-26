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
