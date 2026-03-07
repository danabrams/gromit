package routing_test

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/routing"
)

type stubProvider struct {
	name string
}

func (s *stubProvider) Invoke(_ context.Context, _ llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return &llmtypes.LLMInvokeResponse{}, nil
}

func (s *stubProvider) StreamInvoke(_ context.Context, _ llmtypes.LLMStreamInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return &llmtypes.LLMInvokeResponse{}, nil
}

func TestRouterSelectReturnsErrorWhenNoProviders(t *testing.T) {
	r := routing.NewRouter(routing.RouterConfig{})
	_, _, err := r.Select("build", "low")
	if err == nil {
		t.Fatal("expected error when no providers configured, got nil")
	}
}

func TestRouterMarkUnavailableCausesFallback(t *testing.T) {
	providerA := &stubProvider{name: "providerA"}
	providerB := &stubProvider{name: "providerB"}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"providerA": providerA,
			"providerB": providerB,
		},
		PhasePreferences: map[string]string{
			"build": "providerA",
		},
		Cooldown: 24 * time.Hour,
	})

	// Without mark, build phase should select providerA via preference.
	got, _, err := r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != llmtypes.LLMProvider(providerA) {
		t.Fatalf("expected providerA before mark, got %v", got)
	}

	// After marking providerA unavailable, should fall back to providerB.
	r.MarkUnavailable("providerA")
	got, _, err = r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error after mark: %v", err)
	}
	if got != llmtypes.LLMProvider(providerB) {
		t.Fatalf("expected providerB after providerA marked unavailable, got %v", got)
	}
}

func TestRouterReEnablesProviderAfterCooldown(t *testing.T) {
	providerA := &stubProvider{name: "providerA"}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"providerA": providerA,
		},
		Cooldown: time.Millisecond,
	})

	r.MarkUnavailable("providerA")

	// Immediately after marking, provider should be unavailable.
	_, _, err := r.Select("build", "low")
	if err == nil {
		t.Fatal("expected error when only provider is unavailable, got nil")
	}

	// After cooldown, provider should be available again.
	time.Sleep(5 * time.Millisecond)
	got, _, err := r.Select("build", "low")
	if err != nil {
		t.Fatalf("expected provider re-enabled after cooldown, got error: %v", err)
	}
	if got != llmtypes.LLMProvider(providerA) {
		t.Fatalf("expected providerA after cooldown, got %v", got)
	}
}

func TestRouterSelectRatioBalancesAcrossProviders(t *testing.T) {
	providerA := &stubProvider{name: "providerA"}
	providerB := &stubProvider{name: "providerB"}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"providerA": providerA,
			"providerB": providerB,
		},
		PhasePreferences: map[string]string{
			"forced": "providerA",
		},
		Ratio: map[string]int{
			"providerA": 3,
			"providerB": 1,
		},
	})

	// Use phase preference to force 4 invocations on providerA (Select auto-records).
	for i := 0; i < 4; i++ {
		_, _, err := r.Select("forced", "low")
		if err != nil {
			t.Fatalf("unexpected error on forced select %d: %v", i, err)
		}
	}
	// providerA count=4, providerB count=0.
	// providerA score = 4/3 ≈ 1.33, providerB score = 0/1 = 0 → B should be selected.
	got, _, err := r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != llmtypes.LLMProvider(providerB) {
		t.Fatalf("expected providerB selected after providerA over-served, got %v", got)
	}
}

func TestRouterSelectUsesPhasePreference(t *testing.T) {
	providerA := &stubProvider{name: "providerA"}
	providerB := &stubProvider{name: "providerB"}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"providerA": providerA,
			"providerB": providerB,
		},
		PhasePreferences: map[string]string{
			"build": "providerB",
		},
	})

	got, _, err := r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != llmtypes.LLMProvider(providerB) {
		t.Fatalf("expected providerB selected for build phase, got %v", got)
	}
}
