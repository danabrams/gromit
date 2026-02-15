//go:build acceptance

package runner

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func lintBaselineRepoRoot(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

// TestRunnerLintBaseline_ErrcheckUnusedStaticcheck verifies that running
// golangci-lint with errcheck/unused/staticcheck enabled on the runner package
// returns clean.
func TestRunnerLintBaseline_ErrcheckUnusedStaticcheck(t *testing.T) {
	_ = RunnerLintBaselineAcceptanceMarker

	repoRoot := lintBaselineRepoRoot(t)

	cmd := exec.Command(
		"golangci-lint",
		"run",
		"--disable-all",
		"--enable", "errcheck",
		"--enable", "unused",
		"--enable", "staticcheck",
		"./internal/runner/...",
	)
	cmd.Dir = repoRoot

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("golangci-lint baseline failed: %v\n%s", err, string(out))
	}
}
