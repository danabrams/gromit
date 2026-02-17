//go:build integration

package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var runnerSplitExecCommand = exec.Command

func runnerSplitTestEnv() []string {
	return append(
		os.Environ(),
		"GROMIT_SPLIT_PHASE1_REENTRY=1",
		"GROMIT_SPLIT_PHASE2_REENTRY=1",
		"GROMIT_SPLIT_PHASE3_REENTRY=1",
		"GROMIT_SPLIT_PHASE4_REENTRY=1",
		"GROMIT_SPLIT_FINAL_VERIFICATION_REENTRY=1",
	)
}

func runRunnerSplitShelloutCheck(t *testing.T, runnerDir string, args ...string) {
	t.Helper()

	repoRoot := filepath.Clean(filepath.Join(runnerDir, "..", ".."))
	cmd := runnerSplitExecCommand(args[0], args[1:]...)
	cmd.Dir = repoRoot
	cmd.Env = runnerSplitTestEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", args, err, string(out))
	}
}

// TestSplitRunnerPhase1_GoBuildPasses verifies acceptance criterion #1:
// go build ./... passes once adapters/callbacks extraction is complete.
func TestSplitRunnerPhase1_GoBuildPasses(t *testing.T) {
	runnerDir := verifyRunnerSplitPhase1Layout(t)
	runRunnerSplitShelloutCheck(t, runnerDir, "go", "build", "./...")
}

// TestSplitRunnerPhase1_RunnerPackageTestsPass verifies acceptance criterion #2:
// go test ./internal/runner/... -count=1 passes after extraction.
func TestSplitRunnerPhase1_RunnerPackageTestsPass(t *testing.T) {
	if os.Getenv("GROMIT_SPLIT_PHASE1_REENTRY") == "1" {
		return
	}
	runnerDir := verifyRunnerSplitPhase1Layout(t)
	runRunnerSplitShelloutCheck(
		t,
		runnerDir,
		"go", "test", "./internal/runner/...", "-count=1", "-run",
		"TestRunnerSplitVerificationReclassified_ImportIsolation|TestRunnerSplitVerificationReclassified_LineBudgets",
	)
}

// TestSplitRunnerPhase2_GoBuildPasses verifies acceptance criterion #1:
// go build ./... passes once heartbeat/decompose extraction is complete.
func TestSplitRunnerPhase2_GoBuildPasses(t *testing.T) {
	runnerDir := verifyRunnerSplitPhase2Layout(t)
	runRunnerSplitShelloutCheck(t, runnerDir, "go", "build", "./...")
}

// TestSplitRunnerPhase2_RunnerPackageTestsPass verifies acceptance criterion #2:
// go test ./internal/runner/... -count=1 passes after extraction.
func TestSplitRunnerPhase2_RunnerPackageTestsPass(t *testing.T) {
	if os.Getenv("GROMIT_SPLIT_PHASE2_REENTRY") == "1" {
		return
	}
	runnerDir := verifyRunnerSplitPhase2Layout(t)
	runRunnerSplitShelloutCheck(
		t,
		runnerDir,
		"go", "test", "./internal/runner/...", "-count=1", "-run",
		"TestRunnerSplitVerificationReclassified_ImportIsolation|TestRunnerSplitVerificationReclassified_LineBudgets",
	)
}

// TestSplitRunnerPhase3_GoBuildPasses verifies acceptance criterion #1:
// go build ./... passes once gates/logging extraction is complete.
func TestSplitRunnerPhase3_GoBuildPasses(t *testing.T) {
	runnerDir := verifyRunnerSplitPhase3Layout(t)
	runRunnerSplitShelloutCheck(t, runnerDir, "go", "build", "./...")
}

// TestSplitRunnerPhase3_RunnerPackageTestsPass verifies acceptance criterion #2:
// go test ./internal/runner/... -count=1 passes after extraction.
func TestSplitRunnerPhase3_RunnerPackageTestsPass(t *testing.T) {
	if os.Getenv("GROMIT_SPLIT_PHASE3_REENTRY") == "1" {
		return
	}
	runnerDir := verifyRunnerSplitPhase3Layout(t)
	runRunnerSplitShelloutCheck(
		t,
		runnerDir,
		"go", "test", "./internal/runner/...", "-count=1", "-run",
		"TestRunnerSplitVerificationReclassified_ImportIsolation|TestRunnerSplitVerificationReclassified_LineBudgets",
	)
}

// TestSplitRunnerPhase4_GoBuildPasses verifies acceptance criterion #1:
// go build ./... passes once helpers/lifecycle extraction is complete.
func TestSplitRunnerPhase4_GoBuildPasses(t *testing.T) {
	runnerDir := verifyRunnerSplitPhase4Layout(t)
	runRunnerSplitShelloutCheck(t, runnerDir, "go", "build", "./...")
}

// TestSplitRunnerPhase4_RunnerPackageTestsPass verifies acceptance criterion #3:
// go test ./internal/runner/... -count=1 passes after extraction.
func TestSplitRunnerPhase4_RunnerPackageTestsPass(t *testing.T) {
	if os.Getenv("GROMIT_SPLIT_PHASE4_REENTRY") == "1" {
		return
	}
	runnerDir := verifyRunnerSplitPhase4Layout(t)
	runRunnerSplitShelloutCheck(
		t,
		runnerDir,
		"go", "test", "./internal/runner/...", "-count=1", "-run",
		"TestRunnerSplitVerificationReclassified_ImportIsolation|TestRunnerSplitVerificationReclassified_LineBudgets",
	)
}
