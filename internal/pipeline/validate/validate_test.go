package validate

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// fakeCommandRunner is a test double for CommandRunner.
type fakeCommandRunner struct {
	results []commandRunResult
	callIdx int
}

type commandRunResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func (f *fakeCommandRunner) Run(ctx context.Context, command, workDir string) (string, string, int, error) {
	if f.callIdx >= len(f.results) {
		return "", "", 0, nil
	}
	result := f.results[f.callIdx]
	f.callIdx++
	return result.stdout, result.stderr, result.exitCode, result.err
}

// TestValidate_CleanPass_ReturnsProceed verifies that when all commands exit 0,
// the stage returns Proceed with no ValidationFailures.
func TestValidate_CleanPass_ReturnsProceed(t *testing.T) {
	runner := &fakeCommandRunner{
		results: []commandRunResult{
			{stdout: "", stderr: "", exitCode: 0, err: nil},
		},
	}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test bead"},
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}
	if len(out.ValidationFailures) != 0 {
		t.Errorf("ValidationFailures = %v, want empty", out.ValidationFailures)
	}
}

// TestValidate_SingleCommandFailure_ReturnsBlockWithSummaries verifies that when any
// command fails (exit code 1), the stage returns Block with ValidationFailures populated.
func TestValidate_SingleCommandFailure_ReturnsBlockWithSummaries(t *testing.T) {
	runner := &fakeCommandRunner{
		results: []commandRunResult{
			{
				stdout:   "test output",
				stderr:   "test error",
				exitCode: 1,
				err:      nil,
			},
		},
	}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test bead"},
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Block {
		t.Errorf("Decision = %v, want Block on validation failure", out.Decision)
	}
	if len(out.ValidationFailures) == 0 {
		t.Errorf("ValidationFailures is empty; want non-empty summaries")
	}
}

// TestValidate_NilBead_ReturnsProceed verifies that when bead is nil,
// the stage returns Proceed without running commands.
func TestValidate_NilBead_ReturnsProceed(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:   nil,
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed for nil bead", out.Decision)
	}
	if runner.callIdx != 0 {
		t.Errorf("CommandRunner.Run was called; want no calls for nil bead")
	}
}

// TestValidate_DisabledInConfig_ReturnsProceed verifies that when validation is disabled,
// the stage returns Proceed without running commands.
func TestValidate_DisabledInConfig_ReturnsProceed(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  false,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test bead"},
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed when validation disabled", out.Decision)
	}
	if runner.callIdx != 0 {
		t.Errorf("CommandRunner.Run was called; want no calls when disabled")
	}
}
