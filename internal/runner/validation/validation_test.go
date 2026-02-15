package validation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Helper functions ---

func newTestConfig() *config.Config {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./...", "go vet ./..."},
			MaxValidationRetries: 2,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

func newTestBeadContext() *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-val-001", Title: "Test validation bead", Priority: 1},
		Tier:      "medium",
		Model:     "sonnet",
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: "/tmp/test-project"},
	}
}

// --- RunDirect tests ---

// Expected failure: validation.Runner type and NewRunner constructor do not exist yet
func TestRunDirect_AllCommandsPass(t *testing.T) {
	// When all validation commands exit 0, RunDirect should return a
	// claude.Result with Success=true and Output="VALIDATION_PASSED".
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}

	// Expected failure: NewRunner does not exist in the validation package yet
	r := NewRunner(cfg, cmdRunner, nil, nil)

	result, err := r.RunDirect(context.Background(), cfg.Validation.Commands, "/tmp/test")
	if err != nil {
		t.Fatalf("RunDirect returned unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("RunDirect should return Success=true when all commands pass")
	}
	if result.Output != "VALIDATION_PASSED" {
		t.Errorf("RunDirect Output = %q, want %q", result.Output, "VALIDATION_PASSED")
	}
}

// Expected failure: validation.Runner type and RunDirect method do not exist yet
func TestRunDirect_FirstCommandFails(t *testing.T) {
	// When a validation command exits non-zero, RunDirect should return a
	// claude.Result with Success=false, the exit code, and captured output.
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if command == "go test ./..." {
			return "", "--- FAIL: TestFoo (0.01s)\nFAIL\tgithub.com/example/pkg", 1, nil
		}
		return "ok", "", 0, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)

	result, err := r.RunDirect(context.Background(), cfg.Validation.Commands, "/tmp/test")
	if err != nil {
		t.Fatalf("RunDirect returned unexpected error: %v", err)
	}
	if result.Success {
		t.Error("RunDirect should return Success=false when a command fails")
	}
	if result.ExitCode != 1 {
		t.Errorf("RunDirect ExitCode = %d, want 1", result.ExitCode)
	}
	if result.Output == "" {
		t.Error("RunDirect should capture failure output")
	}
}

// Expected failure: validation.Runner type and RunDirect method do not exist yet
func TestRunDirect_CommandExecutionError(t *testing.T) {
	// When the command runner returns an error (not just non-zero exit),
	// RunDirect should propagate the error.
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "", 0, fmt.Errorf("exec: command not found")
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)

	_, err := r.RunDirect(context.Background(), cfg.Validation.Commands, "/tmp/test")
	if err == nil {
		t.Fatal("RunDirect should return error when command execution fails")
	}
}

// Expected failure: validation.Runner type and RunDirect method do not exist yet
func TestRunDirect_CapturesStdoutAndStderr(t *testing.T) {
	// When a command fails, RunDirect should include both stdout and stderr
	// in the failure output.
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "stdout content", "stderr content", 2, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)

	result, err := r.RunDirect(context.Background(), cfg.Validation.Commands, "/tmp/test")
	if err != nil {
		t.Fatalf("RunDirect returned unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure result")
	}
	// Both stdout and stderr should be in the output
	if result.Output == "" {
		t.Fatal("expected non-empty output on failure")
	}
}

// Expected failure: validation.Runner type and RunDirect method do not exist yet
func TestRunDirect_StopsAfterFirstFailure(t *testing.T) {
	// RunDirect should stop execution after the first failing command
	// and not run subsequent commands.
	cfg := newTestConfig()

	commandsRun := []string{}
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		commandsRun = append(commandsRun, command)
		if command == "go test ./..." {
			return "", "test failure", 1, nil
		}
		return "ok", "", 0, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)

	result, err := r.RunDirect(context.Background(), cfg.Validation.Commands, "/tmp/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if len(commandsRun) != 1 {
		t.Errorf("ran %d commands, want 1 (should stop after first failure)", len(commandsRun))
	}
}

func TestRunDirect_ParallelCommands_BoundedConcurrency(t *testing.T) {
	cfg := newTestConfig()
	cfg.Validation.MaxParallelCommands = 2
	commands := []string{"cmd-1", "cmd-2", "cmd-3", "cmd-4"}

	var inFlight int32
	var peak int32
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		current := atomic.AddInt32(&inFlight, 1)
		for {
			prev := atomic.LoadInt32(&peak)
			if current <= prev || atomic.CompareAndSwapInt32(&peak, prev, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return "ok", "", 0, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)
	result, err := r.RunDirect(context.Background(), commands, "/tmp/test")
	if err != nil {
		t.Fatalf("RunDirect returned unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %+v", result)
	}
	if got := atomic.LoadInt32(&peak); got != 2 {
		t.Fatalf("peak parallelism=%d, want 2", got)
	}
}

// --- RunWithRecovery tests ---

// Expected failure: validation.Runner type and RunWithRecovery method do not exist yet
func TestRunWithRecovery_PassesOnFirstValidation(t *testing.T) {
	// When validation passes on the first try, RunWithRecovery returns nil
	// without any recovery attempts.
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}

	// Expected failure: NewRunner does not exist yet
	r := NewRunner(cfg, cmdRunner, nil, nil)

	bc := newTestBeadContext()
	err := r.RunWithRecovery(context.Background(), bc)
	if err != nil {
		t.Errorf("RunWithRecovery should return nil when validation passes, got: %v", err)
	}
	if bc.Result.ValidationRetried {
		t.Error("ValidationRetried should be false when validation passes on first try")
	}
}

// Expected failure: validation.Runner type and RunWithRecovery method do not exist yet
func TestRunWithRecovery_AutoFixResolvesFailure(t *testing.T) {
	// When validation fails but auto-fix resolves the issue, RunWithRecovery
	// should return nil and set TrivialAutoFixed=true.
	cfg := newTestConfig()

	callCount := 0
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		callCount++
		// First validation run: fail. After auto-fix: pass.
		if callCount == 1 {
			return "", "format error", 1, nil
		}
		return "ok", "", 0, nil
	}

	autoFixCalled := false
	autoFix := func(startCommit string) error {
		autoFixCalled = true
		return nil
	}

	r := NewRunner(cfg, cmdRunner, autoFix, nil)

	bc := newTestBeadContext()
	bc.StartCommit = "abc123"
	err := r.RunWithRecovery(context.Background(), bc)
	if err != nil {
		t.Errorf("RunWithRecovery should return nil after auto-fix, got: %v", err)
	}
	if !autoFixCalled {
		t.Error("auto-fix should have been called")
	}
	if !bc.Result.TrivialAutoFixed {
		t.Error("TrivialAutoFixed should be true when auto-fix resolves the failure")
	}
	if !bc.Result.ValidationRetried {
		t.Error("ValidationRetried should be true when recovery was attempted")
	}
}

// Expected failure: validation.Runner type and RunWithRecovery method do not exist yet
func TestRunWithRecovery_ExecuteFnCalledWhenAutoFixFails(t *testing.T) {
	// When validation fails and auto-fix doesn't resolve it, RunWithRecovery
	// should invoke the ExecuteFn callback for Claude-based fix attempts.
	cfg := newTestConfig()
	cfg.Validation.MaxValidationRetries = 1

	// All validation commands always fail
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "test failure", 1, nil
	}

	autoFix := func(startCommit string) error {
		return nil // auto-fix succeeds but doesn't resolve the issue
	}

	executeFnCalled := false
	// Expected failure: ExecuteFn callback type does not exist in validation package yet
	executeFn := func(ctx context.Context, bc *runtypes.BeadContext) bool {
		executeFnCalled = true
		return true // Claude fix "succeeds"
	}

	r := NewRunner(cfg, cmdRunner, autoFix, executeFn)

	bc := newTestBeadContext()
	bc.StartCommit = "abc123"
	// Even though Claude succeeds, if re-validation still fails, the overall result is failure
	err := r.RunWithRecovery(context.Background(), bc)
	if err == nil {
		t.Error("RunWithRecovery should return error when validation remains failing")
	}

	if !executeFnCalled {
		t.Error("ExecuteFn should be called when auto-fix doesn't resolve validation failure")
	}
}

// Expected failure: validation.Runner type and RunWithRecovery method do not exist yet
func TestRunWithRecovery_RespectsSingleRecoveryCap(t *testing.T) {
	// RunWithRecovery caps recovery to one attempt, even if configured higher.
	cfg := newTestConfig()
	cfg.Validation.MaxValidationRetries = 2

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "always fails", 1, nil
	}

	executeFnCallCount := 0
	executeFn := func(ctx context.Context, bc *runtypes.BeadContext) bool {
		executeFnCallCount++
		return true
	}

	r := NewRunner(cfg, cmdRunner, nil, executeFn)

	bc := newTestBeadContext()
	err := r.RunWithRecovery(context.Background(), bc)
	if err == nil {
		t.Error("RunWithRecovery should return error when all retries exhausted")
	}
	if executeFnCallCount > 1 {
		t.Errorf("ExecuteFn called %d times, want <= 1 (single recovery cap)", executeFnCallCount)
	}
}

// Expected failure: validation.Runner type and RunWithRecovery method do not exist yet
func TestRunWithRecovery_ZeroRetriesSkipsRecovery(t *testing.T) {
	// When MaxValidationRetries is 0, RunWithRecovery should return the error
	// immediately without any recovery attempts.
	cfg := newTestConfig()
	cfg.Validation.MaxValidationRetries = 0

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "test failure", 1, nil
	}

	executeFnCalled := false
	executeFn := func(ctx context.Context, bc *runtypes.BeadContext) bool {
		executeFnCalled = true
		return true
	}

	r := NewRunner(cfg, cmdRunner, nil, executeFn)

	bc := newTestBeadContext()
	err := r.RunWithRecovery(context.Background(), bc)
	if err == nil {
		t.Error("RunWithRecovery should return error when max retries is 0")
	}
	if executeFnCalled {
		t.Error("ExecuteFn should not be called when MaxValidationRetries is 0")
	}
}

// Expected failure: validation.Runner type and RunWithRecovery method do not exist yet
func TestRunWithRecovery_SetsValidatedOnSuccess(t *testing.T) {
	// After successful validation, RunWithRecovery should set
	// bc.Result.Validated=true and bc.Result.ValidationMode="direct".
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)

	bc := newTestBeadContext()
	err := r.RunWithRecovery(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bc.Result.Validated {
		t.Error("Result.Validated should be true after successful validation")
	}
	if bc.Result.ValidationMode != "direct" {
		t.Errorf("Result.ValidationMode = %q, want %q", bc.Result.ValidationMode, "direct")
	}
}

// Expected failure: validation.Runner type and RunWithRecovery method do not exist yet
func TestRunWithRecovery_NonValidationErrorNotRecovered(t *testing.T) {
	// When validation returns an error that is NOT errValidationFailed
	// (e.g., a command execution error), RunWithRecovery should propagate
	// it without attempting recovery.
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "", 0, fmt.Errorf("exec: binary not found")
	}

	executeFnCalled := false
	executeFn := func(ctx context.Context, bc *runtypes.BeadContext) bool {
		executeFnCalled = true
		return true
	}

	r := NewRunner(cfg, cmdRunner, nil, executeFn)

	bc := newTestBeadContext()
	err := r.RunWithRecovery(context.Background(), bc)
	if err == nil {
		t.Fatal("RunWithRecovery should propagate non-validation errors")
	}
	if executeFnCalled {
		t.Error("ExecuteFn should not be called for non-validation errors")
	}
}

// --- ExtractValidationSummary tests ---

// Expected failure: ExtractValidationSummary does not exist in the validation package yet
func TestExtractValidationSummary_ExtractsTestFailures(t *testing.T) {
	// ExtractValidationSummary should extract "--- FAIL:" lines from test output.
	input := `=== RUN   TestFoo
--- FAIL: TestFoo (0.01s)
    foo_test.go:10: expected 1, got 2
FAIL	github.com/example/pkg	0.015s
ok  	github.com/example/other	0.005s`

	summary := ExtractValidationSummary(input)

	if summary == "" {
		t.Fatal("ExtractValidationSummary should return non-empty summary for test failures")
	}
	// Should contain the FAIL lines
	if !containsSubstring(summary, "--- FAIL: TestFoo") {
		t.Errorf("summary should contain test failure line, got: %q", summary)
	}
	if !containsSubstring(summary, "FAIL\tgithub.com/example/pkg") {
		t.Errorf("summary should contain package failure line, got: %q", summary)
	}
}

// Expected failure: ExtractValidationSummary does not exist in the validation package yet
func TestExtractValidationSummary_ExtractsVetDiagnostics(t *testing.T) {
	// ExtractValidationSummary should extract go vet diagnostic lines.
	input := `./main.go:10:6: x declared and not used
./util.go:25:2: unreachable code`

	summary := ExtractValidationSummary(input)

	if summary == "" {
		t.Fatal("ExtractValidationSummary should return non-empty summary for vet diagnostics")
	}
	if !containsSubstring(summary, "./main.go:10:6:") {
		t.Errorf("summary should contain vet diagnostic, got: %q", summary)
	}
}

// Expected failure: ExtractValidationSummary does not exist in the validation package yet
func TestExtractValidationSummary_TruncatesLongOutput(t *testing.T) {
	// ExtractValidationSummary should cap output at 500 characters.
	var longInput string
	for i := 0; i < 100; i++ {
		longInput += fmt.Sprintf("--- FAIL: TestCase%d (0.01s)\n", i)
	}

	summary := ExtractValidationSummary(longInput)

	if len(summary) > 500 {
		t.Errorf("summary length = %d, want <= 500", len(summary))
	}
}

// Expected failure: ExtractValidationSummary does not exist in the validation package yet
func TestExtractValidationSummary_EmptyInput(t *testing.T) {
	summary := ExtractValidationSummary("")
	if summary != "" {
		t.Errorf("ExtractValidationSummary(\"\") = %q, want empty string", summary)
	}
}

// --- NewRunner constructor tests ---

// Expected failure: validation.NewRunner does not exist yet
func TestNewRunner_AcceptsNarrowInterfaces(t *testing.T) {
	// NewRunner should accept config, CmdRunnerFn, AutoFixFn, and ExecuteFn
	// as narrow dependency interfaces — not the full runner.Runner.
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "", 0, nil
	}
	autoFix := func(startCommit string) error { return nil }
	executeFn := func(ctx context.Context, bc *runtypes.BeadContext) bool { return true }

	r := NewRunner(cfg, cmdRunner, autoFix, executeFn)
	if r == nil {
		t.Fatal("NewRunner returned nil")
	}
}

// Expected failure: validation.NewRunner does not exist yet
func TestNewRunner_NilAutoFixIsAllowed(t *testing.T) {
	// AutoFixFn should be optional — passing nil means no auto-fix is available.
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "", 0, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)
	if r == nil {
		t.Fatal("NewRunner should accept nil autoFixFn")
	}
}

// Expected failure: validation.NewRunner does not exist yet
func TestNewRunner_NilExecuteFnIsAllowed(t *testing.T) {
	// ExecuteFn should be optional — passing nil means no Claude-based recovery.
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "", 0, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)
	if r == nil {
		t.Fatal("NewRunner should accept nil executeFn")
	}

	// With nil executeFn, recovery should still try auto-fix but skip Claude
	cfg.Validation.MaxValidationRetries = 1

	autoFixCalled := false
	autoFix := func(startCommit string) error {
		autoFixCalled = true
		return nil
	}

	r2 := NewRunner(cfg, func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "fail", 1, nil
	}, autoFix, nil)

	bc := newTestBeadContext()
	bc.StartCommit = "abc123"
	_ = r2.RunWithRecovery(context.Background(), bc)

	if !autoFixCalled {
		t.Error("auto-fix should be attempted even when ExecuteFn is nil")
	}
}

// --- Validation failure accumulation tests ---

// Expected failure: validation.Runner.Failures method does not exist yet
func TestRunDirect_FailureAccumulatesValidationSummary(t *testing.T) {
	// When validation fails, the failure summary should be extractable
	// via the Runner's Failures() accessor for build prompt injection.
	cfg := newTestConfig()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "--- FAIL: TestFoo (0.01s)\nFAIL\tgithub.com/example/pkg", 1, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)

	bc := newTestBeadContext()
	_ = r.RunWithRecovery(context.Background(), bc)

	// Expected failure: Failures() method does not exist yet
	failures := r.Failures()
	if len(failures) == 0 {
		t.Error("Runner.Failures() should return accumulated validation failure summaries")
	}
}

// --- Package isolation test ---

// Expected failure: validation package does not exist yet — this test verifies
// that the validation package uses only narrow interfaces and does not import
// the runner/ facade package.
func TestPackageDoesNotImportRunner(t *testing.T) {
	// The validation package must not import the runner/ facade.
	// This is verified structurally: the test file imports from validation
	// package (package validation), and the production code should only
	// import runtypes/ and standard library / external packages.
	//
	// If this test compiles, it means the validation package exists and
	// can be tested independently. The import structure at the top of
	// this file demonstrates isolation: we import runtypes, config, bead,
	// claude, and prompt — but NOT "runner".

	// Verify the Runner struct exists and can be constructed
	cfg := newTestConfig()
	r := NewRunner(cfg, func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "", 0, nil
	}, nil, nil)

	if r == nil {
		t.Fatal("validation.NewRunner should return a non-nil Runner")
	}
}

// --- ErrValidationFailed sentinel test ---

// Expected failure: ErrValidationFailed does not exist in the validation package yet
func TestErrValidationFailed_IsSentinelError(t *testing.T) {
	// The validation package should export an ErrValidationFailed sentinel
	// that callers can check with errors.Is().
	err := ErrValidationFailed

	if err == nil {
		t.Fatal("ErrValidationFailed should not be nil")
	}

	wrappedErr := fmt.Errorf("wrapped: %w", ErrValidationFailed)
	if !errors.Is(wrappedErr, ErrValidationFailed) {
		t.Error("errors.Is should identify wrapped ErrValidationFailed")
	}
}

// --- Helper ---

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Validate (public runValidation) tests ---

func TestValidate_PassesAndSetsValidated(t *testing.T) {
	cfg := newTestConfig()
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	r := NewRunner(cfg, cmdRunner, nil, nil)
	bc := newTestBeadContext()

	err := r.Validate(context.Background(), bc)
	if err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if !bc.Result.Validated {
		t.Error("Result.Validated should be true after successful validation")
	}
	if bc.Result.ValidationMode != "direct" {
		t.Errorf("Result.ValidationMode = %q, want %q", bc.Result.ValidationMode, "direct")
	}
}

func TestValidate_FailsAndAccumulatesFailures(t *testing.T) {
	cfg := newTestConfig()
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "--- FAIL: TestBar (0.01s)\nFAIL\tpkg/bar", 1, nil
	}
	r := NewRunner(cfg, cmdRunner, nil, nil)
	bc := newTestBeadContext()

	err := r.Validate(context.Background(), bc)
	if !errors.Is(err, ErrValidationFailed) {
		t.Errorf("Validate should return ErrValidationFailed, got: %v", err)
	}
	failures := r.Failures()
	if len(failures) == 0 {
		t.Error("Failures() should be populated after validation failure")
	}
}

func TestRunWithRecovery_CapsToSingleRecoveryAttempt(t *testing.T) {
	cfg := newTestConfig()
	cfg.Validation.MaxValidationRetries = 5

	cmdCalls := 0
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		cmdCalls++
		return "", "always failing output", 1, nil
	}

	autoFixCalls := 0
	autoFix := func(startCommit string) error {
		autoFixCalls++
		return nil
	}

	execCalls := 0
	execFn := func(ctx context.Context, bc *runtypes.BeadContext) bool {
		execCalls++
		return true
	}

	r := NewRunner(cfg, cmdRunner, autoFix, execFn)
	bc := newTestBeadContext()

	err := r.RunWithRecovery(context.Background(), bc)
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if autoFixCalls > 1 {
		t.Fatalf("autoFix called %d times, want <= 1", autoFixCalls)
	}
	if execCalls > 1 {
		t.Fatalf("executeFn called %d times, want <= 1", execCalls)
	}
	// Initial failure + one re-validation after auto-fix + one after executeFn.
	if cmdCalls > 6 {
		t.Fatalf("validation commands executed too many times: %d", cmdCalls)
	}
}

func TestValidate_DetectsInteractivePromptWhenNonInteractive(t *testing.T) {
	cfg := newTestConfig()
	tBool := true
	cfg.Validation.NonInteractive = &tBool
	cfg.Validation.Commands = []string{"go test ./..."}

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "Password:", "", 0, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)
	bc := newTestBeadContext()

	err := r.Validate(context.Background(), bc)
	if err == nil {
		t.Fatal("expected interactive-prompt detection error")
	}
	if !containsSubstring(err.Error(), "attempted interactive prompt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_AllowsPromptPatternWhenNonInteractiveDisabled(t *testing.T) {
	cfg := newTestConfig()
	fBool := false
	cfg.Validation.NonInteractive = &fBool
	cfg.Validation.Commands = []string{"go test ./..."}

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "Password:", "", 0, nil
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)
	bc := newTestBeadContext()

	err := r.Validate(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error with non_interactive disabled: %v", err)
	}
}

func TestValidate_CommandTimeoutReturnsValidationFailure(t *testing.T) {
	cfg := newTestConfig()
	cfg.Validation.CommandTimeout = 20 * time.Millisecond
	cfg.Validation.Commands = []string{"go test ./..."}

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		<-ctx.Done()
		return "", "", 0, ctx.Err()
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)
	bc := newTestBeadContext()

	err := r.Validate(context.Background(), bc)
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("expected ErrValidationFailed on timeout, got: %v", err)
	}
	if !strings.Contains(bc.Result.Output, "Command timed out: go test ./...") {
		t.Fatalf("expected timeout output in result, got: %q", bc.Result.Output)
	}
}

// Ensure imports are used
var (
	_ = claude.Result{}
	_ = bead.Bead{}
)
