//go:build acceptance

package runner

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestAcceptance_RunAcceptanceTestsUsesRouter verifies that
// runAcceptanceTests calls router.Select() with phase="build" and the tier from beadContext.
// Expected failure: runAcceptanceTests currently calls r.claude.StreamRun() directly, not router.Select()
func TestAcceptance_RunAcceptanceTestsUsesRouter(t *testing.T) {
	cfg := makeTestRunnerConfig()

	// Track router.Select() calls
	var capturedPhase, capturedTier string
	selectCallCount := 0

	mockProvider := &mockProviderForAcceptanceTests{
		onSelect: func(phase, tier string) {
			selectCallCount++
			capturedPhase = phase
			capturedTier = tier
		},
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "tests written"},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-atdd-001",
			Priority: 1,
			Labels:   []string{},
		},
		tier:   provider.TierMedium, // From setupBeadContext via selectTier
		model:  "sonnet",             // Legacy model name for display
		result: &IterationResult{},
		promptCtx: &prompt.Context{
			Model:              "sonnet",
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}

	mockRenderer := &mockPromptRenderer{
		LearningsFile: nil,
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	err := r.runAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("runAcceptanceTests() error = %v", err)
	}

	// Verify router.Select() was called
	if selectCallCount == 0 {
		t.Error("router.Select() was not called - runAcceptanceTests still uses r.claude.StreamRun()")
	}

	// Verify phase is "build" for acceptance tests
	if capturedPhase != "build" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "build")
	}

	// Verify tier matches beadContext.tier
	if capturedTier != bc.tier {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, bc.tier)
	}
}

// TestAcceptance_RunAcceptanceTestsCallsProviderStreamRun verifies that
// runAcceptanceTests invokes provider.StreamRun() with the tier instead of model name.
// Expected failure: provider.StreamRun() signature takes tier parameter but runAcceptanceTests still calls r.claude.StreamRun() with model name
func TestAcceptance_RunAcceptanceTestsCallsProviderStreamRun(t *testing.T) {
	cfg := makeTestRunnerConfig()

	streamRunCalled := false
	var capturedTier string

	mockProvider := &mockProviderForAcceptanceTests{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamRunCalled = true
			capturedTier = tier
			return &provider.Result{Success: true, Model: "sonnet", Output: "tests written"}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-atdd-002",
			Priority: 1,
		},
		tier:   provider.TierMedium,
		model:  "sonnet",
		result: &IterationResult{},
		promptCtx: &prompt.Context{
			Model:              "sonnet",
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}

	mockRenderer := &mockPromptRenderer{
		LearningsFile: nil,
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	err := r.runAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("runAcceptanceTests() error = %v", err)
	}

	if !streamRunCalled {
		t.Error("provider.StreamRun() was not called - runAcceptanceTests does not use router-based invocation")
	}

	// Verify tier (not model name) was passed
	if capturedTier != bc.tier {
		t.Errorf("provider.StreamRun() tier = %q, want %q", capturedTier, bc.tier)
	}
}

// TestAcceptance_RunAcceptanceTestsDetectsUsageLimitError verifies that
// when provider.StreamRun() fails with a usage limit error, runAcceptanceTests
// calls router.MarkUnavailable() and retries with a fallback provider.
// Expected failure: usage limit detection and fallback logic does not exist in runAcceptanceTests yet
func TestAcceptance_RunAcceptanceTestsDetectsUsageLimitError(t *testing.T) {
	cfg := makeTestRunnerConfig()

	usageLimitErr := errors.New("usage limit exceeded")
	streamRunCallCount := 0
	var markedProviderName string

	// First provider hits usage limit
	firstProvider := &mockProviderWithUsageLimit{
		name: "claude",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamRunCallCount++
			if streamRunCallCount == 1 {
				return &provider.Result{Success: false, Output: "usage limit hit"}, usageLimitErr
			}
			return &provider.Result{Success: true, Model: "fallback-model", Output: "tests written"}, nil
		},
		isUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return err != nil && errors.Is(err, usageLimitErr)
		},
	}

	// Second provider succeeds
	secondProvider := &mockProviderWithUsageLimit{
		name: "openai",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamRunCallCount++
			return &provider.Result{Success: true, Model: "gpt-4o", Output: "tests written"}, nil
		},
	}

	mockState := &mockStateForAcceptanceTests{
		onSetUnavailable: func(name string, until time.Time) {
			markedProviderName = name
		},
	}

	mockRouter := provider.NewRouter(
		map[string]provider.Provider{
			"claude": firstProvider,
			"openai": secondProvider,
		},
		map[string]string{"build": "any"},
		map[string]int{"claude": 60, "openai": 40},
		30*time.Minute,
		mockState,
	)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-atdd-003",
			Priority: 1,
		},
		tier:   provider.TierMedium,
		model:  "sonnet",
		result: &IterationResult{},
		promptCtx: &prompt.Context{
			Model:              "sonnet",
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}

	mockRenderer := &mockPromptRenderer{
		LearningsFile: nil,
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	err := r.runAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("runAcceptanceTests() should succeed with fallback provider, got error: %v", err)
	}

	// Verify usage limit was detected and provider was marked unavailable
	if markedProviderName != "claude" {
		t.Errorf("router.MarkUnavailable() called with %q, want %q", markedProviderName, "claude")
	}

	// Verify retry occurred (2 calls: first provider fails, second provider succeeds)
	if streamRunCallCount != 2 {
		t.Errorf("provider.StreamRun() called %d times, want 2 (initial + retry)", streamRunCallCount)
	}
}

// TestAcceptance_RunAcceptanceTestsUpdatesBeadContextModel verifies that
// after router.Select() returns a concrete model name, runAcceptanceTests
// updates bc.model with the router-selected model name.
// Expected failure: runAcceptanceTests does not update bc.model with router-selected model name yet
func TestAcceptance_RunAcceptanceTestsUpdatesBeadContextModel(t *testing.T) {
	cfg := makeTestRunnerConfig()

	mockProvider := &mockProviderForAcceptanceTests{
		streamRunResult: &provider.Result{Success: true, Model: "claude-sonnet-4", Output: "tests written"},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-atdd-004",
			Priority: 1,
		},
		tier:   provider.TierMedium,
		model:  "sonnet", // Legacy name before router selection
		result: &IterationResult{},
		promptCtx: &prompt.Context{
			Model:              "sonnet",
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}

	mockRenderer := &mockPromptRenderer{
		LearningsFile: nil,
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	err := r.runAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("runAcceptanceTests() error = %v", err)
	}

	// Verify bc.model was updated with router-selected model name
	expectedModel := mockProvider.ModelForTier(bc.tier)
	if bc.model != expectedModel {
		t.Errorf("bc.model = %q, want %q (router-selected model name)", bc.model, expectedModel)
	}
}

// TestAcceptance_RunAcceptanceTestsFallbackWhenFirstProviderUnavailable verifies that
// when the first provider is marked unavailable, runAcceptanceTests immediately
// selects the second provider without attempting the first one.
// Expected failure: router selection with unavailable providers not integrated into runAcceptanceTests yet
func TestAcceptance_RunAcceptanceTestsFallbackWhenFirstProviderUnavailable(t *testing.T) {
	cfg := makeTestRunnerConfig()

	firstProviderCalled := false
	secondProviderCalled := false

	firstProvider := &mockProviderWithUsageLimit{
		name: "claude",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			firstProviderCalled = true
			return &provider.Result{Success: true, Model: "claude-sonnet", Output: "tests written"}, nil
		},
	}

	secondProvider := &mockProviderWithUsageLimit{
		name: "openai",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			secondProviderCalled = true
			return &provider.Result{Success: true, Model: "gpt-4o", Output: "tests written"}, nil
		},
	}

	mockState := &mockStateForAcceptanceTests{}

	mockRouter := provider.NewRouter(
		map[string]provider.Provider{
			"claude": firstProvider,
			"openai": secondProvider,
		},
		map[string]string{"build": "any"},
		map[string]int{"claude": 60, "openai": 40},
		30*time.Minute,
		mockState,
	)

	// Mark first provider unavailable
	mockRouter.MarkUnavailable("claude")

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-atdd-005",
			Priority: 1,
		},
		tier:   provider.TierMedium,
		model:  "sonnet",
		result: &IterationResult{},
		promptCtx: &prompt.Context{
			Model:              "sonnet",
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}

	mockRenderer := &mockPromptRenderer{
		LearningsFile: nil,
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	err := r.runAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("runAcceptanceTests() error = %v", err)
	}

	// Verify first provider was not called (it's unavailable)
	if firstProviderCalled {
		t.Error("first provider (claude) was called despite being marked unavailable")
	}

	// Verify second provider was called
	if !secondProviderCalled {
		t.Error("second provider (openai) was not called - router should select available provider")
	}
}

// TestAcceptance_RunAcceptanceTestsReturnsErrorWhenAllProvidersUnavailable verifies that
// when router.Select() returns nil (all providers unavailable), runAcceptanceTests
// returns an error instead of panicking.
// Expected failure: runAcceptanceTests does not check for nil provider from router.Select() yet
func TestAcceptance_RunAcceptanceTestsReturnsErrorWhenAllProvidersUnavailable(t *testing.T) {
	cfg := makeTestRunnerConfig()

	mockProvider := &mockProviderWithUsageLimit{
		name: "claude",
	}

	mockState := &mockStateForAcceptanceTests{}

	mockRouter := provider.NewRouter(
		map[string]provider.Provider{
			"claude": mockProvider,
		},
		map[string]string{"build": "any"},
		map[string]int{"claude": 100},
		30*time.Minute,
		mockState,
	)

	// Mark the only provider unavailable
	mockRouter.MarkUnavailable("claude")

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-atdd-006",
			Priority: 1,
		},
		tier:   provider.TierMedium,
		model:  "sonnet",
		result: &IterationResult{},
		promptCtx: &prompt.Context{
			Model:              "sonnet",
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}

	mockRenderer := &mockPromptRenderer{
		LearningsFile: nil,
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	err := r.runAcceptanceTests(context.Background(), bc)
	if err == nil {
		t.Fatal("runAcceptanceTests() should return error when all providers unavailable, got nil")
	}

	// Verify error message mentions provider availability
	if !strings.Contains(err.Error(), "provider") && !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("error message should mention unavailable providers, got: %v", err)
	}
}

// Mock implementations for acceptance tests

type mockProviderForAcceptanceTests struct {
	onSelect        func(phase, tier string)
	streamRunFn     func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	streamRunResult *provider.Result
}

func (m *mockProviderForAcceptanceTests) Name() string {
	return "mock-provider"
}

func (m *mockProviderForAcceptanceTests) ModelForTier(tier string) string {
	// Return tier-specific model names
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

func (m *mockProviderForAcceptanceTests) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: m.ModelForTier(tier)}, nil
}

func (m *mockProviderForAcceptanceTests) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.onSelect != nil {
		m.onSelect("build", tier)
	}
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	if m.streamRunResult != nil {
		return m.streamRunResult, nil
	}
	return &provider.Result{Success: true, Model: m.ModelForTier(tier)}, nil
}

func (m *mockProviderForAcceptanceTests) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: m.ModelForTier(tier)}, nil
}

func (m *mockProviderForAcceptanceTests) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

type mockProviderWithUsageLimit struct {
	name                string
	streamRunFn         func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	isUsageLimitErrorFn func(result *provider.Result, err error) bool
}

func (m *mockProviderWithUsageLimit) Name() string {
	return m.name
}

func (m *mockProviderWithUsageLimit) ModelForTier(tier string) string {
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

func (m *mockProviderWithUsageLimit) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: m.ModelForTier(tier)}, nil
}

func (m *mockProviderWithUsageLimit) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	return &provider.Result{Success: true, Model: m.ModelForTier(tier)}, nil
}

func (m *mockProviderWithUsageLimit) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: m.ModelForTier(tier)}, nil
}

func (m *mockProviderWithUsageLimit) IsUsageLimitError(result *provider.Result, err error) bool {
	if m.isUsageLimitErrorFn != nil {
		return m.isUsageLimitErrorFn(result, err)
	}
	return false
}

type mockStateForAcceptanceTests struct {
	onSetUnavailable func(name string, until time.Time)
}

func (m *mockStateForAcceptanceTests) IncrementProviderCount(provider string) {
}

func (m *mockStateForAcceptanceTests) GetProviderCounts() map[string]int {
	return map[string]int{}
}

func (m *mockStateForAcceptanceTests) IsProviderAvailable(provider string) bool {
	return true
}

func (m *mockStateForAcceptanceTests) SetProviderUnavailable(provider string, until time.Time) {
	if m.onSetUnavailable != nil {
		m.onSetUnavailable(provider, until)
	}
}
