package runner

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// setupDirectValidationRunner creates a Runner wired for direct validation tests.
// It tracks both Claude CLI invocations and analyzer calls to verify that
// validation commands run directly (not through Claude) and that the analyzer
// is only invoked on failure.
// The cmdRunner parameter is wired into both the facade's cmdRunnerFn and the
// validationRunner. If nil, a default passing runner is used.
func setupDirectValidationRunner(t *testing.T, cfg *config.Config, cmdRunner runtypes.CmdRunnerFn) (*Runner, *mockClaudeClient, *mockFailureAnalyzer) {
	t.Helper()

	mockClaude := &mockClaudeClient{}
	mockAnalyzer := &mockFailureAnalyzer{}

	if cfg == nil {
		cfg = &config.Config{
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"go test ./...", "go vet ./...", "go build ./..."},
			},
			Preflight: config.PreflightConfig{},
			Claude: config.ClaudeConfig{
				AnalysisTimeout: 30,
			},
		}
	}
	cfg.SetDefaults()

	if cmdRunner == nil {
		cmdRunner = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "ok", "", 0, nil
		}
	}

	var buf strings.Builder
	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg:              cfg,
		router:           mockRouter,
		invoker:          newInvokerForTest(mockRouter, &buf, nil),
		renderer:         &mockRenderer{},
		analyzer:         mockAnalyzer,
		output:           &buf,
		cmdRunnerFn:      cmdRunner,
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}

	return r, mockClaude, mockAnalyzer
}

func newBeadContext(t *testing.T) *runtypes.BeadContext {
	t.Helper()
	return &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-direct-val", Title: "Test Direct Validation"},
		Model:  "sonnet",
		Result: &IterationResult{},
		PromptCtx: &prompt.Context{
			WorkDir:            t.TempDir(),
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}
}

// TestDirectValidation_NoClaudeCLIInvocation verifies AC1: validation commands
// run via exec.Command, not by spawning Claude CLI.
//
// Expected failure: runValidation currently delegates to provider.RunValidation
// which invokes Claude CLI. After implementation, it should execute commands
// directly and never call the Claude client's RunValidation method.
func TestDirectValidation_NoClaudeCLIInvocation(t *testing.T) {
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	r, mockClaude, _ := setupDirectValidationRunner(t, nil, cmdRunner)
	bc := newBeadContext(t)

	// Track all Claude CLI invocations
	claudeRunCalled := false
	claudeStreamRunCalled := false
	claudeValidationCalled := false

	mockClaude.RunFn = func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
		claudeRunCalled = true
		return &claude.Result{Success: true, Output: "ok"}, nil
	}
	mockClaude.StreamRunFn = func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
		claudeStreamRunCalled = true
		return &claude.Result{Success: true, Output: "ok"}, nil
	}
	mockClaude.RunValidationFn = func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
		claudeValidationCalled = true
		return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
	}

	_ = r.runValidation(context.Background(), bc)

	// After implementation, validation should NOT invoke Claude CLI at all.
	// Commands should run directly via exec.Command.
	if claudeValidationCalled {
		t.Error("RunValidation was called on Claude client — validation commands should execute directly via exec.Command, not through Claude CLI")
	}
	if claudeRunCalled {
		t.Error("Run was called on Claude client during validation — validation should not use Claude for command execution")
	}
	if claudeStreamRunCalled {
		t.Error("StreamRun was called on Claude client during validation — validation should not use Claude for command execution")
	}
}

// TestDirectValidation_ExitCodeZeroMeansPass verifies AC2: exit code 0 from
// commands maps to VALIDATION_PASSED.
//
// Expected failure: runValidation currently checks for "VALIDATION_PASSED"
// string in Claude's output. After implementation, it should determine
// pass/fail based on command exit codes (0=pass). The Runner needs a
// cmdRunnerFn field for injecting command execution behavior in tests.
func TestDirectValidation_ExitCodeZeroMeansPass(t *testing.T) {
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)
	bc := newBeadContext(t)

	err := r.runValidation(context.Background(), bc)

	if err != nil {
		t.Errorf("expected no error when all commands exit 0, got: %v", err)
	}
	if !bc.Result.Validated {
		t.Error("expected Validated=true when all commands exit with code 0")
	}
}

// TestDirectValidation_NonZeroExitCodeMeansFailWithStderr verifies AC2:
// non-zero exit code maps to VALIDATION_FAILED with captured stderr.
//
// Expected failure: cmdRunnerFn does not exist on Runner yet. After
// implementation, a non-zero exit code from any command should cause
// validation to fail, and the stderr should be captured in the result output.
func TestDirectValidation_NonZeroExitCodeMeansFailWithStderr(t *testing.T) {
	stderrMsg := "FAIL: TestSomething (0.01s)\n    expected 1, got 2"

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if command == "go test ./..." {
			return "", stderrMsg, 1, nil
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)
	bc := newBeadContext(t)

	err := r.runValidation(context.Background(), bc)

	// Validation should fail with sentinel error
	if err != errValidationFailed {
		t.Errorf("expected errValidationFailed, got: %v", err)
	}

	// The captured stderr should appear in the result output
	if !strings.Contains(bc.Result.Output, stderrMsg) {
		t.Errorf("expected result output to contain stderr %q, got: %q", stderrMsg, bc.Result.Output)
	}
}

// TestDirectValidation_StopsOnFirstFailure verifies that direct validation
// stops executing commands after the first failure (AC2 behavior).
//
// Expected failure: cmdRunnerFn does not exist on Runner yet. After
// implementation, when the first command fails, subsequent commands should
// not be executed.
func TestDirectValidation_StopsOnFirstFailure(t *testing.T) {
	commandsExecuted := []string{}

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		commandsExecuted = append(commandsExecuted, command)
		if command == "go test ./..." {
			return "", "test failure", 1, nil
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)
	bc := newBeadContext(t)

	_ = r.runValidation(context.Background(), bc)

	// go test ./... is the first command and should fail, so go vet and go build should not run
	if len(commandsExecuted) != 1 {
		t.Errorf("expected 1 command executed (stop on first failure), got %d: %v", len(commandsExecuted), commandsExecuted)
	}
	if commandsExecuted[0] != "go test ./..." {
		t.Errorf("expected first command to be 'go test ./...', got %q", commandsExecuted[0])
	}
}

// TestDirectValidation_AnalyzerOnlyCalledOnFailure verifies AC3: Claude is
// only invoked for failure interpretation via the existing analyzer path.
//
// Expected failure: cmdRunnerFn does not exist on Runner yet. After
// implementation, the analyzer should be invoked when validation fails
// but should NOT be invoked when validation succeeds.
func TestDirectValidation_AnalyzerOnlyCalledOnFailure(t *testing.T) {
	tests := []struct {
		name            string
		exitCode        int
		expectAnalyzer  bool
		expectValFailed bool
		description     string
	}{
		{
			name:            "analyzer called on failure",
			exitCode:        1,
			expectAnalyzer:  true,
			expectValFailed: true,
			description:     "When commands fail (exit code 1), analyzer should interpret the failure",
		},
		{
			name:            "analyzer not called on success",
			exitCode:        0,
			expectAnalyzer:  false,
			expectValFailed: false,
			description:     "When commands succeed (exit code 0), analyzer should not be invoked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
				if tt.exitCode != 0 {
					return "", "command failed", tt.exitCode, nil
				}
				return "ok", "", 0, nil
			}
			r, _, mockAnalyzer := setupDirectValidationRunner(t, nil, cmdRunner)
			bc := newBeadContext(t)
			mockAnalyzer.AnalyzeCalls = 0

			err := r.runValidation(context.Background(), bc)

			if tt.expectValFailed {
				if err != errValidationFailed {
					t.Errorf("%s: expected errValidationFailed, got: %v", tt.description, err)
				}
			} else {
				if err != nil {
					t.Errorf("%s: expected no error, got: %v", tt.description, err)
				}
			}

			if tt.expectAnalyzer && mockAnalyzer.AnalyzeCalls == 0 {
				t.Errorf("%s: expected analyzer to be called for failure interpretation, but it was not called", tt.description)
			}
			if !tt.expectAnalyzer && mockAnalyzer.AnalyzeCalls > 0 {
				t.Errorf("%s: expected analyzer NOT to be called on success, but it was called %d times", tt.description, mockAnalyzer.AnalyzeCalls)
			}
		})
	}
}

// TestDirectValidation_ExecutesAllCommandsOnSuccess verifies that when all
// commands succeed, every configured command is actually executed (AC1).
//
// Expected failure: cmdRunnerFn does not exist on Runner yet. After
// implementation, all configured validation commands should be executed
// in order when each succeeds.
func TestDirectValidation_ExecutesAllCommandsOnSuccess(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go build ./...", "go vet ./...", "go test ./..."},
		},
		Preflight: config.PreflightConfig{},
		Claude: config.ClaudeConfig{
			AnalysisTimeout: 30,
		},
	}

	commandsExecuted := []string{}

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		commandsExecuted = append(commandsExecuted, command)
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, cfg, cmdRunner)
	bc := newBeadContext(t)

	err := r.runValidation(context.Background(), bc)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if len(commandsExecuted) != 3 {
		t.Errorf("expected 3 commands executed, got %d: %v", len(commandsExecuted), commandsExecuted)
	}

	expectedOrder := []string{"go build ./...", "go vet ./...", "go test ./..."}
	for i, expected := range expectedOrder {
		if i >= len(commandsExecuted) {
			break
		}
		if commandsExecuted[i] != expected {
			t.Errorf("command[%d]: expected %q, got %q", i, expected, commandsExecuted[i])
		}
	}
}

// TestDirectValidation_WorkDirPassedToCommands verifies that commands are
// executed in the correct working directory (AC1 - exec.Command behavior).
//
// Expected failure: cmdRunnerFn does not exist on Runner yet. After
// implementation, commands should be executed with the bead's working
// directory, not the runner's working directory.
func TestDirectValidation_WorkDirPassedToCommands(t *testing.T) {
	var receivedWorkDir string

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		receivedWorkDir = workDir
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)
	bc := newBeadContext(t)

	expectedWorkDir := bc.PromptCtx.WorkDir

	_ = r.runValidation(context.Background(), bc)

	if receivedWorkDir != expectedWorkDir {
		t.Errorf("expected workDir=%q, got %q", expectedWorkDir, receivedWorkDir)
	}
}

// TestDirectValidation_RecoveryStillWorks verifies that the validation
// recovery mechanism (fix attempts) still works with direct execution.
// When validation fails, the runner should still attempt fixes via Claude
// and then re-validate using direct execution (AC3 - Claude only for
// failure interpretation, not for running commands).
//
// Expected failure: cmdRunnerFn does not exist on Runner yet. After
// implementation, runValidationWithRecovery should work with the new
// direct validation path while still using Claude for build fixes.
func TestDirectValidation_RecoveryStillWorks(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxFixAttempts:       1,
			MaxValidationRetries: 1,
		},
		Preflight: config.PreflightConfig{},
		Claude: config.ClaudeConfig{
			StallTimeout:       30,
			StallTimeoutActive: 10,
			AnalysisTimeout:    30,
		},
	}
	cfg.SetDefaults()

	validationCallCount := 0
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCallCount++
		if validationCallCount == 1 {
			return "", "FAIL: TestSomething", 1, nil
		}
		return "ok", "", 0, nil
	}

	// Create a validation.Runner with an executeFn that simulates Claude fixing the issue
	executeFnCalled := false
	valRunner := validation.NewRunner(cfg, cmdRunner, nil,
		func(ctx context.Context, bc *runtypes.BeadContext) bool {
			executeFnCalled = true
			return true
		},
	)

	var buf strings.Builder
	mockAnalyzer := &mockFailureAnalyzer{}

	r := &Runner{
		cfg:              cfg,
		renderer:         &mockRenderer{},
		analyzer:         mockAnalyzer,
		output:           &buf,
		validationRunner: valRunner,
		cmdRunnerFn:      cmdRunner,
	}

	bc := newBeadContext(t)
	bc.MaxRetries = 1
	bc.MaxRetriesPerBead = 5
	bc.ParentCtx = context.Background()

	err := r.runValidationWithRecovery(context.Background(), bc)

	if err != nil {
		t.Errorf("expected recovery to succeed, got: %v", err)
	}
	if validationCallCount < 2 {
		t.Errorf("expected validation to be called at least twice (initial + after fix), got %d", validationCallCount)
	}
	if !bc.Result.ValidationRetried {
		t.Error("expected ValidationRetried=true after recovery attempt")
	}
	_ = executeFnCalled // Used by the validation runner's recovery
}

// TestDirectValidation_CapturesStdoutAndStderr verifies that both stdout
// and stderr from failed commands are captured in the validation output
// (AC2 - captured stderr on failure).
//
// Expected failure: cmdRunnerFn does not exist on Runner yet. After
// implementation, the validation failure output should include both
// the command that failed and its stderr, providing useful context for
// the analyzer and the user.
func TestDirectValidation_CapturesStdoutAndStderr(t *testing.T) {
	expectedStderr := "src/main.go:15:3: undefined: foo"
	expectedCommand := "go vet ./..."

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./...", "go vet ./..."},
		},
		Preflight: config.PreflightConfig{},
		Claude: config.ClaudeConfig{
			AnalysisTimeout: 30,
		},
	}

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if command == "go test ./..." {
			return "ok", "", 0, nil
		}
		if command == "go vet ./..." {
			return "some stdout", expectedStderr, 1, nil
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, cfg, cmdRunner)
	bc := newBeadContext(t)

	_ = r.runValidation(context.Background(), bc)

	// Result output should contain the stderr from the failed command
	if !strings.Contains(bc.Result.Output, expectedStderr) {
		t.Errorf("expected result output to contain stderr %q, got: %q", expectedStderr, bc.Result.Output)
	}

	// Result output should identify which command failed
	if !strings.Contains(bc.Result.Output, expectedCommand) {
		t.Errorf("expected result output to identify failed command %q, got: %q", expectedCommand, bc.Result.Output)
	}
}
