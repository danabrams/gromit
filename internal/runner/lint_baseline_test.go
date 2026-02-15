package runner

import (
	"context"
	"testing"
)

func TestRunRunnerLintBaseline_WithValidRequest_ReturnsSuccess(t *testing.T) {
	req := RunnerLintBaselineRequest{
		GolangciLintPath: "/usr/bin/true", // Use 'true' command to simulate success
		RepoRoot:         t.TempDir(),
		Linters:          []string{"errcheck"},
		Packages:         []string{"./..."},
	}

	result, err := RunRunnerLintBaseline(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}
