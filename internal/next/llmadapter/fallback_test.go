package llmadapter

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// Note: mockProvider is defined in adapter_test.go (same package).

// mockProviderWithUsageLimit extends mockProvider with configurable usage-limit behavior.
type mockProviderWithUsageLimit struct {
	mockProvider
	isUsageLimit bool
}

func (m *mockProviderWithUsageLimit) IsUsageLimitError(result *provider.Result, err error) bool {
	return m.isUsageLimit
}

// mockSelectResult holds a provider/model pair for sequence-based mock router.
type mockSelectResult struct {
	prov  provider.Provider
	model string
}

// mockRouter satisfies RouterSelector with two modes:
// - Single-result mode: selectProvider/selectModel returned on every Select call
// - Sequence mode: selectSequence consumed in order (for testing fallback chains)
type mockRouter struct {
	// Single-result mode
	selectProvider provider.Provider
	selectModel    string

	// Sequence mode (takes priority over single-result if non-empty)
	selectSequence        []mockSelectResult
	selectIdx             int
	markUnavailableCalled bool
	markUnavailableName   string
}

func (m *mockRouter) Select(phase string, tier string) (provider.Provider, string) {
	if len(m.selectSequence) > 0 {
		if m.selectIdx >= len(m.selectSequence) {
			return nil, ""
		}
		r := m.selectSequence[m.selectIdx]
		m.selectIdx++
		return r.prov, r.model
	}
	return m.selectProvider, m.selectModel
}

func (m *mockRouter) MarkUnavailable(name string) {
	m.markUnavailableCalled = true
	m.markUnavailableName = name
}

func TestFallbackAdapter_NormalInvocation_NoFallback(t *testing.T) {
	primaryProv := &mockProvider{name: "claude", runResult: &provider.Result{Output: "hello", CostUSD: 0.01}}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	result, err := fa.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
}

func TestFallbackAdapter_UsageLimit_FallsBackToRouter(t *testing.T) {
	primaryProv := &mockProviderWithUsageLimit{
		mockProvider: mockProvider{name: "claude", runResult: &provider.Result{Output: "", ExitCode: 2}, runErr: errors.New("usage limit")},
		isUsageLimit: true,
	}
	fallbackResult := &provider.Result{Output: "fallback worked", CostUSD: 0.02}
	fallbackProv := &mockProvider{name: "codex", runResult: fallbackResult}
	router := &mockRouter{
		selectSequence: []mockSelectResult{
			{prov: primaryProv, model: "claude-opus"},
			{prov: fallbackProv, model: "gpt-5.3-codex"},
		},
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	result, err := fa.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "fallback worked" {
		t.Errorf("expected 'fallback worked', got %q", result.Output)
	}
	if !router.markUnavailableCalled {
		t.Error("expected router.MarkUnavailable to be called")
	}
	if router.markUnavailableName != "claude" {
		t.Errorf("expected MarkUnavailable('claude'), got %q", router.markUnavailableName)
	}
}

func TestFallbackAdapter_NonUsageLimitError_NoFallback(t *testing.T) {
	primaryProv := &mockProviderWithUsageLimit{
		mockProvider: mockProvider{name: "claude", runResult: &provider.Result{Output: ""}, runErr: errors.New("network timeout")},
		isUsageLimit: false,
	}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	_, err := fa.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !strings.Contains(err.Error(), "network timeout") {
		t.Errorf("expected original error 'network timeout' to propagate unwrapped, got %q", err.Error())
	}
}

func TestFallbackAdapter_AllProvidersExhausted_ReturnsError(t *testing.T) {
	primaryProv := &mockProviderWithUsageLimit{
		mockProvider: mockProvider{name: "claude", runResult: &provider.Result{Output: "", ExitCode: 2}, runErr: errors.New("usage limit")},
		isUsageLimit: true,
	}
	router := &mockRouter{
		selectSequence: []mockSelectResult{
			{prov: primaryProv, model: "claude-opus"},
			{prov: nil, model: ""},
		},
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	_, err := fa.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error when all providers exhausted")
	}
	if !strings.Contains(err.Error(), "all providers exhausted") {
		t.Errorf("expected 'all providers exhausted' in error, got %q", err.Error())
	}
}

func TestFallbackAdapter_SatisfiesProviderAwareInvoker(t *testing.T) {
	var _ ProviderAwareInvoker = (*FallbackAdapter)(nil)
}

func TestFallbackAdapter_Provider_ReturnsPrimaryProvider(t *testing.T) {
	primaryProv := &mockProvider{name: "claude"}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	p := fa.Provider()
	if p.Name() != "claude" {
		t.Errorf("expected provider name 'claude', got %q", p.Name())
	}
}

func TestFallbackAdapter_FallbackAlsoFails_ReturnsWrappedError(t *testing.T) {
	primaryProv := &mockProviderWithUsageLimit{
		mockProvider: mockProvider{name: "claude", runResult: &provider.Result{Output: "", ExitCode: 2}, runErr: errors.New("usage limit")},
		isUsageLimit: true,
	}
	fallbackProv := &mockProvider{name: "codex", runResult: nil, runErr: errors.New("codex internal error")}
	router := &mockRouter{
		selectSequence: []mockSelectResult{
			{prov: primaryProv, model: "claude-opus"},
			{prov: fallbackProv, model: "gpt-5.3-codex"},
		},
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	_, err := fa.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error when fallback also fails")
	}
	if !strings.Contains(err.Error(), "fallback provider") {
		t.Errorf("expected 'fallback provider' in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("expected primary provider name 'claude' in error, got %q", err.Error())
	}
}

// threadSafeMockProvider is a mock provider safe for concurrent use.
type threadSafeMockProvider struct {
	name   string
	result *provider.Result
	mu     sync.Mutex
	calls  int
}

func (m *threadSafeMockProvider) Name() string                    { return m.name }
func (m *threadSafeMockProvider) ModelForTier(tier string) string { return "mock-" + tier }
func (m *threadSafeMockProvider) Run(_ context.Context, _ string, _ string) (*provider.Result, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return m.result, nil
}
func (m *threadSafeMockProvider) StreamRun(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
	return m.result, nil
}
func (m *threadSafeMockProvider) RunValidation(_ context.Context, _ []string, _ string, _ string) (*provider.Result, error) {
	return m.result, nil
}
func (m *threadSafeMockProvider) IsUsageLimitError(_ *provider.Result, _ error) bool { return false }
func (m *threadSafeMockProvider) IsValidationPassed(_ *provider.Result) bool         { return true }
func (m *threadSafeMockProvider) IsScopeTooLarge(_ *provider.Result) (bool, string)  { return false, "" }

func TestFallbackAdapter_ConcurrentInvoke_NoRace(t *testing.T) {
	primaryProv := &threadSafeMockProvider{name: "claude", result: &provider.Result{Output: "ok", CostUSD: 0.01}}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := fa.Invoke(context.Background(), "prompt")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil || result.Output != "ok" {
				t.Errorf("unexpected result: %v", result)
			}
		}()
	}
	wg.Wait()
}

func TestFallbackAdapter_NonUsageLimitError_NoFallback_PreservesOriginalError(t *testing.T) {
	primaryProv := &mockProviderWithUsageLimit{
		mockProvider: mockProvider{name: "claude", runResult: &provider.Result{Output: ""}, runErr: errors.New("network timeout")},
		isUsageLimit: false,
	}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	_, err := fa.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !strings.Contains(err.Error(), "network timeout") {
		t.Errorf("expected original error 'network timeout' to propagate, got %q", err.Error())
	}
}

func TestFallbackAdapter_PrimaryCleared_AfterFallback(t *testing.T) {
	primaryProv := &mockProviderWithUsageLimit{
		mockProvider: mockProvider{name: "claude", runResult: &provider.Result{Output: "", ExitCode: 2}, runErr: errors.New("usage limit")},
		isUsageLimit: true,
	}
	fallbackProv := &mockProvider{name: "codex", runResult: &provider.Result{Output: "fallback ok"}}
	secondProv := &mockProvider{name: "gemini", runResult: &provider.Result{Output: "second ok"}}
	router := &mockRouter{
		selectSequence: []mockSelectResult{
			{prov: primaryProv, model: "claude-opus"},    // first Invoke: primary
			{prov: fallbackProv, model: "gpt-5.3-codex"}, // first Invoke: fallback
			{prov: secondProv, model: "gemini-pro"},      // second Invoke: re-resolved primary
		},
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")

	// First call: primary fails with usage limit, falls back to codex.
	result, err := fa.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error on first invoke: %v", err)
	}
	if result.Output != "fallback ok" {
		t.Errorf("expected 'fallback ok', got %q", result.Output)
	}

	// Second call: primary was cleared, so router.Select is called again (3rd entry).
	result, err = fa.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error on second invoke: %v", err)
	}
	if result.Output != "second ok" {
		t.Errorf("expected 'second ok' from re-resolved provider, got %q", result.Output)
	}
	// Verify all 3 Select calls were made (primary was NOT reused).
	if router.selectIdx != 3 {
		t.Errorf("expected 3 Select calls (primary + fallback + re-resolve), got %d", router.selectIdx)
	}
}

func TestFallbackAdapter_NilResultWithUsageLimitError_NoFallback(t *testing.T) {
	// When the primary returns (nil, error) with isUsageLimit: true,
	// the nil result guard on fallback.go:96 prevents the usage-limit
	// branch from executing. The error should propagate directly and
	// MarkUnavailable should NOT be called.
	primaryProv := &mockProviderWithUsageLimit{
		mockProvider: mockProvider{name: "claude", runResult: nil, runErr: errors.New("usage limit but nil result")},
		isUsageLimit: true,
	}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	result, err := fa.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if !strings.Contains(err.Error(), "usage limit but nil result") {
		t.Errorf("expected original error to propagate, got %q", err.Error())
	}
	if router.markUnavailableCalled {
		t.Error("MarkUnavailable should NOT be called when result is nil")
	}
}

func TestFallbackAdapter_Provider_ReturnsNilWhenNoProviders(t *testing.T) {
	// When the router returns (nil, "") from Select, Provider() should return nil.
	router := &mockRouter{
		selectProvider: nil,
		selectModel:    "",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	p := fa.Provider()
	if p != nil {
		t.Errorf("expected nil provider, got %v", p)
	}
}

// threadSafeMockProviderWithUsageLimit is a concurrency-safe mock that can
// simulate a usage-limit error on the first call and succeed on subsequent calls.
type threadSafeMockProviderWithUsageLimit struct {
	name           string
	mu             sync.Mutex
	calls          int
	failFirst      bool // if true, first call returns usage-limit error
	successResult  *provider.Result
	usageLimitOnce bool // tracks whether the first call has been made
}

func (m *threadSafeMockProviderWithUsageLimit) Name() string { return m.name }
func (m *threadSafeMockProviderWithUsageLimit) ModelForTier(tier string) string {
	return "mock-" + tier
}
func (m *threadSafeMockProviderWithUsageLimit) Run(_ context.Context, _ string, _ string) (*provider.Result, error) {
	m.mu.Lock()
	m.calls++
	shouldFail := m.failFirst && !m.usageLimitOnce
	if shouldFail {
		m.usageLimitOnce = true
	}
	m.mu.Unlock()
	if shouldFail {
		return &provider.Result{Output: "", ExitCode: 2}, errors.New("usage limit")
	}
	return m.successResult, nil
}
func (m *threadSafeMockProviderWithUsageLimit) StreamRun(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
	return m.Run(context.Background(), "", "")
}
func (m *threadSafeMockProviderWithUsageLimit) RunValidation(_ context.Context, _ []string, _ string, _ string) (*provider.Result, error) {
	return m.successResult, nil
}
func (m *threadSafeMockProviderWithUsageLimit) IsUsageLimitError(_ *provider.Result, err error) bool {
	return err != nil && strings.Contains(err.Error(), "usage limit")
}
func (m *threadSafeMockProviderWithUsageLimit) IsValidationPassed(_ *provider.Result) bool {
	return true
}
func (m *threadSafeMockProviderWithUsageLimit) IsScopeTooLarge(_ *provider.Result) (bool, string) {
	return false, ""
}

// threadSafeRouter is a concurrency-safe mock router for the concurrent fallback test.
type threadSafeRouter struct {
	mu                    sync.Mutex
	selectSequence        []mockSelectResult
	selectIdx             int
	markUnavailableCalled bool
	markUnavailableName   string
}

func (r *threadSafeRouter) Select(_ string, _ string) (provider.Provider, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.selectIdx >= len(r.selectSequence) {
		return nil, ""
	}
	s := r.selectSequence[r.selectIdx]
	r.selectIdx++
	return s.prov, s.model
}

func (r *threadSafeRouter) MarkUnavailable(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.markUnavailableCalled = true
	r.markUnavailableName = name
}

func TestFallbackAdapter_ConcurrentInvoke_WithFallback_NoRace(t *testing.T) {
	// One goroutine triggers a usage-limit fallback while others invoke concurrently.
	// This test verifies no data races occur when fallback and normal paths interleave.
	primaryProv := &threadSafeMockProviderWithUsageLimit{
		name:          "claude",
		failFirst:     true,
		successResult: &provider.Result{Output: "ok", CostUSD: 0.01},
	}
	fallbackProv := &threadSafeMockProvider{
		name:   "codex",
		result: &provider.Result{Output: "fallback ok", CostUSD: 0.02},
	}

	// Provide enough entries for all goroutines: each Invoke does one Select for
	// primary, and the one that hits usage-limit does a second Select for fallback.
	// We provide a generous number of entries to handle any ordering.
	const goroutines = 10
	seq := make([]mockSelectResult, 0, goroutines*2)
	for i := 0; i < goroutines*2; i++ {
		if i%2 == 0 {
			seq = append(seq, mockSelectResult{prov: primaryProv, model: "claude-opus"})
		} else {
			seq = append(seq, mockSelectResult{prov: fallbackProv, model: "gpt-5.3-codex"})
		}
	}
	router := &threadSafeRouter{selectSequence: seq}

	fa := NewFallbackAdapter(router, "build", Config{}, "medium")

	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := fa.Invoke(context.Background(), "prompt")
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	// At least some invocations should succeed. Due to the interleaving of
	// goroutines and the limited sequence, some may fail with "no providers available"
	// or "all providers exhausted" — that's acceptable. The key assertion is no race.
	successCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
		}
	}
	if successCount == 0 {
		t.Error("expected at least one successful invocation across concurrent goroutines")
	}
}

func TestFallbackAdapter_InvokeInDir_NormalInvocation(t *testing.T) {
	primaryProv := &mockProvider{name: "claude", runResult: &provider.Result{Output: "dir-hello", CostUSD: 0.01}}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	result, err := fa.InvokeInDir(context.Background(), "prompt", "/tmp/workdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "dir-hello" {
		t.Errorf("expected 'dir-hello', got %q", result.Output)
	}
}

func TestFallbackAdapter_InvokeInDir_UsageLimitFallback(t *testing.T) {
	primaryProv := &mockProviderWithUsageLimit{
		mockProvider: mockProvider{name: "claude", runResult: &provider.Result{Output: "", ExitCode: 2}, runErr: errors.New("usage limit")},
		isUsageLimit: true,
	}
	fallbackResult := &provider.Result{Output: "dir-fallback", CostUSD: 0.02}
	fallbackProv := &mockProvider{name: "codex", runResult: fallbackResult}
	router := &mockRouter{
		selectSequence: []mockSelectResult{
			{prov: primaryProv, model: "claude-opus"},
			{prov: fallbackProv, model: "gpt-5.3-codex"},
		},
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	result, err := fa.InvokeInDir(context.Background(), "prompt", "/tmp/workdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "dir-fallback" {
		t.Errorf("expected 'dir-fallback', got %q", result.Output)
	}
	if !router.markUnavailableCalled {
		t.Error("expected router.MarkUnavailable to be called")
	}
}
