package validate

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// CommandRunner runs a single validation command and returns stdout, stderr, exit code, and any error.
type CommandRunner interface {
	Run(ctx context.Context, command, workDir string) (string, string, int, error)
}

// AutoFixer applies trivial formatting fixes (gofmt, goimports) to changed files.
// Returns nil when fixes were applied successfully (even if no files changed).
type AutoFixer interface {
	Fix() error
}

// Validate implements pipeline.Stage for Stage 3: programmatic validation.
// It runs fast or full validation commands, attempts trivial auto-fix on failure,
// and enforces the mandatory command prefix policy.
//
// On success (directly or after auto-fix), returns Proceed with no ValidationFailures.
// On failure after auto-fix, returns Proceed with ValidationFailures populated so the
// orchestrator can feed them into the next Build stage Input.
// On mandatory prefix policy violation, returns Block immediately.
type Validate struct {
	runner  CommandRunner
	fixer   AutoFixer // optional; nil means skip auto-fix
	workDir string    // working directory for commands; empty means current directory
	output  io.Writer
}

// Compile-time check: *Validate must implement pipeline.Stage.
var _ pipeline.Stage = (*Validate)(nil)

// New creates a Validate stage with the given command runner and output writer.
// output receives warning messages; pass io.Discard to suppress.
func New(runner CommandRunner, output io.Writer) *Validate {
	return &Validate{runner: runner, output: output}
}

// WithAutoFixer configures an optional AutoFixer for applying gofmt/goimports on failure.
func (v *Validate) WithAutoFixer(f AutoFixer) *Validate {
	v.fixer = f
	return v
}

// WithWorkDir sets the working directory for validation commands.
func (v *Validate) WithWorkDir(dir string) *Validate {
	v.workDir = dir
	return v
}

// Run executes the validate stage:
//  1. Returns Proceed immediately when validation is disabled.
//  2. Enforces mandatory command prefix policy; returns Block on violation.
//  3. Selects fast or full commands based on periodic full validation gate.
//  4. Runs commands; returns Proceed on success.
//  5. On failure, runs auto-fix and re-validates.
//  6. Returns Proceed on success after auto-fix.
//  7. Returns Proceed with ValidationFailures if still failing after auto-fix.
func (v *Validate) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	cfg := in.Config
	if cfg == nil || !cfg.Validation.Enabled {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	w := v.output
	if w == nil {
		w = io.Discard
	}

	// Select command set: full or fast based on periodic gate frequency.
	commands := v.selectCommands(cfg, in.Iteration)

	// Enforce mandatory command prefix policy before running any commands.
	if err := checkMandatoryPrefixes(cfg.Validation.MandatoryCommands, commands); err != nil {
		return pipeline.Output{
			Decision:           pipeline.Block,
			ValidationFailures: []string{err.Error()},
		}, nil
	}

	// Run validation commands.
	failures, err := v.runCommands(ctx, commands)
	if err != nil {
		return pipeline.Output{}, err
	}
	if len(failures) == 0 {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	// Validation failed. Try auto-fix if configured.
	if v.fixer != nil {
		if fixErr := v.fixer.Fix(); fixErr != nil {
			fmt.Fprintf(w, "Warning: auto-fix failed: %v\n", fixErr)
		}

		// Re-run validation after auto-fix.
		failures, err = v.runCommands(ctx, commands)
		if err != nil {
			return pipeline.Output{}, err
		}
		if len(failures) == 0 {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}
	}

	// Still failing after auto-fix (or no auto-fixer configured).
	// Return Proceed with ValidationFailures so the orchestrator can feed them to
	// the next Build stage input.
	return pipeline.Output{
		Decision:           pipeline.Proceed,
		ValidationFailures: failures,
	}, nil
}

// selectCommands returns the full command set when this iteration is a periodic
// full-validation boundary, otherwise returns the fast command set.
func (v *Validate) selectCommands(cfg *config.Config, iteration int) []string {
	n := cfg.Validation.FullValidationEveryN
	if n > 0 && iteration%n == 0 {
		return cfg.Validation.FullCommandsOrDefault()
	}
	return cfg.Validation.FastCommandsOrDefault()
}

// checkMandatoryPrefixes returns an error when any mandatory prefix is not
// covered by the active command set. Each prefix must appear as a prefix of
// at least one command in the set.
func checkMandatoryPrefixes(prefixes, commands []string) error {
	var missing []string
	for _, prefix := range prefixes {
		found := false
		for _, cmd := range commands {
			if strings.HasPrefix(strings.TrimSpace(cmd), strings.TrimSpace(prefix)) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, prefix)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("validation missing mandatory command prefixes: %s", strings.Join(missing, ", "))
	}
	return nil
}

// runCommands executes each command in sequence, stopping at the first failure.
// Returns a slice of failure summary strings (empty on success).
func (v *Validate) runCommands(ctx context.Context, commands []string) ([]string, error) {
	var failures []string
	for _, cmd := range commands {
		stdout, stderr, exitCode, err := v.runner.Run(ctx, cmd, v.workDir)
		if err != nil {
			return nil, fmt.Errorf("validate: running %q: %w", cmd, err)
		}
		if exitCode != 0 {
			summary := formatFailureSummary(cmd, exitCode, stdout, stderr)
			failures = append(failures, summary)
			break // stop on first failure
		}
	}
	return failures, nil
}

// formatFailureSummary produces a compact failure message for a command's output.
func formatFailureSummary(command string, exitCode int, stdout, stderr string) string {
	msg := fmt.Sprintf("Command failed: %s (exit code %d)", command, exitCode)
	if stdout != "" {
		msg += "\n" + stdout
	}
	if stderr != "" {
		msg += "\n" + stderr
	}
	return msg
}
