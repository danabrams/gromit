
package runner

import (
	"context"
	"testing"
)

// TestRunnerLintBaseline_RunViaRunnerAPI verifies the runner package exposes a
// public API for running the errcheck/unused/staticcheck lint baseline against
// the runner package, returning an exit code of 0 when no issues remain.
//
// Expected failure: RunRunnerLintBaseline and RunnerLintBaselineRequest do not
// exist yet, and the current lint baseline still reports violations in the
// runner package. The implementation must add the API and make the lint run
// clean.
func TestRunnerLintBaseline_RunViaRunnerAPI(t *testing.T) {
	repoRoot := lintBaselineRepoRoot(t)
	golangciLint := resolveGolangCILintV2Path(t)

	result, err := RunRunnerLintBaseline(context.Background(), RunnerLintBaselineRequest{
		GolangciLintPath: golangciLint,
		RepoRoot:         repoRoot,
		Linters:          []string{"errcheck", "unused", "staticcheck"},
		Packages:         []string{"./internal/runner/..."},
	})
	if err != nil {
		t.Fatalf("RunRunnerLintBaseline: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected lint baseline exit code 0, got %d\nlint output:\n%s", result.ExitCode, result.Output)
	}
}
