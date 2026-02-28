package provider

import (
	"testing"
	"time"
)

// TestSelectCrossReturnsOppositeProvider verifies that SelectCross returns the first
// available provider whose name differs from the buildProvider argument.
func TestSelectCrossReturnsOppositeProvider(t *testing.T) {
	t.Parallel()
	claudeProv := &mockProviderWithModels{
		name: "claude",
		models: map[string]string{
			TierHigh:   "opus",
			TierMedium: "sonnet",
			TierLow:    "haiku",
		},
	}
	openaiProv := &mockProviderWithModels{
		name: "openai",
		models: map[string]string{
			TierHigh:   "o3",
			TierMedium: "gpt-4o",
			TierLow:    "gpt-4o-mini",
		},
	}

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		preferences: map[string]string{
			"review": "cross",
		},
		ratio:       map[string]int{"claude": 50, "openai": 50},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	tests := []struct {
		name          string
		buildProvider string
		tier          string
		wantProvider  string
		wantModel     string
	}{
		{
			name:          "claude build selects openai for cross-review",
			buildProvider: "claude",
			tier:          TierMedium,
			wantProvider:  "openai",
			wantModel:     "gpt-4o",
		},
		{
			name:          "openai build selects claude for cross-review",
			buildProvider: "openai",
			tier:          TierMedium,
			wantProvider:  "claude",
			wantModel:     "sonnet",
		},
		{
			name:          "cross-review respects tier for model selection",
			buildProvider: "claude",
			tier:          TierHigh,
			wantProvider:  "openai",
			wantModel:     "o3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, modelName := r.SelectCross(tt.buildProvider, tt.tier)
			if p == nil {
				t.Fatal("SelectCross() returned nil provider")
			}
			if p.Name() != tt.wantProvider {
				t.Errorf("SelectCross(%q, %q) returned provider %q, want %q",
					tt.buildProvider, tt.tier, p.Name(), tt.wantProvider)
			}
			if modelName != tt.wantModel {
				t.Errorf("SelectCross(%q, %q) returned model %q, want %q",
					tt.buildProvider, tt.tier, modelName, tt.wantModel)
			}
		})
	}
}

// TestSelectCrossFallsBackToBuildProviderWhenNoCrossAvailable verifies that
// SelectCross falls back to the buildProvider itself when no other provider
// is available (either because there's only one provider or others are unavailable).
func TestSelectCrossFallsBackToBuildProviderWhenNoCrossAvailable(t *testing.T) {
	t.Parallel()
	claudeProv := &mockProviderWithModels{
		name: "claude",
		models: map[string]string{
			TierHigh:   "opus",
			TierMedium: "sonnet",
			TierLow:    "haiku",
		},
	}

	tests := []struct {
		name          string
		providers     map[string]Provider
		unavailable   map[string]time.Time
		buildProvider string
		wantProvider  string
	}{
		{
			name: "single provider falls back to itself",
			providers: map[string]Provider{
				"claude": claudeProv,
			},
			unavailable:   map[string]time.Time{},
			buildProvider: "claude",
			wantProvider:  "claude",
		},
		{
			name: "cross provider unavailable falls back to build provider",
			providers: map[string]Provider{
				"claude": claudeProv,
				"openai": &mockProviderWithModels{
					name: "openai",
					models: map[string]string{
						TierMedium: "gpt-4o",
					},
				},
			},
			unavailable: map[string]time.Time{
				"openai": time.Now().Add(1 * time.Hour),
			},
			buildProvider: "claude",
			wantProvider:  "claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &Router{
				providers:   tt.providers,
				preferences: map[string]string{"review": "cross"},
				ratio:       map[string]int{"claude": 100},
				counts:      map[string]int{},
				unavailable: tt.unavailable,
				cooldown:    30 * time.Minute,
				stateFn:     &mockStateFile{},
			}

			p, _ := r.SelectCross(tt.buildProvider, TierMedium)
			if p == nil {
				t.Fatal("SelectCross() returned nil provider")
			}
			if p.Name() != tt.wantProvider {
				t.Errorf("SelectCross(%q) returned provider %q, want %q (fallback to build provider)",
					tt.buildProvider, p.Name(), tt.wantProvider)
			}
		})
	}
}

// TestSelectCrossIncrementsCount verifies that SelectCross increments the
// invocation count for the selected cross-provider, following the same
// pattern as Select().
func TestSelectCrossIncrementsCount(t *testing.T) {
	t.Parallel()
	stateFn := &mockStateFile{
		providerCounts: map[string]int{},
	}

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProviderWithModels{
				name:   "claude",
				models: map[string]string{TierMedium: "sonnet"},
			},
			"openai": &mockProviderWithModels{
				name:   "openai",
				models: map[string]string{TierMedium: "gpt-4o"},
			},
		},
		preferences: map[string]string{"review": "cross"},
		ratio:       map[string]int{"claude": 50, "openai": 50},
		counts:      map[string]int{"claude": 0, "openai": 0},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     stateFn,
	}

	p, _ := r.SelectCross("claude", TierMedium)
	if p == nil {
		t.Fatal("SelectCross() returned nil provider")
	}

	// Should have selected openai and incremented its count
	if r.counts["openai"] != 1 {
		t.Errorf("counts[\"openai\"] = %d after SelectCross, want 1", r.counts["openai"])
	}
	if r.counts["claude"] != 0 {
		t.Errorf("counts[\"claude\"] = %d after SelectCross, want 0 (should not be incremented)", r.counts["claude"])
	}
}

// TestSelectCrossWithThreeProviders verifies that SelectCross picks any
// available provider that differs from the buildProvider when more than
// two providers are configured.
func TestSelectCrossWithThreeProviders(t *testing.T) {
	t.Parallel()
	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProviderWithModels{
				name:   "claude",
				models: map[string]string{TierMedium: "sonnet"},
			},
			"openai": &mockProviderWithModels{
				name:   "openai",
				models: map[string]string{TierMedium: "gpt-4o"},
			},
			"gemini": &mockProviderWithModels{
				name:   "gemini",
				models: map[string]string{TierMedium: "gemini-pro"},
			},
		},
		preferences: map[string]string{"review": "cross"},
		ratio:       map[string]int{"claude": 40, "openai": 30, "gemini": 30},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	p, _ := r.SelectCross("claude", TierMedium)
	if p == nil {
		t.Fatal("SelectCross() returned nil provider")
	}
	if p.Name() == "claude" {
		t.Errorf("SelectCross(\"claude\") should return a non-claude provider, got %q", p.Name())
	}
}

func TestSelectCrossThreeProviderPermutations(t *testing.T) {
	t.Parallel()

	providerModels := map[string]string{
		TierMedium: "cross-model",
	}

	tests := []struct {
		name          string
		buildProvider string
		allowed       []string
	}{
		{
			name:          "gemini build provider selects claude or openai",
			buildProvider: "gemini",
			allowed:       []string{"claude", "openai"},
		},
		{
			name:          "claude build provider selects gemini or openai",
			buildProvider: "claude",
			allowed:       []string{"gemini", "openai"},
		},
		{
			name:          "openai build provider selects gemini or claude",
			buildProvider: "openai",
			allowed:       []string{"gemini", "claude"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &Router{
				providers: map[string]Provider{
					"claude": &mockProviderWithModels{name: "claude", models: providerModels},
					"openai": &mockProviderWithModels{name: "openai", models: providerModels},
					"gemini": &mockProviderWithModels{name: "gemini", models: providerModels},
				},
				preferences: map[string]string{"review": "cross"},
				ratio: map[string]int{
					"claude": 33,
					"openai": 33,
					"gemini": 34,
				},
				counts:      map[string]int{},
				unavailable: map[string]time.Time{},
				cooldown:    30 * time.Minute,
				stateFn:     &mockStateFile{},
			}

			p, _ := r.SelectCross(tt.buildProvider, TierMedium)
			if p == nil {
				t.Fatal("SelectCross() returned nil provider")
			}
			if got := p.Name(); got == tt.buildProvider {
				t.Fatalf("SelectCross(%q) selected build provider %q, want cross provider", tt.buildProvider, got)
			}
			assertAllowedCrossProvider(t, p.Name(), tt.allowed)
		})
	}
}

// TestSelectCrossReturnsNilWhenAllProvidersUnavailable ensures SelectCross
// returns nil when every configured provider is marked as unavailable.
func TestSelectCrossReturnsNilWhenAllProvidersUnavailable(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(1 * time.Hour)

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProviderWithModels{name: "claude", models: map[string]string{TierMedium: "sonnet"}},
			"openai": &mockProviderWithModels{name: "openai", models: map[string]string{TierMedium: "gpt-4o"}},
		},
		preferences: map[string]string{"review": "cross"},
		ratio:       map[string]int{"claude": 50, "openai": 50},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{"claude": future, "openai": future},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	p, model := r.SelectCross("claude", TierMedium)
	if p != nil {
		t.Fatalf("SelectCross() should return nil when no provider is available, got %q", p.Name())
	}
	if model != "" {
		t.Errorf("SelectCross() returned model %q when no providers available, want empty", model)
	}
}

// assertAllowedCrossProvider checks that a selected provider is a member of
// the allowed set. This helper is resilient to non-deterministic map iteration
// because it only checks set membership regardless of order.
func assertAllowedCrossProvider(t *testing.T, selected string, allowed []string) {
	for _, a := range allowed {
		if a == selected {
			return
		}
	}
	t.Errorf("Selected provider %q is not in allowed set %v", selected, allowed)
}

// TestSelectCrossConcurrentAccess verifies that Router can handle concurrent
// calls to SelectCross without data races when run with -race detector.
func TestSelectCrossConcurrentAccess(t *testing.T) {
	t.Parallel()
	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProviderWithModels{
				name:   "claude",
				models: map[string]string{TierMedium: "sonnet"},
			},
			"openai": &mockProviderWithModels{
				name:   "openai",
				models: map[string]string{TierMedium: "gpt-4o"},
			},
		},
		preferences: map[string]string{"review": "cross"},
		ratio:       map[string]int{"claude": 50, "openai": 50},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	// Launch concurrent calls to SelectCross
	const numConcurrent = 10
	done := make(chan struct{}, numConcurrent)
	for i := 0; i < numConcurrent; i++ {
		go func(iteration int) {
			defer func() { done <- struct{}{} }()
			buildProvider := "claude"
			if iteration%2 == 0 {
				buildProvider = "openai"
			}
			p, modelName := r.SelectCross(buildProvider, TierMedium)
			if p == nil {
				t.Errorf("SelectCross() returned nil provider on iteration %d", iteration)
				return
			}
			if modelName == "" {
				t.Errorf("SelectCross() returned empty model name on iteration %d", iteration)
			}
		}(i)
	}

	// Wait for all concurrent calls to complete
	for i := 0; i < numConcurrent; i++ {
		<-done
	}
}

