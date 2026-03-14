package llmadapter

import (
	"context"
	"errors"
	"strings"
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
