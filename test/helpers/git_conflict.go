package helpers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	conflictFileName = "conflict.txt"
	baseBranchName   = "main"
	ourBranchName    = "feature/ours-conflict"
	theirBranchName  = "feature/theirs-conflict"
)

// GitConflictFixture exposes metadata about the deterministic conflict repository.
type GitConflictFixture struct {
	Dir             string
	BaseBranch      string
	OurBranch       string
	TheirBranch     string
	ConflictingFile string
}

// NewDeterministicGitConflictFixture initializes a small git repository with two
// divergent branches that edit the same file so merges between them always conflict.
func NewDeterministicGitConflictFixture(t testing.TB) *GitConflictFixture {
	t.Helper()

	dir := t.TempDir()
	runGitMustSucceed(t, dir, "init")
	runGitMustSucceed(t, dir, "config", "user.email", "gromit@example.com")
	runGitMustSucceed(t, dir, "config", "user.name", "gromit")
	runGitMustSucceed(t, dir, "branch", "-m", baseBranchName)

	writeFile(t, dir, conflictFileName, "base\n")
	runGitMustSucceed(t, dir, "add", conflictFileName)
	runGitMustSucceed(t, dir, "commit", "-m", "base commit")

	runGitMustSucceed(t, dir, "checkout", "-b", ourBranchName)
	writeFile(t, dir, conflictFileName, "ours\n")
	runGitMustSucceed(t, dir, "add", conflictFileName)
	runGitMustSucceed(t, dir, "commit", "-m", "ours branch change")

	runGitMustSucceed(t, dir, "checkout", baseBranchName)
	runGitMustSucceed(t, dir, "checkout", "-b", theirBranchName)
	writeFile(t, dir, conflictFileName, "theirs\n")
	runGitMustSucceed(t, dir, "add", conflictFileName)
	runGitMustSucceed(t, dir, "commit", "-m", "theirs branch change")

	runGitMustSucceed(t, dir, "checkout", baseBranchName)

	return &GitConflictFixture{
		Dir:             dir,
		BaseBranch:      baseBranchName,
		OurBranch:       ourBranchName,
		TheirBranch:     theirBranchName,
		ConflictingFile: conflictFileName,
	}
}

// RunGit executes git in the fixture repository and returns the combined output.
func (f *GitConflictFixture) RunGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = f.Dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func runGitMustSucceed(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\noutput:\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func writeFile(t testing.TB, dir, file, content string) {
	t.Helper()
	path := filepath.Join(dir, file)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
