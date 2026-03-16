package llmadapter

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
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

func (m *mockProvider) Name() string                    { return m.name }
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

// TestInvoke_OnInvocation_PhaseIsStageNotTier verifies that the Phase field in
// InvocationRecord is populated from Config.Phase (the stage name like "plan"),
// not from Config.Tier (the model tier like "high").
//
// RED: Config has no Phase field — invocations currently record Tier in Phase.
// GREEN after: Config.Phase wired through; fireCallbacks uses cfg.Phase.
func TestInvoke_OnInvocation_PhaseIsStageNotTier(t *testing.T) {
	var recorded runstore.InvocationRecord
	mp := &mockProvider{
		name:      "claude",
		runResult: &provider.Result{CostUSD: 0.05, InputTokens: 100, OutputTokens: 50},
	}
	adapter := New(mp, Config{
		Phase:        "plan", // stage name — RED: field does not exist yet
		Tier:         "high",
		OnInvocation: func(r runstore.InvocationRecord) { recorded = r },
	})
	_, err := adapter.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recorded.Phase != "plan" {
		t.Errorf("expected Phase='plan' (stage name), got %q — tier must not be used as phase", recorded.Phase)
	}
	if recorded.Tier != "high" {
		t.Errorf("expected Tier='high', got %q", recorded.Tier)
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

func TestInvokeStream_OnCostCalledOnErrorWithCost(t *testing.T) {
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
	_, err := adapter.InvokeStream(context.Background(), "prompt", io.Discard, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if captured != 0.03 {
		t.Errorf("expected cost 0.03 even on error, got %f", captured)
	}
}

func TestInvokeStream_NilResultOnError(t *testing.T) {
	mp := &mockProvider{
		name:      "test",
		runResult: nil,
		runErr:    errors.New("total failure"),
	}
	adapter := New(mp, Config{
		Tier:   "low",
		OnCost: func(c float64) { t.Fatal("OnCost should not be called when result is nil") },
	})
	result, err := adapter.InvokeStream(context.Background(), "prompt", io.Discard, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestInvoke_Timeout_ResultIsNil(t *testing.T) {
	slowProvider := &slowMockProvider{delay: 5 * time.Second, result: &provider.Result{}}
	adapter := New(slowProvider, Config{
		Tier:    "low",
		Timeout: 50 * time.Millisecond,
	})
	result, err := adapter.Invoke(context.Background(), "prompt")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result on timeout, got %+v", result)
	}
}

func TestInvokeStream_OnCostNotCalledOnZero(t *testing.T) {
	called := false
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{CostUSD: 0},
	}
	adapter := New(mp, Config{
		Tier:   "low",
		OnCost: func(c float64) { called = true },
	})
	_, _ = adapter.InvokeStream(context.Background(), "prompt", io.Discard, nil, nil)
	if called {
		t.Error("OnCost should not be called for zero cost")
	}
}

func TestInvoke_NilResultOnError(t *testing.T) {
	mp := &mockProvider{
		name:      "test",
		runResult: nil,
		runErr:    errors.New("total failure"),
	}
	adapter := New(mp, Config{
		Tier:   "low",
		OnCost: func(c float64) { t.Fatal("OnCost should not be called when result is nil") },
	})
	result, err := adapter.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestInvokeStream_Timeout_ResultIsNil(t *testing.T) {
	slowProvider := &slowMockProvider{delay: 5 * time.Second, result: &provider.Result{}}
	adapter := New(slowProvider, Config{
		Tier:    "low",
		Timeout: 50 * time.Millisecond,
	})
	result, err := adapter.InvokeStream(context.Background(), "prompt", io.Discard, nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result on timeout, got %+v", result)
	}
}

// TestInvoke_TimeoutEnforcement is the Scenario 5 (0002c/0002d) evidence test.
// It verifies that Config.Timeout is enforced as a hard per-invocation deadline:
// when the provider takes longer than the configured timeout, the call returns
// context.DeadlineExceeded and a nil result, and the adapter does not hang.
func TestInvoke_TimeoutEnforcement(t *testing.T) {
	const timeout = 50 * time.Millisecond
	slowProv := &slowMockProvider{delay: 5 * time.Second, result: &provider.Result{Output: "should not arrive"}}
	adapter := New(slowProv, Config{Tier: "medium", Timeout: timeout})

	start := time.Now()
	result, err := adapter.Invoke(context.Background(), "prompt")
	elapsed := time.Since(start)

	// Deadline must be exceeded — not some other error.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	// Result must be nil so callers don't process partial data.
	if result != nil {
		t.Errorf("expected nil result on timeout, got %+v", result)
	}
	// Adapter must not hang — completion within 10× the configured timeout.
	if elapsed > 10*timeout {
		t.Errorf("adapter took %v to time out (timeout=%v) — may be hanging", elapsed, timeout)
	}
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

// mockDirStreamProvider satisfies provider.Provider and provider.DirStreamRunner.
type mockDirStreamProvider struct {
	mockProvider
	lastDir string
}

func (m *mockDirStreamProvider) StreamRunInDir(_ context.Context, _ string, tier string, dir string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
	m.calls++
	m.lastTier = tier
	m.lastDir = dir
	return m.runResult, m.runErr
}

func TestInvokeInDir_UsesDirStreamRunnerWhenAvailable(t *testing.T) {
	mp := &mockDirStreamProvider{
		mockProvider: mockProvider{
			name:      "claude",
			runResult: &provider.Result{Output: "dir-streamed", CostUSD: 0.10},
		},
	}
	adapter := New(mp, Config{Tier: "high"})
	result, err := adapter.InvokeInDir(context.Background(), "prompt", "/tmp/workdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "dir-streamed" {
		t.Errorf("expected output 'dir-streamed', got %q", result.Output)
	}
	if mp.lastDir != "/tmp/workdir" {
		t.Errorf("expected dir '/tmp/workdir', got %q", mp.lastDir)
	}
	if mp.lastTier != "high" {
		t.Errorf("expected tier 'high', got %q", mp.lastTier)
	}
}

func TestInvokeInDir_FallsBackToRunWhenNotDirStreamRunner(t *testing.T) {
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{Output: "run-fallback", CostUSD: 0.05},
	}
	adapter := New(mp, Config{Tier: "medium"})
	result, err := adapter.InvokeInDir(context.Background(), "prompt", "/tmp/workdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "run-fallback" {
		t.Errorf("expected output 'run-fallback', got %q", result.Output)
	}
	if mp.calls != 1 {
		t.Errorf("expected 1 call to Run, got %d", mp.calls)
	}
}

func TestInvokeInDir_CallsOnCost(t *testing.T) {
	var captured float64
	mp := &mockDirStreamProvider{
		mockProvider: mockProvider{
			name:      "claude",
			runResult: &provider.Result{CostUSD: 0.20},
		},
	}
	adapter := New(mp, Config{
		Tier:   "high",
		OnCost: func(c float64) { captured = c },
	})
	_, err := adapter.InvokeInDir(context.Background(), "prompt", "/tmp/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured != 0.20 {
		t.Errorf("expected cost 0.20, got %f", captured)
	}
}

func TestInvokeInDir_RespectsTimeout(t *testing.T) {
	slowProv := &slowMockProvider{delay: 5 * time.Second, result: &provider.Result{}}
	adapter := New(slowProv, Config{
		Tier:    "low",
		Timeout: 50 * time.Millisecond,
	})
	_, err := adapter.InvokeInDir(context.Background(), "prompt", "/tmp/dir")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestInvokeInDir_PropagatesError(t *testing.T) {
	mp := &mockDirStreamProvider{
		mockProvider: mockProvider{
			name:      "claude",
			runResult: &provider.Result{Output: "partial"},
			runErr:    errors.New("dir stream failure"),
		},
	}
	adapter := New(mp, Config{Tier: "high"})
	result, err := adapter.InvokeInDir(context.Background(), "prompt", "/tmp/dir")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "dir stream failure" {
		t.Errorf("expected 'dir stream failure', got %q", err.Error())
	}
	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}
}

// costAwareMockProvider returns different results for Run vs StreamRun,
// allowing tests to verify which method Invoke() delegates to.
// Run returns CostUSD=0 (simulating the real claude.Client.Run which has no --output-format stream-json).
// StreamRun returns CostUSD=0.05 (simulating real cost from JSON stream parsing).
type costAwareMockProvider struct {
	name            string
	runResult       *provider.Result
	streamRunResult *provider.Result
}

func (m *costAwareMockProvider) Name() string                    { return m.name }
func (m *costAwareMockProvider) ModelForTier(tier string) string { return "mock-" + tier }
func (m *costAwareMockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return m.runResult, nil
}
func (m *costAwareMockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return m.streamRunResult, nil
}
func (m *costAwareMockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return m.runResult, nil
}
func (m *costAwareMockProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
func (m *costAwareMockProvider) IsValidationPassed(result *provider.Result) bool {
	return result != nil && result.Success
}
func (m *costAwareMockProvider) IsScopeTooLarge(result *provider.Result) (bool, string) {
	return false, ""
}

// TestInvoke_UsesStreamRun_ForCostCapture verifies that Invoke() uses StreamRun
// (not Run) so that cost data from the JSON event stream is captured in InvocationRecord.
//
// ROOT CAUSE: claude.Client.Run() uses -p (print mode) without --output-format stream-json,
// so CostUSD is always 0. StreamRun parses the JSON event stream and captures real cost.
// Plan/review/accept all call Invoke(), so their invocations had cost_usd=0.
//
// RED: Invoke() currently delegates to provider.Run → cost=0, test fails.
// GREEN after: Invoke() delegates to provider.StreamRun(io.Discard) → cost captured.
func TestInvoke_UsesStreamRun_ForCostCapture(t *testing.T) {
	mp := &costAwareMockProvider{
		name:            "claude",
		runResult:       &provider.Result{Output: "from-run", CostUSD: 0},
		streamRunResult: &provider.Result{Output: "from-stream", CostUSD: 0.05, InputTokens: 100, OutputTokens: 50},
	}
	var recorded runstore.InvocationRecord
	adapter := New(mp, Config{
		Phase:        "plan",
		Tier:         "high",
		OnInvocation: func(r runstore.InvocationRecord) { recorded = r },
	})
	_, err := adapter.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recorded.CostUSD == 0 {
		t.Errorf("Invoke must use StreamRun to capture cost; got cost_usd=0 (Run was used instead of StreamRun)")
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
