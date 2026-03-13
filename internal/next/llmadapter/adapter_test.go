package llmadapter

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/provider"
)

// mockProvider satisfies provider.Provider for unit tests.
type mockProvider struct {
	name      string
	runResult *provider.Result
	runErr    error
	calls     int
	lastTier  string
}

func (m *mockProvider) Name() string                   { return m.name }
func (m *mockProvider) ModelForTier(tier string) string { return "mock-" + tier }
func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	m.calls++
	m.lastTier = tier
	return m.runResult, m.runErr
}
func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	m.calls++
	m.lastTier = tier
	return m.runResult, m.runErr
}
func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return m.runResult, m.runErr
}
func (m *mockProvider) IsUsageLimitError(result *provider.Result, err error) bool { return false }
func (m *mockProvider) IsValidationPassed(result *provider.Result) bool {
	return result != nil && result.Success
}
func (m *mockProvider) IsScopeTooLarge(result *provider.Result) (bool, string) { return false, "" }

func TestInvoke_DelegatesToProviderRun(t *testing.T) {
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{Output: "hello", CostUSD: 0.05, InputTokens: 100, OutputTokens: 50},
	}
	adapter := New(mp, Config{Tier: "medium"})
	result, err := adapter.Invoke(context.Background(), "do something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello" {
		t.Errorf("expected output 'hello', got %q", result.Output)
	}
	if mp.calls != 1 {
		t.Errorf("expected 1 call, got %d", mp.calls)
	}
	if mp.lastTier != "medium" {
		t.Errorf("expected tier 'medium', got %q", mp.lastTier)
	}
}

func TestInvoke_PropagatesError(t *testing.T) {
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{Output: "partial"},
		runErr:    errors.New("api failure"),
	}
	adapter := New(mp, Config{Tier: "high"})
	result, err := adapter.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "api failure" {
		t.Errorf("expected 'api failure', got %q", err.Error())
	}
	// Result must be returned even on error — 0002d's FallbackAdapter
	// needs it for IsUsageLimitError(result, err) checks.
	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}
	if result.Output != "partial" {
		t.Errorf("expected result output 'partial', got %q", result.Output)
	}
}

func TestInvoke_CallsOnCost(t *testing.T) {
	var captured float64
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{CostUSD: 0.12},
	}
	adapter := New(mp, Config{
		Tier:   "low",
		OnCost: func(c float64) { captured = c },
	})
	_, err := adapter.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured != 0.12 {
		t.Errorf("expected cost 0.12, got %f", captured)
	}
}

func TestInvoke_OnCostNotCalledOnZero(t *testing.T) {
	called := false
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{CostUSD: 0},
	}
	adapter := New(mp, Config{
		Tier:   "low",
		OnCost: func(c float64) { called = true },
	})
	_, _ = adapter.Invoke(context.Background(), "prompt")
	if called {
		t.Error("OnCost should not be called for zero cost")
	}
}

func TestInvoke_RespectsTimeout(t *testing.T) {
	slowProvider := &slowMockProvider{delay: 5 * time.Second, result: &provider.Result{}}
	adapter := New(slowProvider, Config{
		Tier:    "low",
		Timeout: 50 * time.Millisecond,
	})
	_, err := adapter.Invoke(context.Background(), "prompt")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestInvoke_OnCostCalledOnErrorWithCost(t *testing.T) {
	var captured float64
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{Output: "partial", CostUSD: 0.03},
		runErr:    errors.New("partial failure"),
	}
	adapter := New(mp, Config{
		Tier:   "low",
		OnCost: func(c float64) { captured = c },
	})
	_, err := adapter.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error")
	}
	if captured != 0.03 {
		t.Errorf("expected cost 0.03 even on error, got %f", captured)
	}
}

func TestProviderName(t *testing.T) {
	mp := &mockProvider{name: "claude"}
	adapter := New(mp, Config{Tier: "high"})
	if adapter.ProviderName() != "claude" {
		t.Errorf("expected 'claude', got %q", adapter.ProviderName())
	}
}

func TestTier(t *testing.T) {
	mp := &mockProvider{name: "test"}
	adapter := New(mp, Config{Tier: "xhigh"})
	if adapter.Tier() != "xhigh" {
		t.Errorf("expected 'xhigh', got %q", adapter.Tier())
	}
}

func TestInvokeStream_DelegatesToProvider(t *testing.T) {
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{Output: "streamed", CostUSD: 0.08, InputTokens: 150, OutputTokens: 75},
	}
	adapter := New(mp, Config{Tier: "medium"})
	result, err := adapter.InvokeStream(context.Background(), "do something", io.Discard, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "streamed" {
		t.Errorf("expected output 'streamed', got %q", result.Output)
	}
	if mp.calls != 1 {
		t.Errorf("expected 1 call, got %d", mp.calls)
	}
	if mp.lastTier != "medium" {
		t.Errorf("expected tier 'medium', got %q", mp.lastTier)
	}
}

func TestInvokeStream_CallsOnCost(t *testing.T) {
	var captured float64
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{CostUSD: 0.15},
	}
	adapter := New(mp, Config{
		Tier:   "low",
		OnCost: func(c float64) { captured = c },
	})
	_, err := adapter.InvokeStream(context.Background(), "prompt", io.Discard, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured != 0.15 {
		t.Errorf("expected cost 0.15, got %f", captured)
	}
}

func TestInvokeStream_PropagatesError(t *testing.T) {
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{Output: "partial"},
		runErr:    errors.New("stream failure"),
	}
	adapter := New(mp, Config{Tier: "high"})
	result, err := adapter.InvokeStream(context.Background(), "prompt", io.Discard, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "stream failure" {
		t.Errorf("expected 'stream failure', got %q", err.Error())
	}
	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}
	if result.Output != "partial" {
		t.Errorf("expected result output 'partial', got %q", result.Output)
	}
}

func TestInvokeStream_RespectsTimeout(t *testing.T) {
	slowProvider := &slowMockProvider{delay: 5 * time.Second, result: &provider.Result{}}
	adapter := New(slowProvider, Config{
		Tier:    "low",
		Timeout: 50 * time.Millisecond,
	})
	_, err := adapter.InvokeStream(context.Background(), "prompt", io.Discard, nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestLLMAdapter_SatisfiesInvoker(t *testing.T) {
	var _ Invoker = (*LLMAdapter)(nil)
}

func TestLLMAdapter_SatisfiesProviderAwareInvoker(t *testing.T) {
	var _ ProviderAwareInvoker = (*LLMAdapter)(nil)
}

func TestProviderAware_SatisfiesProviderAwareInvoker(t *testing.T) {
	var _ ProviderAwareInvoker = (*ProviderAware)(nil)
}

func TestProviderAware_DelegatesInvokeAndReturnsProvider(t *testing.T) {
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{Output: "hello"},
	}
	adapter := New(mp, Config{Tier: "low"})
	pa := NewProviderAware(adapter, mp)

	result, err := pa.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello" {
		t.Errorf("expected output 'hello', got %q", result.Output)
	}
	if pa.Provider() != mp {
		t.Error("Provider() should return the underlying provider")
	}
}

func TestLLMAdapter_ProviderReturnsUnderlying(t *testing.T) {
	mp := &mockProvider{name: "claude"}
	adapter := New(mp, Config{Tier: "high"})
	if adapter.Provider() != mp {
		t.Error("Provider() should return the underlying provider")
	}
}

// slowMockProvider blocks for a configurable delay.
type slowMockProvider struct {
	mockProvider
	delay  time.Duration
	result *provider.Result
}

func (m *slowMockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	select {
	case <-time.After(m.delay):
		return m.result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *slowMockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	select {
	case <-time.After(m.delay):
		return m.result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
