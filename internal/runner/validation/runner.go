package validation

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

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
	cfg               *config.Config
	cmdRunner         runtypes.CmdRunnerFn
	autoFixFn         runtypes.AutoFixFn
	executeFn         ExecuteFn
	failures          []string // accumulated validation failure summaries
	lastFailureOutput string
}

type commandResult struct {
	command  string
	stdout   string
	stderr   string
	exitCode int
	err      error
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
	results, err := r.runCommands(ctx, commands, workDir)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		if result.err != nil {
			return nil, fmt.Errorf("validation command %q: %w", result.command, result.err)
		}
		if r.cfg.Validation.IsNonInteractive() {
			if prompt := detectInteractivePrompt(result.stdout, result.stderr); prompt != "" {
				return nil, fmt.Errorf("validation command %q attempted interactive prompt: %s", result.command, prompt)
			}
		}
		if result.exitCode != 0 {
			failureOutput := formatFailureOutput(result.command, result.exitCode, result.stdout, result.stderr)
			return &claude.Result{
				Success:  false,
				Output:   failureOutput,
				ExitCode: result.exitCode,
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
	return r.RunWithRecoveryForCommands(ctx, bc, r.cfg.Validation.FastCommandsOrDefault(), "fast")
}

func (r *Runner) RunWithRecoveryForCommands(ctx context.Context, bc *runtypes.BeadContext, commands []string, mode string) error {
	err := r.runValidationWithCommands(ctx, bc, commands, mode)
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
	// Keep validation recovery bounded to one fix loop to avoid repeated
	// low-signal retries in unattended runs.
	if maxRetries > 1 {
		maxRetries = 1
	}

	lastFailure := r.lastFailureOutput

	for attempt := 0; attempt < maxRetries; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("validation recovery aborted: %w", ctxErr)
		}
		bc.Result.ValidationRetried = true

		// Step 1: Try trivial auto-fix before invoking Claude
		if r.autoFixFn != nil {
			_ = r.autoFixFn(bc.StartCommit)

			// Re-validate after auto-fix
			if valErr := r.runValidationWithCommands(ctx, bc, commands, mode); valErr == nil {
				bc.Result.TrivialAutoFixed = true
				return nil
			} else if !errors.Is(valErr, ErrValidationFailed) {
				return valErr
			} else if errors.Is(valErr, ErrValidationFailed) {
				lastFailure = r.lastFailureOutput
			}
		}

		// Step 2: Auto-fix didn't resolve it — invoke Claude for a fix
		if r.executeFn != nil {
			success := r.executeFn(ctx, bc)

			if !success {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return fmt.Errorf("validation recovery aborted: %w", ctxErr)
				}
				continue
			}

			// Re-validate after Claude fix
			if valErr := r.runValidationWithCommands(ctx, bc, commands, mode); valErr == nil {
				return nil
			} else if !errors.Is(valErr, ErrValidationFailed) {
				return valErr
			} else if errors.Is(valErr, ErrValidationFailed) && r.lastFailureOutput == lastFailure {
				return valErr
			} else if errors.Is(valErr, ErrValidationFailed) {
				lastFailure = r.lastFailureOutput
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
	return r.runValidationWithCommands(ctx, bc, r.cfg.Validation.FastCommandsOrDefault(), "fast")
}

// runValidation runs validation commands and updates the bead context result.
func (r *Runner) runValidationWithCommands(ctx context.Context, bc *runtypes.BeadContext, commands []string, mode string) error {
	if !r.cfg.Validation.Enabled {
		return nil
	}

	results, err := r.runCommands(ctx, commands, bc.PromptCtx.WorkDir)
	if err != nil {
		return err
	}
	for _, result := range results {
		if result.err != nil {
			// If the parent bead context has already expired/canceled, surface that
			// directly so callers can distinguish bead budget exhaustion from a
			// command-level validation timeout.
			if parentErr := ctx.Err(); parentErr != nil {
				return fmt.Errorf("validation command %q aborted: %w", result.command, parentErr)
			}
			if errors.Is(result.err, context.DeadlineExceeded) {
				failureOutput := formatTimeoutFailureOutput(result.command, r.cfg.Validation.CommandTimeout, result.stdout, result.stderr)
				r.lastFailureOutput = failureOutput
				bc.Result.Output += runtypes.ValidationOutputHeader + failureOutput
				return ErrValidationFailed
			}
			if errors.Is(result.err, context.Canceled) {
				return fmt.Errorf("validation command %q aborted: %w", result.command, context.Canceled)
			}
			return fmt.Errorf("validation command %q: %w", result.command, result.err)
		}
		if r.cfg.Validation.IsNonInteractive() {
			if prompt := detectInteractivePrompt(result.stdout, result.stderr); prompt != "" {
				return fmt.Errorf("validation command %q attempted interactive prompt: %s", result.command, prompt)
			}
		}
		if result.exitCode != 0 {
			failureOutput := formatFailureOutput(result.command, result.exitCode, result.stdout, result.stderr)
			if summary := ExtractValidationSummary(failureOutput); summary != "" {
				r.failures = append(r.failures, summary)
			}
			r.lastFailureOutput = failureOutput
			bc.Result.Output += runtypes.ValidationOutputHeader + runtypes.TruncateOutput(failureOutput)
			return ErrValidationFailed
		}
	}

	r.lastFailureOutput = ""
	bc.Result.Validated = true
	if mode == "" || mode == "fast" {
		mode = "direct"
	}
	bc.Result.ValidationMode = mode
	return nil
}

func (r *Runner) runCommands(ctx context.Context, commands []string, workDir string) ([]commandResult, error) {
	if len(commands) == 0 {
		return nil, nil
	}
	maxParallel := r.cfg.Validation.MaxParallelCommands
	if maxParallel <= 1 || len(commands) == 1 {
		results := make([]commandResult, 0, len(commands))
		for _, command := range commands {
			result := r.runSingleCommand(ctx, command, workDir)
			results = append(results, result)
			if result.err != nil || result.exitCode != 0 {
				break
			}
		}
		return results, nil
	}
	if maxParallel > len(commands) {
		maxParallel = len(commands)
	}

	results := make([]commandResult, len(commands))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallel)
	for i, command := range commands {
		i := i
		command := command
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			results[i] = r.runSingleCommand(ctx, command, workDir)
			<-sem
		}()
	}
	wg.Wait()
	return results, nil
}

func (r *Runner) runSingleCommand(ctx context.Context, command, workDir string) commandResult {
	commandCtx := ctx
	cancel := func() {}
	if r.cfg.Validation.CommandTimeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, r.cfg.Validation.CommandTimeout)
	}
	defer cancel()

	stdout, stderr, exitCode, err := r.cmdRunner(commandCtx, command, workDir)
	if err != nil && errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
		err = context.DeadlineExceeded
	}
	return commandResult{
		command:  command,
		stdout:   stdout,
		stderr:   stderr,
		exitCode: exitCode,
		err:      err,
	}
}

// maxValidationSummaryLen caps the length of extracted validation summaries.
const maxValidationSummaryLen = 500

// vetDiagnosticPattern matches go vet diagnostic lines like:
// ./file.go:10:6: x declared and not used
var vetDiagnosticPattern = regexp.MustCompile(`^\./[^:]+:\d+:\d+: .+`)

var failHeaderPattern = regexp.MustCompile(`^\s*--- FAIL:\s+(.+?)(\s+\(|$)`)

var assertionLinePattern = regexp.MustCompile(`^\s*.+\.go:\d+:`)

var interactivePromptPatterns = []string{
	"password:",
	"passphrase:",
	"enter input:",
	"do you want to continue",
	"waiting for user input",
}

func detectInteractivePrompt(stdout, stderr string) string {
	combined := strings.ToLower(stdout + "\n" + stderr)
	for _, pattern := range interactivePromptPatterns {
		if strings.Contains(combined, pattern) {
			return pattern
		}
	}
	return ""
}

func formatFailureOutput(command string, exitCode int, stdout, stderr string) string {
	failureOutput := fmt.Sprintf("Command failed: %s (exit code %d)\n", command, exitCode)
	if stdout != "" {
		failureOutput += "\nStdout:\n" + stdout
	}
	if stderr != "" {
		failureOutput += "\nStderr:\n" + stderr
	}
	return failureOutput
}

func formatTimeoutFailureOutput(command string, timeout time.Duration, stdout, stderr string) string {
	failureOutput := fmt.Sprintf("Command timed out: %s (timeout %s)\n", command, timeout)
	if stdout != "" {
		failureOutput += "\nStdout:\n" + stdout
	}
	if stderr != "" {
		failureOutput += "\nStderr:\n" + stderr
	}
	return failureOutput
}

// ExtractValidationSummary extracts key error lines from go test/vet output.
// It returns test failure names (--- FAIL: lines), package failure lines (FAIL\t),
// and go vet diagnostic lines. The result is capped at 500 characters.
func ExtractValidationSummary(failureOutput string) string {
	if failureOutput == "" {
		return "PASS: all validations passed"
	}

	type failureEntry struct {
		testName   string
		assertions []string
	}

	var (
		lines      []string
		entries    []failureEntry
		packages   []string
		currentRef *failureEntry
	)

	for _, line := range strings.Split(failureOutput, "\n") {
		if match := failHeaderPattern.FindStringSubmatch(line); match != nil {
			lines = append(lines, line)
			entry := failureEntry{testName: strings.TrimSpace(match[1])}
			entries = append(entries, entry)
			currentRef = &entries[len(entries)-1]
			continue
		}
		if strings.HasPrefix(line, "FAIL\t") {
			lines = append(lines, line)
			fields := strings.Split(line, "\t")
			if len(fields) > 1 {
				pkg := strings.TrimSpace(fields[1])
				if pkg != "" {
					packages = append(packages, pkg)
				}
			}
			continue
		}
		if vetDiagnosticPattern.MatchString(line) {
			lines = append(lines, line)
			continue
		}
		if currentRef != nil && assertionLinePattern.MatchString(line) {
			currentRef.assertions = append(currentRef.assertions, strings.TrimSpace(line))
		}
	}

	if len(entries) == 0 && len(lines) == 0 {
		return "PASS: all validations passed"
	}

	output := make([]string, 0, len(lines)+len(entries)+1)
	if len(entries) > 0 || len(lines) > 0 {
		output = append(output, "FAILURES:")
	}

	if len(entries) > 0 {
		pkgIdentifier := formatPackageIdentifier(firstPackage(packages))
		detailLines := make([]string, 0, len(entries))
		for _, entry := range entries {
			if len(entry.assertions) == 0 {
				detailLines = append(detailLines, fmt.Sprintf("package: %s | test: %s", pkgIdentifier, entry.testName))
				continue
			}
			for _, assertion := range entry.assertions {
				detailLines = append(detailLines, fmt.Sprintf("package: %s | test: %s | assert: %s", pkgIdentifier, entry.testName, assertion))
			}
		}
		output = append(output, detailLines...)
	}

	output = append(output, lines...)

	result := strings.Join(output, "\n")
	if len(result) > maxValidationSummaryLen {
		result = result[:maxValidationSummaryLen]
	}
	return result
}

func firstPackage(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	return packages[0]
}

func formatPackageIdentifier(pkg string) string {
	if pkg == "" {
		return "unknown"
	}
	short := pkg
	if idx := strings.Index(pkg, "/internal/"); idx != -1 {
		short = pkg[idx+1:]
	}
	base := path.Base(pkg)
	if short == base {
		return short
	}
	return fmt.Sprintf("%s (%s)", short, base)
}
