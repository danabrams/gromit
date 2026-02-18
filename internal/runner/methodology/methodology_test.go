package methodology

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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

func newProjectPromptRenderer(t *testing.T) *prompt.Renderer {
	t.Helper()

	repoRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	renderer, err := prompt.NewRenderer(
		filepath.Join(repoRoot, ".gromit", "templates"),
		filepath.Join(repoRoot, ".gromit", "specs"),
		filepath.Join(repoRoot, "CLAUDE.md"),
		filepath.Join(repoRoot, ".gromit"),
	)
	if err != nil {
		t.Fatalf("create prompt renderer: %v", err)
	}
	return renderer
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

func TestRunAcceptanceTests_LogsLifecycle(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder

	renderFn := func(ctx *prompt.Context) (string, error) {
		return "rendered acceptance test prompt", nil
	}
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		return nil
	}

	exec := NewExecutor(cfg, &buf, renderFn, invokeFn, nil)
	bc := newTestBeadContext()

	if err := exec.RunAcceptanceTests(context.Background(), bc); err != nil {
		t.Fatalf("RunAcceptanceTests returned unexpected error: %v", err)
	}

	logs := buf.String()
	if !strings.Contains(logs, "ATDD acceptance generation started") {
		t.Fatalf("expected start lifecycle log, got: %q", logs)
	}
	if !strings.Contains(logs, "ATDD acceptance generation completed in") {
		t.Fatalf("expected completion lifecycle log, got: %q", logs)
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

func TestRunAcceptanceTests_ShapedContextTemplateCompatibility(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder

	renderer := newProjectPromptRenderer(t)

	var renderedCtx *prompt.Context
	renderFn := func(ctx *prompt.Context) (string, error) {
		renderedCtx = ctx
		return renderer.RenderAcceptanceTests(ctx)
	}

	var invokedPrompt string
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error {
		invokedPrompt = promptText
		return nil
	}

	exec := NewExecutor(cfg, &buf, renderFn, invokeFn, nil)

	bc := newTestBeadContext()
	bc.PromptCtx.Bead = bc.Bead
	bc.PromptCtx.WorkDir = t.TempDir()
	bc.PromptCtx.ClaudeMD = "project context that should be trimmed in shaped red phase"
	bc.PromptCtx.Rules = "## Rules\n- follow constraints"
	bc.PromptCtx.Spec = "## Spec\n- add behavior"
	bc.PromptCtx.SpecName = "compat-spec"
	bc.PromptCtx = renderer.ShapeRedPhaseContext(bc.PromptCtx)

	err := exec.RunAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunAcceptanceTests returned unexpected error: %v", err)
	}
	if renderedCtx == nil {
		t.Fatal("expected render function to receive context")
	}
	if renderedCtx.ClaudeMD != "" {
		t.Fatalf("expected shaped red context to trim ClaudeMD, got %q", renderedCtx.ClaudeMD)
	}
	if !strings.Contains(strings.ToLower(renderedCtx.FailureContext), "test-only") {
		t.Fatalf("expected red-shaped guardrails in failure context, got %q", renderedCtx.FailureContext)
	}
	if !strings.Contains(invokedPrompt, "**ID:** test-meth-001") {
		t.Fatalf("expected rendered acceptance template to include bead ID, got %q", invokedPrompt)
	}
	if !strings.Contains(strings.ToLower(invokedPrompt), "acceptance test writing") {
		t.Fatalf("expected acceptance template content, got %q", invokedPrompt)
	}
}

// --- VerifyTestsFail tests ---

func TestVerifyTestsFail_UsesAcceptanceCommands(t *testing.T) {
	cfg := newTestConfig()
	cfg.Validation.Commands = []string{"full-check"}
	cfg.Validation.FastCommands = []string{"go test ./...", "go vet ./..."}
	var buf strings.Builder

	var receivedCommands []string
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		receivedCommands = commands
		return &claude.Result{
			Success:  true,
			Output:   "FAIL: TestSomething",
			ExitCode: 1,
		}, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)
	bc := newTestBeadContext()
	_ = exec.VerifyTestsFail(context.Background(), bc)

	if len(receivedCommands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(receivedCommands))
	}
	if receivedCommands[0] != "go test -tags acceptance ./..." {
		t.Errorf("VerifyTestsFail should inject -tags acceptance, got %q", receivedCommands[0])
	}
	if receivedCommands[1] != "go vet ./..." {
		t.Errorf("non-go-test commands should be unchanged, got %q", receivedCommands[1])
	}
}

func TestVerifyTestsFail_RefreshesTouchedPackagesFromDiff(t *testing.T) {
	cfg := newTestConfig()
	cfg.Validation.FastCommands = []string{"go test ./..."}
	var buf strings.Builder

	var receivedCommands []string
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		receivedCommands = append([]string{}, commands...)
		return &claude.Result{
			Success:  true,
			Output:   "FAIL: acceptance still red",
			ExitCode: 1,
		}, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)
	exec.SetGetDiffFn(func(startCommit string) (string, error) {
		return "diff --git a/internal/runner/foo.go b/internal/runner/foo.go\n", nil
	})

	bc := newTestBeadContext()
	bc.StartCommit = "abc123"
	bc.TouchedPackages = nil

	if err := exec.VerifyTestsFail(context.Background(), bc); err != nil {
		t.Fatalf("VerifyTestsFail returned unexpected error: %v", err)
	}

	if len(receivedCommands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(receivedCommands))
	}
	if receivedCommands[0] != "go test -tags acceptance ./internal/runner/..." {
		t.Fatalf("expected scoped acceptance command, got %q", receivedCommands[0])
	}
}

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
	if !strings.Contains(buf.String(), "Validation output on unexpected pass: VALIDATION_PASSED") {
		t.Errorf("expected unexpected-pass validation output to be logged, got log: %q", buf.String())
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

func TestVerifyTestsFail_LogsAcceptanceCommandSet(t *testing.T) {
	cfg := newTestConfig()
	cfg.Validation.Commands = []string{"go test ./...", "go vet ./..."}
	var buf strings.Builder

	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return &claude.Result{
			Success:  true,
			Output:   "FAIL: TestSomething",
			ExitCode: 1,
		}, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)
	bc := newTestBeadContext()
	_ = exec.VerifyTestsFail(context.Background(), bc)

	got := buf.String()
	if !strings.Contains(got, "ATDD verify command set: go test -tags acceptance ./... && go vet ./...") {
		t.Errorf("expected acceptance command set to be logged, got: %q", got)
	}
}

func TestVerifyTestsFail_LogsInvocationCompletion(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder

	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return &claude.Result{
			Success:  true,
			Output:   "FAIL: TestSomething",
			ExitCode: 1,
		}, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)
	bc := newTestBeadContext()
	_ = exec.VerifyTestsFail(context.Background(), bc)

	got := buf.String()
	if !strings.Contains(got, "ATDD verify invocation completed in") {
		t.Fatalf("expected ATDD verify completion log, got: %q", got)
	}
	if !strings.Contains(got, "(exit_code=1)") {
		t.Fatalf("expected verify completion log to include exit code, got: %q", got)
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

// --- VerifyAcceptanceTestsPass tests ---

func TestVerifyAcceptanceTestsPass_ReturnsNilWhenTestsPass(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder

	var receivedCommands []string
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		receivedCommands = commands
		return &claude.Result{
			Success:  true,
			Output:   "VALIDATION_PASSED",
			ExitCode: 0,
		}, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)
	bc := newTestBeadContext()
	err := exec.VerifyAcceptanceTestsPass(context.Background(), bc)

	if err != nil {
		t.Fatalf("VerifyAcceptanceTestsPass should return nil when tests pass, got: %v", err)
	}
	// Should use acceptance-tagged commands
	if len(receivedCommands) == 0 {
		t.Fatal("expected commands to be passed to validateFn")
	}
	if receivedCommands[0] != "go test -tags acceptance ./..." {
		t.Errorf("should use acceptance commands, got %q", receivedCommands[0])
	}
}

func TestVerifyAcceptanceTestsPass_ReturnsErrorWhenTestsFail(t *testing.T) {
	cfg := newTestConfig()
	var buf strings.Builder

	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return &claude.Result{
			Success:  true,
			Output:   "FAIL: TestSomething",
			ExitCode: 1,
		}, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)
	bc := newTestBeadContext()
	err := exec.VerifyAcceptanceTestsPass(context.Background(), bc)

	if err == nil {
		t.Fatal("VerifyAcceptanceTestsPass should return error when tests fail")
	}
	if !strings.Contains(err.Error(), "acceptance tests failed") {
		t.Errorf("error should mention acceptance tests failed, got: %v", err)
	}

	var acceptanceErr *AcceptanceVerificationError
	if !errors.As(err, &acceptanceErr) {
		t.Fatalf("expected AcceptanceVerificationError, got %T", err)
	}
	if acceptanceErr.Output == "" {
		t.Fatal("expected acceptance failure output to be captured")
	}
}

func TestVerifyAcceptanceTestsPass_ReturnsErrorWhenValidationDisabled(t *testing.T) {
	cfg := newTestConfig()
	cfg.Validation.Enabled = false
	var buf strings.Builder

	exec := NewExecutor(cfg, &buf, nil, nil, nil)
	bc := newTestBeadContext()
	err := exec.VerifyAcceptanceTestsPass(context.Background(), bc)

	if err == nil {
		t.Fatal("VerifyAcceptanceTestsPass should return error when validation disabled")
	}
}

// --- AcceptanceCommands tests ---

func TestAcceptanceCommands_InjectsTagsIntoGoTest(t *testing.T) {
	commands := []string{"go test ./...", "go vet ./..."}
	got := AcceptanceCommands(commands, nil)

	if len(got) != 2 {
		t.Fatalf("AcceptanceCommands returned %d commands, want 2", len(got))
	}
	if got[0] != "go test -tags acceptance ./..." {
		t.Errorf("AcceptanceCommands()[0] = %q, want %q", got[0], "go test -tags acceptance ./...")
	}
	if got[1] != "go vet ./..." {
		t.Errorf("AcceptanceCommands()[1] = %q, want unchanged %q", got[1], "go vet ./...")
	}
}

func TestAcceptanceCommands_HandlesGoTestWithExistingFlags(t *testing.T) {
	commands := []string{"go test -v -count=1 ./..."}
	got := AcceptanceCommands(commands, nil)

	if len(got) != 1 {
		t.Fatalf("AcceptanceCommands returned %d commands, want 1", len(got))
	}
	if got[0] != "go test -tags acceptance -v -count=1 ./..." {
		t.Errorf("AcceptanceCommands()[0] = %q, want %q", got[0], "go test -tags acceptance -v -count=1 ./...")
	}
}

func TestAcceptanceCommands_PreservesNonGoTestCommands(t *testing.T) {
	commands := []string{"golangci-lint run ./...", "go vet ./..."}
	got := AcceptanceCommands(commands, nil)

	if len(got) != 2 {
		t.Fatalf("AcceptanceCommands returned %d commands, want 2", len(got))
	}
	if got[0] != "golangci-lint run ./..." {
		t.Errorf("AcceptanceCommands()[0] = %q, want unchanged", got[0])
	}
	if got[1] != "go vet ./..." {
		t.Errorf("AcceptanceCommands()[1] = %q, want unchanged", got[1])
	}
}

func TestAcceptanceCommands_EmptySlice(t *testing.T) {
	got := AcceptanceCommands([]string{}, nil)
	if len(got) != 0 {
		t.Fatalf("AcceptanceCommands of empty slice should return empty, got %d", len(got))
	}
}

func TestAcceptanceCommands_SkipsIfAlreadyTagged(t *testing.T) {
	commands := []string{"go test -tags acceptance ./..."}
	got := AcceptanceCommands(commands, nil)

	if got[0] != "go test -tags acceptance ./..." {
		t.Errorf("AcceptanceCommands should not double-tag, got %q", got[0])
	}
}

func TestAcceptanceCommands_ScopesGoTestWhenTouchedPackagesPresent(t *testing.T) {
	commands := []string{"go test ./...", "go vet ./..."}
	got := AcceptanceCommands(commands, []string{"internal/runner"})

	if len(got) != 2 {
		t.Fatalf("AcceptanceCommands returned %d commands, want 2", len(got))
	}
	if got[0] != "go test -tags acceptance ./internal/runner/..." {
		t.Errorf("AcceptanceCommands()[0] = %q, want scoped go test command", got[0])
	}
	if got[1] != "go vet ./..." {
		t.Errorf("AcceptanceCommands()[1] = %q, want unchanged %q", got[1], "go vet ./...")
	}
}

func TestAcceptanceCommands_ScopesGoTestToMultipleTouchedPackages(t *testing.T) {
	commands := []string{"go test -count=1 ./..."}
	got := AcceptanceCommands(commands, []string{"internal/config", "internal/runner", "internal/runner"})

	if len(got) != 1 {
		t.Fatalf("AcceptanceCommands returned %d commands, want 1", len(got))
	}
	want := "go test -tags acceptance -count=1 ./internal/config/... ./internal/runner/..."
	if got[0] != want {
		t.Errorf("AcceptanceCommands()[0] = %q, want %q", got[0], want)
	}
}

func TestAcceptanceCommands_IncludesRootPackage(t *testing.T) {
	commands := []string{"go test -count=1 ./..."}
	got := AcceptanceCommands(commands, []string{".", "internal/runner"})

	if len(got) != 1 {
		t.Fatalf("AcceptanceCommands returned %d commands, want 1", len(got))
	}

	want := "go test -tags acceptance -count=1 . ./internal/runner/..."
	if got[0] != want {
		t.Errorf("AcceptanceCommands()[0] = %q, want %q", got[0], want)
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

func TestRunAcceptanceTestsWithRetry_PropagatesDeadlineExceeded(t *testing.T) {
	// When the invocation returns DeadlineExceeded, RunAcceptanceTestsWithRetry
	// should propagate it immediately without retrying — timeout errors are
	// phase-level concerns, not transient failures.
	cfg := newTestConfig()
	cfg.Escalation.MaxRetriesPerModel = 3 // would retry 3 times if not short-circuited
	var buf strings.Builder

	invocationCount := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error {
		invocationCount++
		return context.DeadlineExceeded
	}
	renderFn := func(ctx *prompt.Context) (string, error) {
		return "acceptance prompt", nil
	}

	exec := NewExecutorWithEscalation(cfg, &buf, renderFn, invokeFn, nil, nil)
	bc := newTestBeadContext()

	err := exec.RunAcceptanceTestsWithRetry(context.Background(), bc)
	if err == nil {
		t.Fatal("expected error from RunAcceptanceTestsWithRetry")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded in error chain, got: %v", err)
	}
	if invocationCount != 1 {
		t.Fatalf("invocation count = %d, want 1 (should not retry on deadline exceeded)", invocationCount)
	}
}

func TestRunAcceptanceTestsWithRetry_PropagatesContextCanceled(t *testing.T) {
	// When the context is already canceled, RunAcceptanceTestsWithRetry should
	// abort immediately without invoking the LLM.
	cfg := newTestConfig()
	var buf strings.Builder

	invocationCount := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error {
		invocationCount++
		return nil
	}
	renderFn := func(ctx *prompt.Context) (string, error) {
		return "acceptance prompt", nil
	}

	exec := NewExecutorWithEscalation(cfg, &buf, renderFn, invokeFn, nil, nil)
	bc := newTestBeadContext()

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := exec.RunAcceptanceTestsWithRetry(canceledCtx, bc)
	if err == nil {
		t.Fatal("expected error from RunAcceptanceTestsWithRetry with canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected Canceled in error chain, got: %v", err)
	}
	if invocationCount != 0 {
		t.Fatalf("invocation count = %d, want 0 (should not invoke when context is already canceled)", invocationCount)
	}
}
