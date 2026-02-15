package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	if result.Output != "" {
		t.Errorf("expected empty output on success, got %q", result.Output)
	}
}

func TestRunRunnerLintBaseline_WithLintFailure_CapturesOutputAndExitCode(t *testing.T) {
	tempDir := t.TempDir()
	fakeLintPath := filepath.Join(tempDir, "fake-lint.sh")
	script := "#!/bin/sh\n" +
		"echo lint failed on stderr >&2\n" +
		"exit 5\n"
	if err := os.WriteFile(fakeLintPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake lint script: %v", err)
	}

	req := RunnerLintBaselineRequest{
		GolangciLintPath: fakeLintPath,
		RepoRoot:         tempDir,
		Linters:          []string{"errcheck"},
		Packages:         []string{"./..."},
	}

	result, err := RunRunnerLintBaseline(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 5 {
		t.Fatalf("expected exit code 5, got %d", result.ExitCode)
	}
	if result.Output == "" {
		t.Fatal("expected captured output for lint failure, got empty output")
	}
	if got, want := result.Output, "lint failed on stderr"; !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q, got %q", want, got)
	}
}
