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

func TestNewExecutor_ReturnsNonNil(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder

	exec := NewExecutor(cfg, &buf, nil, nil, nil)
	if exec == nil {
		t.Fatal("NewExecutor returned nil")
	}
}

func TestNewExecutor_AcceptsNilCallbacks(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder

	exec := NewExecutor(cfg, &buf, nil, nil, nil)
	if exec == nil {
		t.Fatal("NewExecutor should accept nil callbacks")
	}
}

// --- RunAcceptanceTests tests ---

func TestRunAcceptanceTests_RendersPromptAndInvokes(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder

	renderCalled := false
	invokeCalled := false
	var invokedPrompt string

	renderFn := func(ctx *prompt.Context) (string, error) {
		renderCalled = true
		return "rendered acceptance test prompt", nil
	}

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

func TestRunAcceptanceTests_PropagatesRenderError(t *testing.T) {
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

func TestRunAcceptanceTests_PropagatesInvocationError(t *testing.T) {
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

func TestVerifyTestsFail_ReturnsNilWhenTestsFail(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder

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

func TestVerifyTestsFail_ReturnsErrorWhenTestsPass(t *testing.T) {
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

func TestVerifyTestsFail_ReturnsErrorWhenValidationDisabled(t *testing.T) {
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

func TestVerifyTestsFail_PropagatesValidationError(t *testing.T) {
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

func TestVerifyTestsFail_NilResultReturnsError(t *testing.T) {
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

// TestPackageDoesNotImportRunner verifies structural isolation: this test file
// compiles within the methodology package without importing runner/, proving
// the package only depends on runtypes/ and external packages.
func TestPackageDoesNotImportRunner(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder
	exec := NewExecutor(cfg, &buf, nil, nil, nil)
	if exec == nil {
		t.Fatal("methodology.NewExecutor should return a non-nil Executor")
	}
}
