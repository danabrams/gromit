//go:build acceptance

package runner

import (
	"os/exec"
	"testing"
)

// TestSplitRunnerFinalVerification_RunnerLintPasses verifies
// `golangci-lint run ./internal/runner/...` passes for the finalized split.
//
// Expected failure: `finalVerificationVerifyLayout` fails first while
// `RunnerSplitFinalVerificationLintClean` is not present in the codebase yet.
func TestSplitRunnerFinalVerification_RunnerLintPasses(t *testing.T) {
	finalVerificationVerifyLayout(t)
	repoRoot := finalVerificationRepoRoot(t)
	golangciLint := resolveGolangCILintV2Path(t)

	cmd := exec.Command(golangciLint, "run", "./internal/runner/...")
	cmd.Dir = repoRoot
	cmd.Env = golangciLintAcceptanceEnv(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("golangci-lint run ./internal/runner/... failed: %v\n%s", err, string(out))
	}
}
