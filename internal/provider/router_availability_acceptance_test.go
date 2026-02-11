//go:build acceptance

package provider

import (
	"testing"
	"time"
)

// TestRouterAvailabilityMethods_AcceptanceCriteria1 verifies the acceptance criterion:
// "MarkUnavailable(name string) method records current time plus cooldown"
// Expected failure: MarkUnavailable method does not exist yet on Router
func TestRouterAvailabilityMethods_AcceptanceCriteria1(t *testing.T) {
	claudeProv := &mockProvider{name: "claude"}
	cooldown := 30 * time.Minute

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
		},
		unavailable: make(map[string]time.Time),
		cooldown:    cooldown,
		stateFn:     &mockStateFile{},
	}

	beforeCall := time.Now()
	r.MarkUnavailable("claude")
	afterCall := time.Now()

	// Verify that unavailable time was recorded
	unavailableUntil, exists := r.unavailable["claude"]
	if !exists {
		t.Fatal("MarkUnavailable did not record provider in unavailable map")
	}

	// The recorded time should be approximately now + cooldown
	expectedEarliest := beforeCall.Add(cooldown)
	expectedLatest := afterCall.Add(cooldown)

	if unavailableUntil.Before(expectedEarliest) {
		t.Errorf("MarkUnavailable recorded time %v, expected not before %v",
			unavailableUntil, expectedEarliest)
	}
	if unavailableUntil.After(expectedLatest) {
		t.Errorf("MarkUnavailable recorded time %v, expected not after %v",
			unavailableUntil, expectedLatest)
	}
}

// TestRouterAvailabilityMethods_AcceptanceCriteria2 verifies the acceptance criterion:
// "RecordInvocation(name string) increments count and persists to state via stateFn"
// Expected failure: RecordInvocation method does not exist yet on Router
func TestRouterAvailabilityMethods_AcceptanceCriteria2(t *testing.T) {
	stateFn := &mockStateFile{
		providerCounts: map[string]int{
			"claude": 5,
		},
	}

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
		},
		counts:  map[string]int{"claude": 5},
		stateFn: stateFn,
	}

	// Call RecordInvocation
	r.RecordInvocation("claude")

	// Verify internal count was incremented
	if r.counts["claude"] != 6 {
		t.Errorf("RecordInvocation resulted in local count %d, want 6", r.counts["claude"])
	}

	// Verify state file was updated
	if !stateFn.incrementCalled {
		t.Error("RecordInvocation did not call IncrementProviderCount on state file")
	}
	if stateFn.lastIncrementedProvider != "claude" {
		t.Errorf("IncrementProviderCount called with %q, want %q",
			stateFn.lastIncrementedProvider, "claude")
	}
	if stateFn.providerCounts["claude"] != 6 {
		t.Errorf("State file count is %d, want 6", stateFn.providerCounts["claude"])
	}
}

// TestRouterAvailabilityMethods_AcceptanceCriteria3 verifies the acceptance criterion:
// "NewSingleProviderRouter(p Provider) convenience constructor creates a minimal router
// with one provider, all preferences 'any', ratio 100%"
// Expected failure: NewSingleProviderRouter constructor does not exist yet
func TestRouterAvailabilityMethods_AcceptanceCriteria3(t *testing.T) {
	provider := &mockProvider{name: "claude"}

	// Create router using convenience constructor
	r := NewSingleProviderRouter(provider)

	// Verify router was created
	if r == nil {
		t.Fatal("NewSingleProviderRouter returned nil")
	}

	// Verify single provider
	if len(r.providers) != 1 {
		t.Errorf("Router has %d providers, want 1", len(r.providers))
	}
	if r.providers["claude"] != provider {
		t.Error("Router does not contain the provided provider")
	}

	// Verify ratio is 100%
	if r.ratio["claude"] != 100 {
		t.Errorf("Ratio for provider is %d%%, want 100%%", r.ratio["claude"])
	}

	// Verify preferences set to "any"
	if r.preferences["any"] != "any" {
		t.Errorf("Preferences[\"any\"] = %q, want %q", r.preferences["any"], "any")
	}

	// Verify empty maps initialized
	if r.counts == nil {
		t.Error("Counts map not initialized")
	}
	if r.unavailable == nil {
		t.Error("Unavailable map not initialized")
	}

	// Verify nil stateFn for backward compatibility
	if r.stateFn != nil {
		t.Error("StateFn should be nil for backward compatibility")
	}

	// Verify zero cooldown
	if r.cooldown != 0 {
		t.Errorf("Cooldown = %v, want 0", r.cooldown)
	}
}

// TestRouterMarkUnavailableIntegrationWithSelect verifies that MarkUnavailable
// causes Select to return a different provider (integration behavior).
// Expected failure: Integration between MarkUnavailable and Select does not work
func TestRouterMarkUnavailableIntegrationWithSelect(t *testing.T) {
	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	r := NewRouter(
		map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		map[string]string{
			"build": "claude", // Prefer claude
		},
		map[string]int{
			"claude": 60,
			"openai": 40,
		},
		30*time.Minute,
		&mockStateFile{},
	)

	// First select should return claude
	p1, _ := r.Select("build", TierMedium)
	if p1 == nil || p1.Name() != "claude" {
		t.Fatalf("Initial Select did not return claude as preferred, got: %v", p1)
	}

	// Mark claude unavailable
	r.MarkUnavailable("claude")

	// Second select should return openai (fallback)
	p2, _ := r.Select("build", TierMedium)
	if p2 == nil {
		t.Fatal("Select after MarkUnavailable returned nil")
	}
	if p2.Name() != "openai" {
		t.Errorf("Select after MarkUnavailable returned %q, want %q (fallback)",
			p2.Name(), "openai")
	}
}

// TestRouterRecordInvocationAffectsRatioBalancingInSelect verifies that
// RecordInvocation actually affects the counts used in ratio balancing.
// Expected failure: RecordInvocation does not affect Select's ratio balancing logic
func TestRouterRecordInvocationAffectsRatioBalancingInSelect(t *testing.T) {
	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	stateFn := &mockStateFile{
		providerCounts: make(map[string]int),
	}

	r := NewRouter(
		map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		map[string]string{
			"build": "any", // Use ratio balancing
		},
		map[string]int{
			"claude": 60, // Target 60%
			"openai": 40, // Target 40%
		},
		30*time.Minute,
		stateFn,
	)

	// Artificially skew counts using RecordInvocation
	// Give claude 90 invocations and openai 10 invocations
	for i := 0; i < 90; i++ {
		r.RecordInvocation("claude")
	}
	for i := 0; i < 10; i++ {
		r.RecordInvocation("openai")
	}

	// Current state: claude=90 (90%), openai=10 (10%)
	// Target: claude=60%, openai=40%
	// Gap: claude is 30% above target, openai is 30% below target
	// Next selection should strongly favor openai

	provider, _ := r.Select("build", TierMedium)
	if provider == nil {
		t.Fatal("Select returned nil provider")
	}

	// Select should choose openai to balance toward the target ratio
	if provider.Name() != "openai" {
		t.Errorf("Select with skewed ratio returned %q, want %q (balancing)",
			provider.Name(), "openai")
	}
}

// TestRouterMarkUnavailableAllProvidersReturnsNil verifies that when all
// providers are marked unavailable, Select returns nil provider.
// Expected failure: Select does not properly handle all-unavailable case
func TestRouterMarkUnavailableAllProvidersReturnsNil(t *testing.T) {
	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	r := NewRouter(
		map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		map[string]string{
			"build": "any",
		},
		map[string]int{
			"claude": 50,
			"openai": 50,
		},
		30*time.Minute,
		&mockStateFile{},
	)

	// Mark both providers unavailable
	r.MarkUnavailable("claude")
	r.MarkUnavailable("openai")

	// Select should return nil
	provider, modelName := r.Select("build", TierMedium)
	if provider != nil {
		t.Errorf("Select with all providers unavailable returned %q, want nil",
			provider.Name())
	}
	if modelName != "" {
		t.Errorf("Select with all providers unavailable returned model %q, want empty",
			modelName)
	}
}

// TestNewSingleProviderRouterBackwardCompatibility verifies that the
// NewSingleProviderRouter constructor creates a functional router that works
// without state persistence (backward compatibility mode).
// Expected failure: NewSingleProviderRouter does not properly configure for backward compatibility
func TestNewSingleProviderRouterBackwardCompatibility(t *testing.T) {
	provider := &mockProvider{name: "claude"}

	// Create single-provider router (backward compatibility mode)
	r := NewSingleProviderRouter(provider)

	// Should be able to select successfully
	selectedProvider, _ := r.Select("build", TierMedium)
	if selectedProvider == nil {
		t.Fatal("Select on single-provider router returned nil")
	}
	if selectedProvider.Name() != "claude" {
		t.Errorf("Select returned %q, want %q", selectedProvider.Name(), "claude")
	}

	// Should handle RecordInvocation without state file (no panic)
	r.RecordInvocation("claude")
	if r.counts["claude"] != 2 { // 1 from Select, 1 from RecordInvocation
		t.Errorf("Count after RecordInvocation = %d, want 2", r.counts["claude"])
	}

	// Should handle MarkUnavailable without state file (no panic)
	r.MarkUnavailable("claude")
	if _, exists := r.unavailable["claude"]; !exists {
		t.Error("MarkUnavailable should record locally even without state file")
	}
}
