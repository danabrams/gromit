package helpers

import "testing"

const dirtyWorktreeFileName = "dirty_worktree.txt"

// DirtyWorktreeFixture exposes metadata for a repo deliberately left dirty.
type DirtyWorktreeFixture struct {
	Dir        string
	BaseBranch string
	DirtyFile  string
}

// NewDirtyWorktreeFixture creates a git repo with a tracked file that has uncommitted changes.
func NewDirtyWorktreeFixture(t testing.TB) *DirtyWorktreeFixture {
	t.Helper()

	dir := t.TempDir()
	runGitMustSucceed(t, dir, "init")
	runGitMustSucceed(t, dir, "config", "user.email", "gromit@example.com")
	runGitMustSucceed(t, dir, "config", "user.name", "gromit")
	runGitMustSucceed(t, dir, "branch", "-m", baseBranchName)

	writeFile(t, dir, dirtyWorktreeFileName, "clean\n")
	runGitMustSucceed(t, dir, "add", dirtyWorktreeFileName)
	runGitMustSucceed(t, dir, "commit", "-m", "initial commit")

	// Leave the tracked file dirty to trigger worktree-blocking status.
	writeFile(t, dir, dirtyWorktreeFileName, "dirty\n")

	return &DirtyWorktreeFixture{
		Dir:        dir,
		BaseBranch: baseBranchName,
		DirtyFile:  dirtyWorktreeFileName,
	}
}
