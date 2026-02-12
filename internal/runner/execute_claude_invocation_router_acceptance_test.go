//go:build acceptance

package runner

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestAcceptance_ExecuteClaudeInvocationUsesRouterSelect verifies that
// executeClaudeInvocation calls router.Select() with phase="build" and tier from selectTier().
// Expected failure: executeClaudeInvocation does not yet call router.Select() - it still uses r.claude.StreamRun()
func TestAcceptance_ExecuteClaudeInvocationUsesRouterSelect(t *testing.T) {
	cfg := makeTestRunnerConfig()

	// Track router.Select() calls
	var capturedPhase, capturedTier string

	mockProvider := &mockProviderWithSelectTracking{
		onSelect: func(phase, tier string) {
			capturedPhase = phase
			capturedTier = tier
		},
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-001",
			Priority: 1,
			Labels:   []string{},
		},
		model:       "sonnet", // Legacy model name from setupBeadContext
		result:      &IterationResult{},
		buildPrompt: "implement feature X",
	}

	// Provide a mock claude client for backward compatibility (current implementation uses r.claude)
	mockClaude := &mockClaudeClientForProcess{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		output: io.Discard,
	}

	// Call executeClaudeInvocation
	_, _, _, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation() error = %v", err)
	}

	// Verify router.Select() was called with correct phase
	if capturedPhase != "build" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "build")
	}

	// Verify tier matches selectTier(bead) result
	expectedTier := r.selectTier(bc.bead)
	if capturedTier != expectedTier {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, expectedTier)
	}
}

// makeTestRunnerConfig creates a minimal config for runner tests
func makeTestRunnerConfig() *config.Config {
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: provider.TierHigh,
			P1: provider.TierMedium,
			P2: provider.TierLow,
		},
		Claude: config.ClaudeConfig{
			Binary: "claude",
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

// TestAcceptance_ExecuteClaudeInvocationCallsProviderStreamRun verifies that
// executeClaudeInvocation invokes provider.StreamRun() with the selected tier.
// Expected failure: provider.StreamRun() method does not exist yet - executeClaudeInvocation uses claude.StreamRun()
func TestAcceptance_ExecuteClaudeInvocationCallsProviderStreamRun(t *testing.T) {
	cfg := makeTestRunnerConfig()

	streamRunCalled := false
	var capturedPrompt, capturedTier string

	mockProvider := &mockProviderWithSelectTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamRunCalled = true
			capturedPrompt = prompt
			capturedTier = tier
			return &provider.Result{Success: true, Model: "test-model", Output: "done"}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-002",
			Priority: 1,
		},
		result:      &IterationResult{},
		buildPrompt: "implement authentication",
	}

	mockClaude := &mockClaudeClientForProcess{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		output: io.Discard,
	}

	_, _, _, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation() error = %v", err)
	}

	if !streamRunCalled {
		t.Error("provider.StreamRun() was not called - executeClaudeInvocation does not invoke the provider")
	}

	if capturedPrompt != bc.buildPrompt {
		t.Errorf("provider.StreamRun() prompt = %q, want %q", capturedPrompt, bc.buildPrompt)
	}

	expectedTier := r.selectTier(bc.bead)
	if capturedTier != expectedTier {
		t.Errorf("provider.StreamRun() tier = %q, want %q", capturedTier, expectedTier)
	}
}

// TestAcceptance_ExecuteClaudeInvocationDetectsUsageLimitError verifies that
// when provider.StreamRun() fails and provider.IsUsageLimitError() returns true,
// executeClaudeInvocation calls router.MarkUnavailable() and retries with a new provider.
// Expected failure: IsUsageLimitError() check does not exist in executeClaudeInvocation yet
func TestAcceptance_ExecuteClaudeInvocationDetectsUsageLimitError(t *testing.T) {
	cfg := makeTestRunnerConfig()

	usageLimitErr := errors.New("usage limit exceeded")
	streamRunCallCount := 0
	var markedProviderName string

	// First provider hits usage limit
	firstProvider := &mockProviderWithUsageLimitTracking{
		name: "claude",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamRunCallCount++
			if streamRunCallCount == 1 {
				return &provider.Result{Success: false, Output: "usage limit hit"}, usageLimitErr
			}
			return &provider.Result{Success: true, Model: "fallback-model", Output: "done"}, nil
		},
		isUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return err != nil && errors.Is(err, usageLimitErr)
		},
		onMarkUnavailable: func(name string) {
			markedProviderName = name
		},
	}

	// Second provider succeeds
	secondProvider := &mockProviderWithUsageLimitTracking{
		name: "openai",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamRunCallCount++
			return &provider.Result{Success: true, Model: "fallback-model", Output: "done"}, nil
		},
	}

	// Create router with both providers
	mockState := &mockStateForRouterTest{
		onSetUnavailable: func(name string, until time.Time) {
			markedProviderName = name
			if firstProvider.onMarkUnavailable != nil {
				firstProvider.onMarkUnavailable(name)
			}
		},
	}
	mockRouter := provider.NewRouter(
		map[string]provider.Provider{
			"claude": firstProvider,
			"openai": secondProvider,
		},
		map[string]string{"build": "any"},
		map[string]int{"claude": 50, "openai": 50},
		0,
		mockState,
	)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-003",
			Priority: 1,
		},
		result:      &IterationResult{},
		buildPrompt: "implement feature with fallback",
	}

	mockClaude := &mockClaudeClientForProcess{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		output: io.Discard,
	}

	result, _, _, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation() error = %v, want success after fallback", err)
	}

	// Verify first call failed and second succeeded
	if streamRunCallCount != 2 {
		t.Errorf("provider.StreamRun() called %d times, want 2 (failure + retry)", streamRunCallCount)
	}

	// Verify MarkUnavailable was called
	if markedProviderName != "claude" {
		t.Errorf("router.MarkUnavailable() called with %q, want %q", markedProviderName, "claude")
	}

	// Verify retry succeeded
	if result == nil || !result.Success {
		t.Error("executeClaudeInvocation() should succeed after fallback to available provider")
	}
}

// TestAcceptance_ExecuteClaudeInvocationReturnsErrorWhenAllProvidersUnavailable verifies
// that when router.Select() returns nil provider, executeClaudeInvocation returns an error.
// Expected failure: executeClaudeInvocation does not check for nil provider from router.Select()
func TestAcceptance_ExecuteClaudeInvocationReturnsErrorWhenAllProvidersUnavailable(t *testing.T) {
	cfg := makeTestRunnerConfig()

	// Create router with no providers (empty map) to simulate all unavailable
	mockState := &mockStateForRouterTest{}
	mockRouter := provider.NewRouter(
		map[string]provider.Provider{},
		map[string]string{"build": "any"},
		map[string]int{},
		0,
		mockState,
	)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-004",
			Priority: 1,
		},
		result:      &IterationResult{},
		buildPrompt: "should fail with no providers",
	}

	mockClaude := &mockClaudeClientForProcess{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		output: io.Discard,
	}

	_, _, _, err := r.executeClaudeInvocation(context.Background(), bc)
	if err == nil {
		t.Fatal("executeClaudeInvocation() should return error when all providers unavailable")
	}

	if err.Error() == "" {
		t.Error("executeClaudeInvocation() error message should not be empty")
	}
}

// TestAcceptance_ExecuteClaudeInvocationUpdatesBeadContextModel verifies that
// the model name returned from router.Select() updates bc.model and bc.result.Model.
// Expected failure: executeClaudeInvocation does not update bc.model with the router-selected model
func TestAcceptance_ExecuteClaudeInvocationUpdatesBeadContextModel(t *testing.T) {
	cfg := makeTestRunnerConfig()

	routerReturnedModel := "provider-specific-model-name"

	mockProvider := &mockProviderWithSelectTracking{
		streamRunResult: &provider.Result{Success: true, Model: routerReturnedModel, Output: "done"},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-005",
			Priority: 1,
		},
		model:       "legacy-model",
		result:      &IterationResult{Model: "legacy-model"},
		buildPrompt: "test model update",
	}

	mockClaude := &mockClaudeClientForProcess{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		output: io.Discard,
	}

	_, _, _, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation() error = %v", err)
	}

	// Verify bc.model was updated to router-selected model
	if bc.model != routerReturnedModel {
		t.Errorf("bc.model = %q, want %q (router-selected model)", bc.model, routerReturnedModel)
	}

	// Verify bc.result.Model was updated
	if bc.result.Model != routerReturnedModel {
		t.Errorf("bc.result.Model = %q, want %q (router-selected model)", bc.result.Model, routerReturnedModel)
	}
}

// mockProviderWithSelectTracking is a test double that tracks router.Select() calls
type mockProviderWithSelectTracking struct {
	name            string
	onSelect        func(phase, tier string)
	streamRunFn     func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	streamRunResult *provider.Result
	streamRunErr    error
}

func (m *mockProviderWithSelectTracking) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockProviderWithSelectTracking) ModelForTier(tier string) string {
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

func (m *mockProviderWithSelectTracking) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if m.onSelect != nil {
		m.onSelect("", tier)
	}
	modelName := "mock-model"
	if m.streamRunResult != nil && m.streamRunResult.Model != "" {
		modelName = m.streamRunResult.Model
	}
	return &provider.Result{Success: true, Model: modelName}, nil
}

func (m *mockProviderWithSelectTracking) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.onSelect != nil {
		m.onSelect("build", tier)
	}
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	if m.streamRunResult != nil {
		return m.streamRunResult, m.streamRunErr
	}
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderWithSelectTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderWithSelectTracking) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

// mockProviderWithUsageLimitTracking is a test double for usage limit testing
type mockProviderWithUsageLimitTracking struct {
	name                string
	streamRunFn         func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	isUsageLimitErrorFn func(result *provider.Result, err error) bool
	onMarkUnavailable   func(name string)
}

func (m *mockProviderWithUsageLimitTracking) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockProviderWithUsageLimitTracking) ModelForTier(tier string) string {
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

func (m *mockProviderWithUsageLimitTracking) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderWithUsageLimitTracking) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderWithUsageLimitTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderWithUsageLimitTracking) IsUsageLimitError(result *provider.Result, err error) bool {
	if m.isUsageLimitErrorFn != nil {
		return m.isUsageLimitErrorFn(result, err)
	}
	return false
}

// mockClaudeClientForProcess provides backward-compatible claude client for tests.
// This allows tests to run against current implementation (which uses r.claude)
// without panicking, while still testing the new router behavior.
// Tests will pass using this mock but should fail their assertions because
// router.Select() wasn't called.
type mockClaudeClientForProcess struct{}

func (m *mockClaudeClientForProcess) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	return &claude.Result{Success: true, Model: model, Output: "done"}, nil
}

func (m *mockClaudeClientForProcess) StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
	// Current implementation uses this path - tests should fail because router.Select() wasn't called
	return &claude.Result{Success: true, Model: model, Output: "done"}, nil
}

func (m *mockClaudeClientForProcess) RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
	return &claude.Result{Success: true, Model: model}, nil
}

// mockStateForRouterTest provides a minimal StateFile implementation for router tests
type mockStateForRouterTest struct {
	counts           map[string]int
	onSetUnavailable func(string, time.Time)
}

func (m *mockStateForRouterTest) IncrementProviderCount(provider string) {
	if m.counts == nil {
		m.counts = make(map[string]int)
	}
	m.counts[provider]++
}

func (m *mockStateForRouterTest) GetProviderCounts() map[string]int {
	if m.counts == nil {
		return make(map[string]int)
	}
	return m.counts
}

func (m *mockStateForRouterTest) IsProviderAvailable(provider string) bool {
	return true
}

func (m *mockStateForRouterTest) SetProviderUnavailable(provider string, until time.Time) {
	if m.onSetUnavailable != nil {
		m.onSetUnavailable(provider, until)
	}
}
