package runner

import (
	"context"
	"fmt"
	"os/exec"
)

// RunnerLintBaselineRequest contains parameters for running the golangci-lint
// baseline check on the runner package.
type RunnerLintBaselineRequest struct {
	GolangciLintPath string
	RepoRoot         string
	Linters          []string
	Packages         []string
}

// RunnerLintBaselineResult contains the result of running the lint baseline.
type RunnerLintBaselineResult struct {
	ExitCode int
	Output   string
}

// RunRunnerLintBaseline executes golangci-lint with the specified linters
// and packages, returning the exit code.
func RunRunnerLintBaseline(ctx context.Context, req RunnerLintBaselineRequest) (*RunnerLintBaselineResult, error) {
	args := []string{"run", "--default=none", "--enable-only"}

	// Join linters with commas
	linterArg := ""
	for i, linter := range req.Linters {
		if i > 0 {
			linterArg += ","
		}
		linterArg += linter
	}
	args = append(args, linterArg)
	args = append(args, req.Packages...)

	cmd := exec.CommandContext(ctx, req.GolangciLintPath, args...)
	cmd.Dir = req.RepoRoot

	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("run runner lint baseline: %w\noutput:\n%s", err, string(out))
		}
	}

	return &RunnerLintBaselineResult{
		ExitCode: exitCode,
		Output:   string(out),
	}, nil
}
