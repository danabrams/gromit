package validation

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// ErrValidationFailed is a sentinel error indicating validation commands failed
// (non-zero exit code). Callers use errors.Is to distinguish recoverable
// validation failures from execution errors.
var ErrValidationFailed = errors.New("validation failed")

// ExecuteFn is a callback for Claude-based fix attempts during validation recovery.
// The facade wraps escalation.Handler.ExecuteWithRetry and maps it to this signature.
// Returns true if the fix invocation succeeded.
type ExecuteFn func(ctx context.Context, bc *runtypes.BeadContext) bool

// Runner handles direct validation command execution and recovery.
type Runner struct {
	cfg       *config.Config
	cmdRunner runtypes.CmdRunnerFn
	autoFixFn runtypes.AutoFixFn
	executeFn ExecuteFn
	failures  []string // accumulated validation failure summaries
}

// NewRunner creates a Runner with narrow dependency interfaces.
// autoFixFn and executeFn may be nil (no auto-fix or Claude-based recovery).
func NewRunner(cfg *config.Config, cmdRunner runtypes.CmdRunnerFn, autoFixFn runtypes.AutoFixFn, executeFn ExecuteFn) *Runner {
	return &Runner{
		cfg:       cfg,
		cmdRunner: cmdRunner,
		autoFixFn: autoFixFn,
		executeFn: executeFn,
	}
}

// Failures returns the accumulated validation failure summaries from this run.
func (r *Runner) Failures() []string {
	return r.failures
}

// ResetFailures clears accumulated validation failure summaries.
func (r *Runner) ResetFailures() {
	r.failures = []string{}
}

// RunDirect executes validation commands directly via the injected command runner.
// Returns a claude.Result for compatibility with the existing runner facade.
func (r *Runner) RunDirect(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
	for _, command := range commands {
		stdout, stderr, exitCode, err := r.cmdRunner(ctx, command, workDir)
		if err != nil {
			return nil, fmt.Errorf("validation command %q: %w", command, err)
		}

		if exitCode != 0 {
			failureOutput := fmt.Sprintf("Command failed: %s (exit code %d)\n", command, exitCode)
			if stdout != "" {
				failureOutput += "\nStdout:\n" + stdout
			}
			if stderr != "" {
				failureOutput += "\nStderr:\n" + stderr
			}
			return &claude.Result{
				Success:  false,
				Output:   failureOutput,
				ExitCode: exitCode,
			}, nil
		}
	}

	return &claude.Result{
		Success: true,
		Output:  "VALIDATION_PASSED",
	}, nil
}

// RunWithRecovery runs validation with a recovery mechanism.
// On validation failure, it first attempts trivial auto-fixes (gofmt/goimports),
// then falls back to Claude-based fixes. Retry depth is capped by MaxValidationRetries.
func (r *Runner) RunWithRecovery(ctx context.Context, bc *runtypes.BeadContext) error {
	err := r.runValidation(ctx, bc)
	if err == nil {
		return nil
	}

	// Only recover from ErrValidationFailed, not execution errors
	if !errors.Is(err, ErrValidationFailed) {
		return err
	}

	maxRetries := r.cfg.Validation.MaxValidationRetries
	if maxRetries <= 0 {
		return err
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		bc.Result.ValidationRetried = true

		// Step 1: Try trivial auto-fix before invoking Claude
		if r.autoFixFn != nil {
			_ = r.autoFixFn(bc.StartCommit)

			// Re-validate after auto-fix
			if valErr := r.runValidation(ctx, bc); valErr == nil {
				bc.Result.TrivialAutoFixed = true
				return nil
			}
		}

		// Step 2: Auto-fix didn't resolve it — invoke Claude for a fix
		if r.executeFn != nil {
			success := r.executeFn(ctx, bc)

			if !success {
				continue
			}

			// Re-validate after Claude fix
			if valErr := r.runValidation(ctx, bc); valErr == nil {
				return nil
			}
		}
	}

	return err
}

// Validate runs validation commands and updates the bead context result.
// On failure, it accumulates failure summaries accessible via Failures()
// and returns ErrValidationFailed. On success, it sets bc.Result.Validated
// and bc.Result.ValidationMode.
func (r *Runner) Validate(ctx context.Context, bc *runtypes.BeadContext) error {
	return r.runValidation(ctx, bc)
}

// runValidation runs validation commands and updates the bead context result.
func (r *Runner) runValidation(ctx context.Context, bc *runtypes.BeadContext) error {
	if !r.cfg.Validation.Enabled {
		return nil
	}

	for _, command := range r.cfg.Validation.Commands {
		stdout, stderr, exitCode, err := r.cmdRunner(ctx, command, bc.PromptCtx.WorkDir)
		if err != nil {
			return fmt.Errorf("validation command %q: %w", command, err)
		}

		if exitCode != 0 {
			failureOutput := fmt.Sprintf("Command failed: %s (exit code %d)\n", command, exitCode)
			if stdout != "" {
				failureOutput += "\nStdout:\n" + stdout
			}
			if stderr != "" {
				failureOutput += "\nStderr:\n" + stderr
			}

			// Extract and accumulate validation failure summary
			if summary := ExtractValidationSummary(failureOutput); summary != "" {
				r.failures = append(r.failures, summary)
			}

			bc.Result.Output += "\n\n=== VALIDATION OUTPUT ===\n" + failureOutput
			return ErrValidationFailed
		}
	}

	bc.Result.Validated = true
	bc.Result.ValidationMode = "direct"
	return nil
}

// maxValidationSummaryLen caps the length of extracted validation summaries.
const maxValidationSummaryLen = 500

// vetDiagnosticPattern matches go vet diagnostic lines like:
// ./file.go:10:6: x declared and not used
var vetDiagnosticPattern = regexp.MustCompile(`^\./[^:]+:\d+:\d+: .+`)

// ExtractValidationSummary extracts key error lines from go test/vet output.
// It returns test failure names (--- FAIL: lines), package failure lines (FAIL\t),
// and go vet diagnostic lines. The result is capped at 500 characters.
func ExtractValidationSummary(failureOutput string) string {
	if failureOutput == "" {
		return ""
	}

	var lines []string
	for _, line := range strings.Split(failureOutput, "\n") {
		if strings.HasPrefix(line, "--- FAIL:") {
			lines = append(lines, line)
		} else if strings.HasPrefix(line, "FAIL\t") {
			lines = append(lines, line)
		} else if vetDiagnosticPattern.MatchString(line) {
			lines = append(lines, line)
		}
	}

	result := strings.Join(lines, "\n")
	if len(result) > maxValidationSummaryLen {
		result = result[:maxValidationSummaryLen]
	}
	return result
}
