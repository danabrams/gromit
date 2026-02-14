package execution

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Mock types for narrow interfaces ---

// mockRouter implements the narrow Router interface for the execution package.
type mockRouter struct {
	selectFn          func(phase, tier string) (Provider, string)
	markUnavailableFn func(name string)
	markCalls         []string
}

func (m *mockRouter) Select(phase, tier string) (Provider, string) {
	if m.selectFn != nil {
		return m.selectFn(phase, tier)
	}
	return nil, ""
}

func (m *mockRouter) MarkUnavailable(name string) {
	m.markCalls = append(m.markCalls, name)
	if m.markUnavailableFn != nil {
		m.markUnavailableFn(name)
	}
}

// mockProvider implements the narrow Provider interface for the execution package.
type mockProvider struct {
	name           string
	streamRunFn    func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	isUsageLimitFn func(result *provider.Result, err error) bool
}

func (m *mockProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-provider"
}

func (m *mockProvider) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	if m.isUsageLimitFn != nil {
		return m.isUsageLimitFn(result, err)
	}
	return false
}

func (m *mockProvider) ModelForTier(tier string) string {
	return "test-model"
}

func (m *mockProvider) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProvider) IsValidationPassed(result *provider.Result) bool {
	return result.Success
}

func (m *mockProvider) IsScopeTooLarge(result *provider.Result) (bool, string) {
	return false, ""
}

// --- Helper ---

func newTestBeadContext() *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		BuildPrompt: "test prompt",
		Result:      &runtypes.IterationResult{},
	}
}

// --- Invoker.Execute tests ---

// Expected failure: Invoker type does not exist in execution/ package yet
func TestInvokerExecute_ReturnsInvocationResult(t *testing.T) {
	// Tests that Execute returns a properly populated InvocationResult
	// with Claude result data, model name, and provider name.
	mp := &mockProvider{
		name: "test-claude",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  "build complete",
				Model:   "claude-sonnet",
			}, nil
		},
	}

	selectCount := 0
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			selectCount++
			return mp, "claude-sonnet"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	result, err := invoker.Execute(context.Background(), bc, "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify InvocationResult contains expected data
	if result.ModelName != "claude-sonnet" {
		t.Errorf("ModelName = %q, want %q", result.ModelName, "claude-sonnet")
	}
	if result.ProviderName != "test-claude" {
		t.Errorf("ProviderName = %q, want %q", result.ProviderName, "test-claude")
	}
	if !result.Result.Success {
		t.Error("Result.Success = false, want true")
	}
	if result.Result.Output != "build complete" {
		t.Errorf("Result.Output = %q, want %q", result.Result.Output, "build complete")
	}
	if result.StallFired {
		t.Error("StallFired = true, want false for successful invocation")
	}
	if selectCount != 1 {
		t.Errorf("router.Select called %d times, want 1", selectCount)
	}
}

// Expected failure: Invoker type does not exist in execution/ package yet
func TestInvokerExecute_PropagatesModelAndProviderToBeadContext(t *testing.T) {
	// Tests that Execute updates bc.Model, bc.Result.Model, and bc.BuildProvider
	// with the router-selected values.
	mp := &mockProvider{name: "anthropic"}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "opus-4"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	bc.Tier = provider.TierHigh

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.Model != "opus-4" {
		t.Errorf("bc.Model = %q, want %q", bc.Model, "opus-4")
	}
	if bc.Result.Model != "opus-4" {
		t.Errorf("bc.Result.Model = %q, want %q", bc.Result.Model, "opus-4")
	}
	if bc.BuildProvider != "anthropic" {
		t.Errorf("bc.BuildProvider = %q, want %q", bc.BuildProvider, "anthropic")
	}
}

// Expected failure: Invoker type does not exist in execution/ package yet
func TestInvokerExecute_NilRouterReturnsError(t *testing.T) {
	invoker := NewInvoker(nil, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error for nil router, got nil")
	}
}

// Expected failure: Invoker type does not exist in execution/ package yet
func TestInvokerExecute_NoProviderAvailableReturnsError(t *testing.T) {
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return nil, ""
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error when no providers available, got nil")
	}
}

// Expected failure: Invoker type does not exist in execution/ package yet
func TestInvokerExecute_UsageLimitTriggersProviderFallback(t *testing.T) {
	// When the primary provider returns a usage limit error, Execute should
	// mark it unavailable and retry with a fallback provider.
	primaryCalled := false
	fallbackCalled := false

	primary := &mockProvider{
		name: "provider-a",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			primaryCalled = true
			return nil, fmt.Errorf("usage limit exceeded")
		},
		isUsageLimitFn: func(result *provider.Result, err error) bool {
			return true
		},
	}

	fallback := &mockProvider{
		name: "provider-b",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			fallbackCalled = true
			return &provider.Result{Success: true, Model: "fallback-model"}, nil
		},
	}

	callCount := 0
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			callCount++
			if callCount == 1 {
				return primary, "primary-model"
			}
			return fallback, "fallback-model"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	result, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error after fallback: %v", err)
	}

	if !primaryCalled {
		t.Error("primary provider should have been called")
	}
	if !fallbackCalled {
		t.Error("fallback provider should have been called")
	}
	if len(mr.markCalls) != 1 || mr.markCalls[0] != "provider-a" {
		t.Errorf("MarkUnavailable calls = %v, want [provider-a]", mr.markCalls)
	}
	if result.ModelName != "fallback-model" {
		t.Errorf("ModelName = %q, want %q after fallback", result.ModelName, "fallback-model")
	}
}

// Expected failure: Invoker type does not exist in execution/ package yet
func TestInvokerExecute_EscalatedInvocationUpdatesEscalatedTo(t *testing.T) {
	// When bc.Result.Escalated is true and EscalatedTo is set, Execute
	// should update EscalatedTo with the concrete model name from the router.
	mp := &mockProvider{name: "provider-x"}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "opus-latest"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	bc.Result.Escalated = true
	bc.Result.EscalatedTo = "placeholder" // will be overwritten

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.Result.EscalatedTo != "opus-latest" {
		t.Errorf("bc.Result.EscalatedTo = %q, want %q", bc.Result.EscalatedTo, "opus-latest")
	}
}

// Expected failure: Invoker type does not exist in execution/ package yet
func TestInvokerExecute_CapturesDiagnosticDataFromStreamStats(t *testing.T) {
	// Execute should populate bc.Result diagnostic fields from the StreamStats
	// DiagnosticSnapshot after the invocation completes.
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "test-model"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "test-model"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After execution, diagnostic fields should be populated (at minimum, set to zero values
	// since no events were recorded). The key behavior is that DiagnosticSnapshot is called
	// and its values are propagated to bc.Result.
	// StallCount should be 0 since no stall occurred
	if bc.Result.StallCount != 0 {
		t.Errorf("bc.Result.StallCount = %d, want 0", bc.Result.StallCount)
	}
	// ToolCallCount should be 0 since no tool calls were made
	if bc.Result.ToolCallCount != 0 {
		t.Errorf("bc.Result.ToolCallCount = %d, want 0", bc.Result.ToolCallCount)
	}
}

// Expected failure: Invoker type does not exist in execution/ package yet
func TestInvokerExecute_PassesTierFromBeadContext(t *testing.T) {
	// Execute should use bc.Tier when calling router.Select and pass it
	// through to provider.StreamRun.
	var capturedRouterTier string
	var capturedStreamTier string

	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			capturedStreamTier = tier
			return &provider.Result{Success: true, Model: "m"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			capturedRouterTier = tier
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	bc.Tier = provider.TierHigh

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedRouterTier != provider.TierHigh {
		t.Errorf("router received tier %q, want %q", capturedRouterTier, provider.TierHigh)
	}
	if capturedStreamTier != provider.TierHigh {
		t.Errorf("StreamRun received tier %q, want %q", capturedStreamTier, provider.TierHigh)
	}
}

// Expected failure: Invoker type does not exist in execution/ package yet
func TestInvokerExecute_StreamRunErrorPropagates(t *testing.T) {
	// When StreamRun returns a non-usage-limit error, it should propagate to the caller.
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return nil, fmt.Errorf("connection refused")
		},
		isUsageLimitFn: func(result *provider.Result, err error) bool {
			return false // not a usage limit error
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error from StreamRun failure, got nil")
	}
}

// Expected failure: InvocationResult type does not exist in execution/ package yet
func TestInvocationResult_ContainsStreamStats(t *testing.T) {
	// InvocationResult should include StreamStats for the caller to inspect.
	mp := &mockProvider{}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	result, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats == nil {
		t.Fatal("InvocationResult.Stats should not be nil")
	}

	// Stats should be a real StreamStats with valid data
	toolCalls, _, elapsed := result.Stats.Snapshot()
	if toolCalls != 0 {
		t.Errorf("Stats.ToolCalls = %d, want 0 for no-op invocation", toolCalls)
	}
	if elapsed < 0 {
		t.Error("Stats.Elapsed should be non-negative")
	}
}

// Expected failure: InvocationResult type does not exist in execution/ package yet
func TestInvocationResult_ConvertsClaudeResult(t *testing.T) {
	// The Result field in InvocationResult should be a *claude.Result,
	// converted from the provider.Result returned by StreamRun.
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:  true,
				Output:   "output text",
				ExitCode: 0,
				Duration: 5 * time.Second,
				Model:    "opus-4",
			}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "opus-4"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	invResult, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cr := invResult.Result
	if cr == nil {
		t.Fatal("InvocationResult.Result should not be nil")
	}
	if !cr.Success {
		t.Error("claude.Result.Success = false, want true")
	}
	if cr.Output != "output text" {
		t.Errorf("claude.Result.Output = %q, want %q", cr.Output, "output text")
	}
	if cr.ExitCode != 0 {
		t.Errorf("claude.Result.ExitCode = %d, want 0", cr.ExitCode)
	}
	if cr.Duration != 5*time.Second {
		t.Errorf("claude.Result.Duration = %v, want 5s", cr.Duration)
	}
}

// Expected failure: NewInvoker does not exist in execution/ package yet
func TestNewInvoker_AcceptsNarrowInterfaces(t *testing.T) {
	// Verify that NewInvoker accepts the narrow Router interface (not *provider.Router),
	// enabling mock injection without importing the provider package's concrete types.
	var buf bytes.Buffer
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return &mockProvider{}, "m"
		},
	}

	// This must compile with the narrow Router interface, not *provider.Router
	invoker := NewInvoker(mr, &buf, nil)
	bc := newTestBeadContext()

	result, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

// Suppress unused import warnings during test compilation
var _ = logger.NewStreamStats
