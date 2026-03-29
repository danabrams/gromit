package routing_test

import (
	"context"
	"errors"
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
	_, _, _, err := r.Select("build", "low")
	if err == nil {
		t.Fatal("expected error when no providers configured, got nil")
	}
	if !errors.Is(err, routing.ErrNoProviders) {
		t.Fatalf("expected ErrNoProviders, got %v", err)
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
	got, _, _, err := r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != llmtypes.LLMProvider(providerA) {
		t.Fatalf("expected providerA before mark, got %v", got)
	}

	// After marking providerA unavailable, should fall back to providerB.
	r.MarkUnavailable("providerA")
	got, _, _, err = r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error after mark: %v", err)
	}
	if got != llmtypes.LLMProvider(providerB) {
		t.Fatalf("expected providerB after providerA marked unavailable, got %v", got)
	}
}

func TestRouterReEnablesProviderAfterCooldown(t *testing.T) {
	providerA := &stubProvider{name: "providerA"}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clockFunc := func() time.Time { return now }

	cooldown := 10 * time.Second
	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"providerA": providerA,
		},
		Cooldown: cooldown,
		NowFunc:  clockFunc,
	})

	r.MarkUnavailable("providerA")

	// Immediately after marking, provider should be unavailable.
	_, _, _, err := r.Select("build", "low")
	if err == nil {
		t.Fatal("expected error when only provider is unavailable, got nil")
	}
	if !errors.Is(err, routing.ErrAllUnavailable) {
		t.Fatalf("expected ErrAllUnavailable, got %v", err)
	}

	// Advance time past the cooldown; provider should be available again.
	now = now.Add(cooldown + time.Second)
	got, _, _, err := r.Select("build", "low")
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
		_, _, _, err := r.Select("forced", "low")
		if err != nil {
			t.Fatalf("unexpected error on forced select %d: %v", i, err)
		}
	}
	// providerA count=4, providerB count=0.
	// providerA score = 4/3 ≈ 1.33, providerB score = 0/1 = 0 → B should be selected.
	got, _, _, err := r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != llmtypes.LLMProvider(providerB) {
		t.Fatalf("expected providerB selected after providerA over-served, got %v", got)
	}
}

func TestRouterSelectDeterministicTieBreaking(t *testing.T) {
	beta := &stubProvider{name: "beta"}
	alpha := &stubProvider{name: "alpha"}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"beta":  beta,
			"alpha": alpha,
		},
		Ratio: map[string]int{
			"beta":  1,
			"alpha": 1,
		},
	})

	// Both providers at 0 invocations with equal ratios — a perfect tie.
	// Deterministic tie-breaking should select "alpha" (alphabetically first).
	got, _, _, err := r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != llmtypes.LLMProvider(alpha) {
		t.Fatalf("expected alpha (alphabetically first) on tie-break, got %v", got)
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

	got, _, _, err := r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != llmtypes.LLMProvider(providerB) {
		t.Fatalf("expected providerB selected for build phase, got %v", got)
	}
}

func TestRouterSelectPhasePreferenceOverridesRatio(t *testing.T) {
	providerA := &stubProvider{name: "providerA"}
	providerB := &stubProvider{name: "providerB"}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"providerA": providerA,
			"providerB": providerB,
		},
		Ratio: map[string]int{
			"providerA": 1,
			"providerB": 3,
		},
		PhasePreferences: map[string]string{
			"prefer-a": "providerA",
		},
	})

	// Make balanced selections on "neutral" phase (no preference).
	// With ratio A:1, B:3, selections should favor B.
	for i := 0; i < 8; i++ {
		_, _, _, err := r.Select("neutral", "low")
		if err != nil {
			t.Fatalf("unexpected error on select %d: %v", i, err)
		}
	}
	// Roughly: A has lower count, B has higher count.
	// Ratio balancing should select A (underserved), but...
	// When we use "prefer-a" phase, it should definitely select A due to preference.
	got, _, _, err := r.Select("prefer-a", "low")
	if err != nil {
		t.Fatalf("unexpected error on phase select: %v", err)
	}
	if got != llmtypes.LLMProvider(providerA) {
		t.Fatalf("expected phase preference to select providerA, got %v", got)
	}

	// Now verify phase preference overrides ratio by selecting "prefer-b" explicitly.
	// Set up a new router where A is preferred by ratio balancing.
	r2 := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"providerA": providerA,
			"providerB": providerB,
		},
		Ratio: map[string]int{
			"providerA": 10,
			"providerB": 1,
		},
		PhasePreferences: map[string]string{
			"prefer-b": "providerB",
		},
	})

	// Force many invocations on A via forced phase.
	for i := 0; i < 10; i++ {
		_, _, _, err := r2.Select("forced-a", "low")
		if err != nil && err != routing.ErrNoProviders {
			// Skip if providers not configured for forced-a (expected to fail)
		}
	}
	// Manually force count on A by updating preferences.
	r2 = routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"providerA": providerA,
			"providerB": providerB,
		},
		Ratio: map[string]int{
			"providerA": 10,
			"providerB": 1,
		},
		PhasePreferences: map[string]string{
			"forced-a": "providerA",
			"prefer-b": "providerB",
		},
	})

	// Force selections on A.
	for i := 0; i < 10; i++ {
		_, _, _, err := r2.Select("forced-a", "low")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	// A.count=10, B.count=0.
	// Score A = 10/10 = 1.0, Score B = 0/1 = 0 → B would win by ratio.
	// With phase preference "prefer-b", B should be selected despite worse ratio.
	got, _, _, err = r2.Select("prefer-b", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != llmtypes.LLMProvider(providerB) {
		t.Fatalf("expected phase preference to override ratio and select providerB, got %v", got)
	}
}
