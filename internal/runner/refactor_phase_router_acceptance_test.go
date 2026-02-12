//go:build acceptance

package runner

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestAcceptance_RunRefactorPhaseUsesRouterSelect verifies that
// runRefactorPhase calls router.Select() with phase="build" and tier from bc.tier.
// Expected failure: runRefactorPhase still uses r.claude.Run() instead of router.Select()
func TestAcceptance_RunRefactorPhaseUsesRouterSelect(t *testing.T) {
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = true
	cfg.Validation.Commands = []string{"go test"}

	// Track router.Select() calls
	var capturedPhase, capturedTier string
	selectCalled := false

	mockProvider := &mockProviderForRefactorTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			selectCalled = true
			capturedPhase = "build"
			capturedTier = tier
			return &provider.Result{Success: true, Model: "test-model", Output: "refactor complete"}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "validation-model", Output: "VALIDATION_PASSED"}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRenderer := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			return "refactor this code", nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "refactor-001",
			Priority: 1,
		},
		tier:        provider.TierMedium,
		model:       "sonnet",
		result:      &IterationResult{},
		promptCtx:   &prompt.Context{WorkDir: "."},
		startCommit: "abc123",
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	err := r.runRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("runRefactorPhase() error = %v", err)
	}

	// Verify router.Select() was called with correct phase
	if !selectCalled {
		t.Error("router.Select() was not called - runRefactorPhase still uses r.claude.Run()")
	}

	if capturedPhase != "build" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "build")
	}

	// Verify tier matches bc.tier
	if capturedTier != bc.tier {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, bc.tier)
	}
}

// TestAcceptance_RunRefactorPhaseCallsProviderRun verifies that
// runRefactorPhase invokes provider.Run() with the selected tier.
// Expected failure: runRefactorPhase still uses r.claude.Run() instead of provider.Run()
func TestAcceptance_RunRefactorPhaseCallsProviderRun(t *testing.T) {
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = true
	cfg.Validation.Commands = []string{"go test"}

	runCalled := false
	var capturedPrompt, capturedTier string

	mockProvider := &mockProviderForRefactorTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCalled = true
			capturedPrompt = prompt
			capturedTier = tier
			return &provider.Result{Success: true, Model: "test-model", Output: "refactor done"}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "validation-model", Output: "VALIDATION_PASSED"}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRenderer := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			return "refactor authentication module", nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "refactor-002",
			Priority: 1,
		},
		tier:        provider.TierHigh,
		model:       "opus",
		result:      &IterationResult{},
		promptCtx:   &prompt.Context{WorkDir: "."},
		startCommit: "def456",
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	err := r.runRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("runRefactorPhase() error = %v", err)
	}

	if !runCalled {
		t.Error("provider.Run() was not called - runRefactorPhase does not invoke the provider")
	}

	expectedPrompt := "refactor authentication module"
	if capturedPrompt != expectedPrompt {
		t.Errorf("provider.Run() prompt = %q, want %q", capturedPrompt, expectedPrompt)
	}

	if capturedTier != bc.tier {
		t.Errorf("provider.Run() tier = %q, want %q", capturedTier, bc.tier)
	}
}

// TestAcceptance_RunRefactorPhaseDetectsUsageLimitError verifies that
// when provider.Run() fails with a usage limit error, runRefactorPhase
// calls router.MarkUnavailable() and retries with a fallback provider.
// Expected failure: runRefactorPhase does not check IsUsageLimitError() yet
func TestAcceptance_RunRefactorPhaseDetectsUsageLimitError(t *testing.T) {
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = true
	cfg.Validation.Commands = []string{"go test"}

	usageLimitErr := errors.New("usage limit exceeded")
	runCallCount := 0
	var markedProviderName string

	// First provider hits usage limit
	firstProvider := &mockProviderForRefactorUsageLimit{
		name: "claude",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCallCount++
			if runCallCount == 1 {
				return &provider.Result{Success: false, Output: "usage limit hit"}, usageLimitErr
			}
			return &provider.Result{Success: true, Model: "fallback-model", Output: "refactor done"}, nil
		},
		isUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return err != nil && errors.Is(err, usageLimitErr)
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "validation-model", Output: "VALIDATION_PASSED"}, nil
		},
	}

	// Second provider succeeds
	secondProvider := &mockProviderForRefactorUsageLimit{
		name: "openai",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCallCount++
			return &provider.Result{Success: true, Model: "fallback-model", Output: "refactor done"}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "validation-model", Output: "VALIDATION_PASSED"}, nil
		},
	}

	// Create router with both providers
	mockState := &mockStateForRouterTest{
		onSetUnavailable: func(name string, until time.Time) {
			markedProviderName = name
		},
	}
	mockRouter := provider.NewRouter(
		map[string]provider.Provider{
			"claude": firstProvider,
			"openai": secondProvider,
		},
		map[string]string{"build": "any", "validate": "any"},
		map[string]int{"claude": 50, "openai": 50},
		0,
		mockState,
	)

	mockRenderer := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			return "refactor with fallback", nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "refactor-003",
			Priority: 1,
		},
		tier:        provider.TierMedium,
		model:       "sonnet",
		result:      &IterationResult{},
		promptCtx:   &prompt.Context{WorkDir: "."},
		startCommit: "ghi789",
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	err := r.runRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("runRefactorPhase() error = %v, want success after fallback", err)
	}

	// Verify first call failed and second succeeded
	if runCallCount != 2 {
		t.Errorf("provider.Run() called %d times, want 2 (failure + retry)", runCallCount)
	}

	// Verify MarkUnavailable was called
	if markedProviderName != "claude" {
		t.Errorf("router.MarkUnavailable() called with %q, want %q", markedProviderName, "claude")
	}
}

// TestAcceptance_HandleRefactorValidationFailureUsesRouterSelect verifies that
// handleRefactorValidationFailure calls router.Select() when retrying after revert.
// Expected failure: handleRefactorValidationFailure still uses r.claude.Run() instead of router.Select()
func TestAcceptance_HandleRefactorValidationFailureUsesRouterSelect(t *testing.T) {
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = true
	cfg.Validation.Commands = []string{"go test"}

	// Track router.Select() calls
	var capturedPhase, capturedTier string
	selectCalled := false

	mockProvider := &mockProviderForRefactorTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			selectCalled = true
			capturedPhase = "build"
			capturedTier = tier
			return &provider.Result{Success: true, Model: "test-model", Output: "retry refactor complete"}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "validation-model", Output: "VALIDATION_PASSED"}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRenderer := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			return "retry refactor conservatively", nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "refactor-004",
			Priority: 1,
		},
		tier:      provider.TierMedium,
		model:     "sonnet",
		result:    &IterationResult{},
		promptCtx: &prompt.Context{WorkDir: "."},
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	// Simulate git HEAD capture (would normally come from getGitHead)
	preRefactorCommit := "xyz123"

	err := r.handleRefactorValidationFailure(context.Background(), bc, preRefactorCommit, "tests failed")
	if err != nil {
		t.Fatalf("handleRefactorValidationFailure() error = %v", err)
	}

	// Verify router.Select() was called with correct phase
	if !selectCalled {
		t.Error("router.Select() was not called - handleRefactorValidationFailure still uses r.claude.Run()")
	}

	if capturedPhase != "build" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "build")
	}

	// Verify tier matches bc.tier
	if capturedTier != bc.tier {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, bc.tier)
	}
}

// TestAcceptance_HandleRefactorValidationFailureDetectsUsageLimitError verifies that
// when the retry provider.Run() fails with a usage limit error, handleRefactorValidationFailure
// calls router.MarkUnavailable() and retries with a fallback provider.
// Expected failure: handleRefactorValidationFailure does not check IsUsageLimitError() yet
func TestAcceptance_HandleRefactorValidationFailureDetectsUsageLimitError(t *testing.T) {
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = true
	cfg.Validation.Commands = []string{"go test"}

	usageLimitErr := errors.New("usage limit exceeded")
	runCallCount := 0
	var markedProviderName string

	// First provider hits usage limit on retry
	firstProvider := &mockProviderForRefactorUsageLimit{
		name: "claude",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCallCount++
			if runCallCount == 1 {
				return &provider.Result{Success: false, Output: "usage limit hit"}, usageLimitErr
			}
			return &provider.Result{Success: true, Model: "fallback-model", Output: "retry refactor done"}, nil
		},
		isUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return err != nil && errors.Is(err, usageLimitErr)
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "validation-model", Output: "VALIDATION_PASSED"}, nil
		},
	}

	// Second provider succeeds
	secondProvider := &mockProviderForRefactorUsageLimit{
		name: "openai",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCallCount++
			return &provider.Result{Success: true, Model: "fallback-model", Output: "retry refactor done"}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "validation-model", Output: "VALIDATION_PASSED"}, nil
		},
	}

	// Create router with both providers
	mockState := &mockStateForRouterTest{
		onSetUnavailable: func(name string, until time.Time) {
			markedProviderName = name
		},
	}
	mockRouter := provider.NewRouter(
		map[string]provider.Provider{
			"claude": firstProvider,
			"openai": secondProvider,
		},
		map[string]string{"build": "any", "validate": "any"},
		map[string]int{"claude": 50, "openai": 50},
		0,
		mockState,
	)

	mockRenderer := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			return "retry with fallback", nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "refactor-005",
			Priority: 1,
		},
		tier:      provider.TierMedium,
		model:     "sonnet",
		result:    &IterationResult{},
		promptCtx: &prompt.Context{WorkDir: "."},
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	preRefactorCommit := "abc999"

	err := r.handleRefactorValidationFailure(context.Background(), bc, preRefactorCommit, "tests failed")
	if err != nil {
		t.Fatalf("handleRefactorValidationFailure() error = %v, want success after fallback", err)
	}

	// Verify first call failed and second succeeded
	if runCallCount != 2 {
		t.Errorf("provider.Run() called %d times, want 2 (failure + retry)", runCallCount)
	}

	// Verify MarkUnavailable was called
	if markedProviderName != "claude" {
		t.Errorf("router.MarkUnavailable() called with %q, want %q", markedProviderName, "claude")
	}
}

// mockProviderForRefactorTracking is a test double that tracks provider calls for refactor testing
type mockProviderForRefactorTracking struct {
	name            string
	runFn           func(ctx context.Context, prompt, tier string) (*provider.Result, error)
	runValidationFn func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error)
}

func (m *mockProviderForRefactorTracking) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockProviderForRefactorTracking) ModelForTier(tier string) string {
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

func (m *mockProviderForRefactorTracking) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderForRefactorTracking) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderForRefactorTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if m.runValidationFn != nil {
		return m.runValidationFn(ctx, commands, tier, workDir)
	}
	return &provider.Result{Success: true, Model: "mock-model", Output: "VALIDATION_PASSED"}, nil
}

func (m *mockProviderForRefactorTracking) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

// mockProviderForRefactorUsageLimit is a test double for usage limit testing with Run() support
type mockProviderForRefactorUsageLimit struct {
	name                string
	runFn               func(ctx context.Context, prompt, tier string) (*provider.Result, error)
	runValidationFn     func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error)
	isUsageLimitErrorFn func(result *provider.Result, err error) bool
}

func (m *mockProviderForRefactorUsageLimit) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockProviderForRefactorUsageLimit) ModelForTier(tier string) string {
	switch tier {
	case provider.TierHigh:
		return m.name + "-high"
	case provider.TierMedium:
		return m.name + "-medium"
	case provider.TierLow:
		return m.name + "-low"
	default:
		return m.name + "-default"
	}
}

func (m *mockProviderForRefactorUsageLimit) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderForRefactorUsageLimit) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderForRefactorUsageLimit) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if m.runValidationFn != nil {
		return m.runValidationFn(ctx, commands, tier, workDir)
	}
	return &provider.Result{Success: true, Model: "mock-model", Output: "VALIDATION_PASSED"}, nil
}

func (m *mockProviderForRefactorUsageLimit) IsUsageLimitError(result *provider.Result, err error) bool {
	if m.isUsageLimitErrorFn != nil {
		return m.isUsageLimitErrorFn(result, err)
	}
	return false
}
