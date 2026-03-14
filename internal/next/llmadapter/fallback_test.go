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
