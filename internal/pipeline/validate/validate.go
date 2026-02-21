package validate

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// CommandRunner runs a single validation command and returns stdout, stderr, exit code, and any error.
type CommandRunner interface {
	Run(ctx context.Context, command, workDir string) (string, string, int, error)
}

// Validate implements pipeline.Stage for Stage 3: programmatic validation.
// It runs validation commands via CommandRunner and returns Proceed on success
// or Block with structured summaries on failure.
type Validate struct {
	runner CommandRunner
}

// Compile-time check: *Validate must implement pipeline.Stage.
var _ pipeline.Stage = (*Validate)(nil)

// New creates a Validate stage with the given command runner.
func New(runner CommandRunner, output interface{}) *Validate {
	return &Validate{runner: runner}
}

// Run executes the validate stage:
//  1. Returns Proceed immediately when validation is disabled or bead is nil.
//  2. Retrieves fast validation commands from config.
//  3. Executes commands via CommandRunner; returns Proceed on success.
//  4. On any command failure, formats summaries using ExtractValidationSummary
//     and returns Block with ValidationFailures.
func (v *Validate) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if in.Bead == nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	cfg := in.Config
	if cfg == nil || !cfg.Validation.Enabled {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	// Get fast validation commands from config.
	commands := cfg.Validation.FastCommandsOrDefault()
	if len(commands) == 0 {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	// Run validation commands and collect failures.
	for _, cmd := range commands {
		stdout, stderr, exitCode, err := v.runner.Run(ctx, cmd, "")
		if err != nil {
			return pipeline.Output{}, err
		}

		if exitCode != 0 {
			// Format failure output using ExtractValidationSummary.
			failureOutput := formatCommandFailure(cmd, exitCode, stdout, stderr)
			summary := validation.ExtractValidationSummary(failureOutput)
			return pipeline.Output{
				Decision:           pipeline.Block,
				ValidationFailures: []string{summary},
			}, nil
		}
	}

	return pipeline.Output{Decision: pipeline.Proceed}, nil
}

// formatCommandFailure formats the failure output from a failed command.
func formatCommandFailure(command string, exitCode int, stdout, stderr string) string {
	msg := fmt.Sprintf("Command failed: %s (exit code %d)", command, exitCode)
	if stdout != "" {
		msg += "\n" + stdout
	}
	if stderr != "" {
		msg += "\n" + stderr
	}
	return msg
}
