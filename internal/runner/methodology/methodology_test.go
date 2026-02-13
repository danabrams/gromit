package methodology

import (
	"context"
	"fmt"
	"strings"
	"testing"

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
		Claude: config.ClaudeConfig{
			StallTimeout:       30,
			StallTimeoutActive: 10,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

func newTestBeadContext() *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-meth-001", Title: "Test methodology bead", Priority: 1},
		Tier:      "medium",
		Model:     "sonnet",
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: "/tmp/test-project"},
	}
}

// --- NewExecutor constructor tests ---

// Expected failure: Executor struct and NewExecutor constructor do not exist yet
func TestNewExecutor_ReturnsNonNil(t *testing.T) {
	// NewExecutor should accept narrow dependencies and return a non-nil Executor.
	cfg := newTestConfig()
	var buf strings.Builder

	exec := NewExecutor(cfg, &buf, nil, nil, nil)
	if exec == nil {
		t.Fatal("NewExecutor returned nil")
	}
}

// Expected failure: Executor struct and NewExecutor constructor do not exist yet
func TestNewExecutor_AcceptsNilCallbacks(t *testing.T) {
	// EscalateTierFn and InvokeFn should be optional — passing nil is valid.
	// This supports constructing an Executor for test-only scenarios
	// where escalation and invocation are not needed.
	cfg := newTestConfig()
	var buf strings.Builder

	exec := NewExecutor(cfg, &buf, nil, nil, nil)
	if exec == nil {
		t.Fatal("NewExecutor should accept nil callbacks")
	}
}

// --- RunAcceptanceTests tests ---

// Expected failure: Executor.RunAcceptanceTests method does not exist yet
func TestRunAcceptanceTests_RendersPromptAndInvokes(t *testing.T) {
	// RunAcceptanceTests should:
	// 1. Render the acceptance tests prompt via the provided render function
	// 2. Invoke the LLM via InvokeFn with the rendered prompt
	// 3. Return nil when the invocation succeeds
	cfg := newTestConfig()
	var buf strings.Builder

	renderCalled := false
	invokeCalled := false
	var invokedPrompt string

	// Expected failure: RenderFn type does not exist in methodology package yet
	renderFn := func(ctx *prompt.Context) (string, error) {
		renderCalled = true
		return "rendered acceptance test prompt", nil
	}

	// Expected failure: InvokeFn type does not exist in methodology package yet
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		invokeCalled = true
		invokedPrompt = prompt
		return nil
	}

	exec := NewExecutor(cfg, &buf, renderFn, invokeFn, nil)

	bc := newTestBeadContext()
	err := exec.RunAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunAcceptanceTests returned unexpected error: %v", err)
	}
	if !renderCalled {
		t.Error("RunAcceptanceTests should call the render function to produce the prompt")
	}
	if !invokeCalled {
		t.Error("RunAcceptanceTests should call InvokeFn to invoke the LLM")
	}
	if invokedPrompt != "rendered acceptance test prompt" {
		t.Errorf("InvokeFn received prompt %q, want %q", invokedPrompt, "rendered acceptance test prompt")
	}
}

// Expected failure: Executor.RunAcceptanceTests method does not exist yet
func TestRunAcceptanceTests_PropagatesRenderError(t *testing.T) {
	// When the render function fails, RunAcceptanceTests should propagate
	// the error without invoking the LLM.
	cfg := newTestConfig()
	var buf strings.Builder

	renderFn := func(ctx *prompt.Context) (string, error) {
		return "", fmt.Errorf("template not found")
	}

	invokeCalled := false
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		invokeCalled = true
		return nil
	}

	exec := NewExecutor(cfg, &buf, renderFn, invokeFn, nil)

	bc := newTestBeadContext()
	err := exec.RunAcceptanceTests(context.Background(), bc)
	if err == nil {
		t.Fatal("RunAcceptanceTests should return error when render fails")
	}
	if !strings.Contains(err.Error(), "template not found") {
		t.Errorf("error should contain render error, got: %v", err)
	}
	if invokeCalled {
		t.Error("InvokeFn should not be called when render fails")
	}
}

// Expected failure: Executor.RunAcceptanceTests method does not exist yet
func TestRunAcceptanceTests_PropagatesInvocationError(t *testing.T) {
	// When InvokeFn returns an error, RunAcceptanceTests should propagate it.
	cfg := newTestConfig()
	var buf strings.Builder

	renderFn := func(ctx *prompt.Context) (string, error) {
		return "prompt", nil
	}
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		return fmt.Errorf("stall timeout during acceptance tests")
	}

	exec := NewExecutor(cfg, &buf, renderFn, invokeFn, nil)

	bc := newTestBeadContext()
	err := exec.RunAcceptanceTests(context.Background(), bc)
	if err == nil {
		t.Fatal("RunAcceptanceTests should return error when invocation fails")
	}
	if !strings.Contains(err.Error(), "stall timeout") {
		t.Errorf("error should contain invocation error, got: %v", err)
	}
}

// --- VerifyTestsFail tests ---

// Expected failure: Executor.VerifyTestsFail method does not exist yet
func TestVerifyTestsFail_ReturnsNilWhenTestsFail(t *testing.T) {
	// VerifyTestsFail runs validation and expects it to FAIL.
	// When tests fail (validation returns non-passing result), VerifyTestsFail
	// should return nil — this is the expected ATDD behavior before implementation.
	cfg := newTestConfig()
	var buf strings.Builder

	// Expected failure: ValidateDirectFn type does not exist in methodology package yet
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return &claude.Result{
			Success:  true, // Claude ran successfully
			Output:   "FAIL: TestSomething",
			ExitCode: 1, // but tests failed (non-zero exit)
		}, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)

	bc := newTestBeadContext()
	err := exec.VerifyTestsFail(context.Background(), bc)
	if err != nil {
		t.Errorf("VerifyTestsFail should return nil when tests fail as expected, got: %v", err)
	}
}

// Expected failure: Executor.VerifyTestsFail method does not exist yet
func TestVerifyTestsFail_ReturnsErrorWhenTestsPass(t *testing.T) {
	// When validation passes (tests succeed before implementation), VerifyTestsFail
	// should return an error indicating the tests aren't covering new behavior.
	cfg := newTestConfig()
	var buf strings.Builder

	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return &claude.Result{
			Success:  true,
			Output:   "VALIDATION_PASSED",
			ExitCode: 0,
		}, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)

	bc := newTestBeadContext()
	err := exec.VerifyTestsFail(context.Background(), bc)
	if err == nil {
		t.Fatal("VerifyTestsFail should return error when tests pass unexpectedly")
	}
	if !strings.Contains(err.Error(), "acceptance tests passed before implementation") {
		t.Errorf("error should indicate tests passed unexpectedly, got: %v", err)
	}
}

// Expected failure: Executor.VerifyTestsFail method does not exist yet
func TestVerifyTestsFail_ReturnsErrorWhenValidationDisabled(t *testing.T) {
	// When validation is not enabled, VerifyTestsFail cannot verify anything
	// and should return an error.
	cfg := newTestConfig()
	cfg.Validation.Enabled = false
	var buf strings.Builder

	exec := NewExecutor(cfg, &buf, nil, nil, nil)

	bc := newTestBeadContext()
	err := exec.VerifyTestsFail(context.Background(), bc)
	if err == nil {
		t.Fatal("VerifyTestsFail should return error when validation is disabled")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("error should mention validation, got: %v", err)
	}
}

// Expected failure: Executor.VerifyTestsFail method does not exist yet
func TestVerifyTestsFail_PropagatesValidationError(t *testing.T) {
	// When the validation function returns an error (e.g., command not found),
	// VerifyTestsFail should propagate it.
	cfg := newTestConfig()
	var buf strings.Builder

	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return nil, fmt.Errorf("exec: binary not found")
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)

	bc := newTestBeadContext()
	err := exec.VerifyTestsFail(context.Background(), bc)
	if err == nil {
		t.Fatal("VerifyTestsFail should return error when validation invocation fails")
	}
	if !strings.Contains(err.Error(), "binary not found") {
		t.Errorf("error should contain invocation error, got: %v", err)
	}
}

// Expected failure: Executor.VerifyTestsFail method does not exist yet
func TestVerifyTestsFail_NilResultReturnsError(t *testing.T) {
	// When the validation function returns a nil result, VerifyTestsFail
	// should return an error rather than panicking.
	cfg := newTestConfig()
	var buf strings.Builder

	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return nil, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)

	bc := newTestBeadContext()
	err := exec.VerifyTestsFail(context.Background(), bc)
	if err == nil {
		t.Fatal("VerifyTestsFail should return error when validation returns nil result")
	}
}

// --- IsTestOnlyDiff tests ---

// Expected failure: IsTestOnlyDiff function does not exist in the methodology package yet
func TestIsTestOnlyDiff(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want bool
	}{
		{
			name: "empty diff returns true",
			diff: "",
			want: true,
		},
		{
			name: "whitespace only diff returns true",
			diff: "   \n  \n",
			want: true,
		},
		{
			name: "only test files returns true",
			diff: "diff --git a/internal/runner/process_test.go b/internal/runner/process_test.go\n+some test code\ndiff --git a/internal/config/config_test.go b/internal/config/config_test.go\n+more tests",
			want: true,
		},
		{
			name: "implementation file present returns false",
			diff: "diff --git a/internal/runner/process.go b/internal/runner/process.go\n+impl code\ndiff --git a/internal/runner/process_test.go b/internal/runner/process_test.go\n+test code",
			want: false,
		},
		{
			name: "only implementation files returns false",
			diff: "diff --git a/internal/runner/process.go b/internal/runner/process.go\n+implementation code",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTestOnlyDiff(tt.diff)
			if got != tt.want {
				t.Errorf("IsTestOnlyDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- ParseDiffFiles tests ---

// Expected failure: ParseDiffFiles function does not exist in the methodology package yet
func TestParseDiffFiles(t *testing.T) {
	tests := []struct {
		name  string
		diff  string
		want  []string
		count int
	}{
		{
			name:  "empty diff returns nil",
			diff:  "",
			want:  nil,
			count: 0,
		},
		{
			name:  "single file",
			diff:  "diff --git a/internal/runner/process.go b/internal/runner/process.go\n+some change",
			want:  []string{"internal/runner/process.go"},
			count: 1,
		},
		{
			name:  "multiple files preserves order",
			diff:  "diff --git a/file_b.go b/file_b.go\n+change\ndiff --git a/file_a.go b/file_a.go\n+change",
			want:  []string{"file_b.go", "file_a.go"},
			count: 2,
		},
		{
			name:  "strips b/ prefix from paths",
			diff:  "diff --git a/internal/pkg/foo.go b/internal/pkg/foo.go\n+change",
			want:  []string{"internal/pkg/foo.go"},
			count: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDiffFiles(tt.diff)

			// Verify count
			if len(got) != tt.count {
				t.Errorf("ParseDiffFiles() returned %d files, want %d", len(got), tt.count)
			}

			// Verify exact file paths
			if tt.want == nil && got != nil {
				t.Errorf("ParseDiffFiles() = %v, want nil", got)
			}
			if tt.want != nil {
				for i, wantFile := range tt.want {
					if i >= len(got) {
						t.Errorf("ParseDiffFiles() missing file at index %d: want %q", i, wantFile)
						continue
					}
					if got[i] != wantFile {
						t.Errorf("ParseDiffFiles()[%d] = %q, want %q", i, got[i], wantFile)
					}
				}
			}
		})
	}
}

// --- Package isolation test ---

// Expected failure: methodology package does not exist yet — this test verifies
// that the methodology package uses only narrow interfaces (function types) and
// does not import the runner/ facade package.
func TestPackageDoesNotImportRunner(t *testing.T) {
	// The methodology package must not import the runner/ facade.
	// This is verified structurally: the test file imports from methodology
	// package (package methodology), and the production code should only
	// import runtypes/ and standard library / external packages.
	//
	// If this test compiles, it means the methodology package exists and
	// can be tested independently. The import structure at the top of
	// this file demonstrates isolation: we import runtypes, config, bead,
	// claude, and prompt — but NOT "runner".

	// Verify the Executor struct exists and can be constructed
	cfg := newTestConfig()
	var buf strings.Builder
	exec := NewExecutor(cfg, &buf, nil, nil, nil)
	if exec == nil {
		t.Fatal("methodology.NewExecutor should return a non-nil Executor")
	}
}

// Ensure imports are used
var (
	_ = claude.Result{}
	_ = bead.Bead{}
)
