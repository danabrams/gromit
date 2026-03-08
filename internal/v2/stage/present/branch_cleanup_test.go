package present

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	execgit "github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

func TestCleanupMergedWorktreeBranchDeletesMergedSpecBranch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := initPresentTestRepo(t)
	integrationBranch := strings.TrimSpace(runGitInDir(t, repoDir, "rev-parse", "--abbrev-ref", "HEAD"))
	if integrationBranch == "" || integrationBranch == "HEAD" {
		t.Fatalf("integration branch = %q", integrationBranch)
	}

	worktreesDir := t.TempDir()
	gitAdapter := execgit.NewExecGitAdapter(repoDir, worktreesDir)

	const specID = "spec-branch-cleanup"
	wtPath, err := gitAdapter.Checkout(context.Background(), specID)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	writeFile(t, filepath.Join(wtPath, "cleanup.txt"), "change")
	runGitInDir(t, wtPath, "add", "cleanup.txt")
	runGitInDir(t, wtPath, "commit", "-m", "spec change")

	specBranch := presentation.SpecBranchName(specID)
	runGitInDir(t, repoDir, "merge", "--ff-only", specBranch)

	if err := cleanupMergedWorktreeBranch(context.Background(), wtPath, specID, integrationBranch); err != nil {
		t.Fatalf("cleanup merged worktree branch: %v", err)
	}

	branchList := strings.TrimSpace(runGitInDir(t, repoDir, "branch", "--list", specBranch))
	if branchList != "" {
		t.Fatalf("expected merged spec branch %q to be deleted, got %q", specBranch, branchList)
	}

	currentBranch := strings.TrimSpace(runGitInDir(t, wtPath, "rev-parse", "--abbrev-ref", "HEAD"))
	if currentBranch != integrationBranch {
		t.Fatalf("worktree branch = %q, want %q", currentBranch, integrationBranch)
	}
}
