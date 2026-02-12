package provider

import (
	"testing"
	"time"
)

// TestSelectCrossReturnsOppositeProvider verifies that SelectCross returns the first
// available provider whose name differs from the buildProvider argument.
// Expected failure: SelectCross method does not exist on Router yet
func TestSelectCrossReturnsOppositeProvider(t *testing.T) {
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
		name             string
		buildProvider    string
		tier             string
		wantProvider     string
		wantModel        string
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
			// Expected failure: SelectCross method does not exist on Router yet
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
// Expected failure: SelectCross method does not exist on Router yet
func TestSelectCrossFallsBackToBuildProviderWhenNoCrossAvailable(t *testing.T) {
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
			r := &Router{
				providers:   tt.providers,
				preferences: map[string]string{"review": "cross"},
				ratio:       map[string]int{"claude": 100},
				counts:      map[string]int{},
				unavailable: tt.unavailable,
				cooldown:    30 * time.Minute,
				stateFn:     &mockStateFile{},
			}

			// Expected failure: SelectCross method does not exist on Router yet
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
// Expected failure: SelectCross method does not exist on Router yet
func TestSelectCrossIncrementsCount(t *testing.T) {
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

	// Expected failure: SelectCross method does not exist on Router yet
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
// Expected failure: SelectCross method does not exist on Router yet
func TestSelectCrossWithThreeProviders(t *testing.T) {
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

	// Expected failure: SelectCross method does not exist on Router yet
	p, _ := r.SelectCross("claude", TierMedium)
	if p == nil {
		t.Fatal("SelectCross() returned nil provider")
	}
	if p.Name() == "claude" {
		t.Errorf("SelectCross(\"claude\") should return a non-claude provider, got %q", p.Name())
	}
}
