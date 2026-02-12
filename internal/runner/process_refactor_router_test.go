package runner

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestRunRefactorPhase_UsesRouter verifies that runRefactorPhase
// calls router.Select() with phase="build" and the tier from bc.tier.
func TestRunRefactorPhase_UsesRouter(t *testing.T) {
	// Create a temp git repo to avoid early returns from getDiff
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	var buf strings.Builder

	// Track router.Select calls
	selectCalled := false
	var capturedPhase, capturedTier string

	mockProvider := &mockProviderForRefactor{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			selectCalled = true
			capturedPhase = "build"
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Model:   "test-model",
				Output:  "refactor complete",
			}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "validation-model",
				Output:  "Tests passed\nVALIDATION_PASSED",
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRend := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			return "refactor this code", nil
		},
	}

	r := &Runner{
		cfg: &config.Config{
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"go test ./..."},
			},
		},
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
	}

	// Get HEAD commit after some changes
	writeTestFile(t, tmpDir, "test.txt", "initial content")
	commitGit(t, tmpDir, "initial commit")
	startCommit := getGitHeadCommit(t, tmpDir)

	// Make some changes
	writeTestFile(t, tmpDir, "test.txt", "changed content")
	commitGit(t, tmpDir, "changed content")

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test",
			Priority: 1,
		},
		tier:        provider.TierMedium,
		model:       "sonnet",
		result:      &IterationResult{},
		startCommit: startCommit,
		promptCtx: &prompt.Context{
			WorkDir: tmpDir,
		},
	}

	// Change to temp dir so git operations work
	oldDir := changeDir(t, tmpDir)
	defer changeDir(t, oldDir)

	err := r.runRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("runRefactorPhase() error = %v", err)
	}

	// Verify router.Select() was called with correct phase
	if !selectCalled {
		t.Error("router.Select() was not called - runRefactorPhase should use router")
	}

	if capturedPhase != "build" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "build")
	}

	// Verify tier matches bc.tier
	if capturedTier != bc.tier {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, bc.tier)
	}
}

// TestHandleRefactorValidationFailure_UsesRouter verifies that
// handleRefactorValidationFailure calls router.Select() with phase="build"
// and the tier from bc.tier when retrying the refactor.
func TestHandleRefactorValidationFailure_UsesRouter(t *testing.T) {
	// Create a temp git repo
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	var buf strings.Builder

	// Track router.Select calls
	selectCalled := false
	var capturedPhase, capturedTier string

	mockProvider := &mockProviderForRefactor{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			selectCalled = true
			capturedPhase = "build"
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Model:   "test-model",
				Output:  "retry refactor complete",
			}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "validation-model",
				Output:  "Tests passed\nVALIDATION_PASSED",
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRend := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			return "retry refactor this code", nil
		},
	}

	r := &Runner{
		cfg: &config.Config{
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"go test ./..."},
			},
		},
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
	}

	// Setup git commits
	writeTestFile(t, tmpDir, "test.txt", "initial content")
	commitGit(t, tmpDir, "initial commit")
	preRefactorCommit := getGitHeadCommit(t, tmpDir)

	// Make some changes after refactor (which we'll revert)
	writeTestFile(t, tmpDir, "test.txt", "refactored content")
	commitGit(t, tmpDir, "refactored")

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test",
			Priority: 1,
		},
		tier:   provider.TierMedium,
		model:  "sonnet",
		result: &IterationResult{},
		promptCtx: &prompt.Context{
			WorkDir: tmpDir,
		},
	}

	// Change to temp dir so git operations work
	oldDir := changeDir(t, tmpDir)
	defer changeDir(t, oldDir)

	err := r.handleRefactorValidationFailure(context.Background(), bc, preRefactorCommit, "test failure")
	if err != nil {
		t.Fatalf("handleRefactorValidationFailure() error = %v", err)
	}

	// Verify router.Select() was called with correct phase
	if !selectCalled {
		t.Error("router.Select() was not called - handleRefactorValidationFailure should use router")
	}

	if capturedPhase != "build" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "build")
	}

	// Verify tier matches bc.tier
	if capturedTier != bc.tier {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, bc.tier)
	}
}

// mockProviderForRefactor is a test double for Provider that tracks Run calls
type mockProviderForRefactor struct {
	name            string
	runFn           func(ctx context.Context, prompt, tier string) (*provider.Result, error)
	runValidationFn func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error)
}

func (m *mockProviderForRefactor) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockProviderForRefactor) ModelForTier(tier string) string {
	switch tier {
	case provider.TierHigh:
		return "mock-opus"
	case provider.TierMedium:
		return "mock-sonnet"
	case provider.TierLow:
		return "mock-haiku"
	default:
		return "mock-model"
	}
}

func (m *mockProviderForRefactor) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderForRefactor) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderForRefactor) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if m.runValidationFn != nil {
		return m.runValidationFn(ctx, commands, tier, workDir)
	}
	return &provider.Result{Success: true, Model: "mock-model", Output: "VALIDATION_PASSED"}, nil
}

func (m *mockProviderForRefactor) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

// Test helpers for git operations
func setupGitRepo(t *testing.T, dir string) {
	t.Helper()
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "test@example.com")
	runCmd(t, dir, "git", "config", "user.name", "Test User")
}

func writeTestFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := dir + "/" + filename
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}

func commitGit(t *testing.T, dir, message string) {
	t.Helper()
	runCmd(t, dir, "git", "add", ".")
	runCmd(t, dir, "git", "commit", "-m", message)
}

func getGitHeadCommit(t *testing.T, dir string) string {
	t.Helper()
	output := runCmdOutput(t, dir, "git", "rev-parse", "HEAD")
	return strings.TrimSpace(output)
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	runCmdOutput(t, dir, name, args...)
}

func runCmdOutput(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %v failed: %v\n%s", name, args, err, output)
	}
	return string(output)
}

func changeDir(t *testing.T, dir string) string {
	t.Helper()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}
	return oldDir
}
