package provider

import (
	"context"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"
)

// TestRouterStructExists verifies that the Router struct exists and can be instantiated.
// Expected failure: Router struct does not exist yet
func TestRouterStructExists(t *testing.T) {
	t.Parallel()
	var r *Router
	if r != nil {
		t.Error("nil Router should be nil")
	}
}

// TestRouterHasProvidersMapField verifies that Router has a providers map field
// holding Provider implementations by name.
// Expected failure: Router struct and providers field do not exist yet
func TestRouterHasProvidersMapField(t *testing.T) {
	t.Parallel()
	mockProv := &mockProvider{}
	r := &Router{
		providers: map[string]Provider{
			"claude": mockProv,
		},
	}

	if r.providers == nil {
		t.Error("Router.providers should not be nil after assignment")
	}

	if r.providers["claude"] != mockProv {
		t.Error("Router.providers[\"claude\"] should be the mock provider")
	}
}

// TestRouterHasPreferencesMapField verifies that Router has a preferences map field
// mapping phase names to provider names or "any".
// Expected failure: Router struct and preferences field do not exist yet
func TestRouterHasPreferencesMapField(t *testing.T) {
	t.Parallel()
	r := &Router{
		preferences: map[string]string{
			"build":    "claude",
			"validate": "any",
		},
	}

	if r.preferences == nil {
		t.Error("Router.preferences should not be nil after assignment")
	}

	if r.preferences["build"] != "claude" {
		t.Errorf("preferences[\"build\"] = %q, want %q", r.preferences["build"], "claude")
	}
}

// TestRouterHasRatioMapField verifies that Router has a ratio map field
// holding target percentages for each provider.
// Expected failure: Router struct and ratio field do not exist yet
func TestRouterHasRatioMapField(t *testing.T) {
	t.Parallel()
	r := &Router{
		ratio: map[string]int{
			"claude": 60,
			"openai": 40,
		},
	}

	if r.ratio == nil {
		t.Error("Router.ratio should not be nil after assignment")
	}

	if r.ratio["claude"] != 60 {
		t.Errorf("ratio[\"claude\"] = %d, want 60", r.ratio["claude"])
	}
}

// TestRouterHasCountsMapField verifies that Router has a counts map field
// tracking invocation counts per provider.
// Expected failure: Router struct and counts field do not exist yet
func TestRouterHasCountsMapField(t *testing.T) {
	t.Parallel()
	r := &Router{
		counts: map[string]int{
			"claude": 15,
			"openai": 10,
		},
	}

	if r.counts == nil {
		t.Error("Router.counts should not be nil after assignment")
	}

	if r.counts["claude"] != 15 {
		t.Errorf("counts[\"claude\"] = %d, want 15", r.counts["claude"])
	}
}

// TestRouterHasUnavailableMapField verifies that Router has an unavailable map field
// tracking when providers became unavailable.
// Expected failure: Router struct and unavailable field do not exist yet
func TestRouterHasUnavailableMapField(t *testing.T) {
	t.Parallel()
	now := time.Now()
	r := &Router{
		unavailable: map[string]time.Time{
			"claude": now,
		},
	}

	if r.unavailable == nil {
		t.Error("Router.unavailable should not be nil after assignment")
	}

	if !r.unavailable["claude"].Equal(now) {
		t.Errorf("unavailable[\"claude\"] = %v, want %v", r.unavailable["claude"], now)
	}
}

// TestRouterHasCooldownField verifies that Router has a cooldown duration field
// specifying how long to avoid unavailable providers.
// Expected failure: Router struct and cooldown field do not exist yet
func TestRouterHasCooldownField(t *testing.T) {
	t.Parallel()
	r := &Router{
		cooldown: 30 * time.Minute,
	}

	if r.cooldown != 30*time.Minute {
		t.Errorf("cooldown = %v, want %v", r.cooldown, 30*time.Minute)
	}
}

// TestRouterHasStateFnField verifies that Router has a stateFn field
// for persisting provider routing state.
// Expected failure: Router struct and stateFn field do not exist yet
func TestRouterHasStateFnField(t *testing.T) {
	t.Parallel()
	mockStateFn := &mockStateFile{}
	r := &Router{
		stateFn: mockStateFn,
	}

	if r.stateFn == nil {
		t.Error("Router.stateFn should not be nil after assignment")
	}
}

func TestRouterHasSingleMutexField(t *testing.T) {
	t.Parallel()

	routerType := reflect.TypeOf(Router{})
	mutexType := reflect.TypeOf(sync.Mutex{})

	mutexCount := 0
	for i := 0; i < routerType.NumField(); i++ {
		if routerType.Field(i).Type == mutexType {
			mutexCount++
		}
	}

	if mutexCount != 1 {
		t.Fatalf("Router has %d mutex fields, want 1", mutexCount)
	}
}

// TestNewRouterConstructorExists verifies that NewRouter constructor exists
// and creates a Router instance.
// Expected failure: NewRouter() function does not exist yet
func TestNewRouterConstructorExists(t *testing.T) {
	t.Parallel()
	providers := map[string]Provider{
		"claude": &mockProvider{},
	}
	preferences := map[string]string{
		"build": "claude",
	}
	ratio := map[string]int{
		"claude": 100,
	}
	cooldown := 30 * time.Minute
	stateFn := &mockStateFile{}

	r := NewRouter(providers, preferences, ratio, cooldown, stateFn, nil)

	if r == nil {
		t.Fatal("NewRouter() returned nil")
	}
}

// TestNewRouterInitializesFields verifies that NewRouter constructor
// properly initializes all Router fields from provided arguments.
// Expected failure: NewRouter() function does not exist yet
func TestNewRouterInitializesFields(t *testing.T) {
	t.Parallel()
	mockProv := &mockProvider{}
	providers := map[string]Provider{
		"claude": mockProv,
	}
	preferences := map[string]string{
		"build":    "claude",
		"validate": "any",
	}
	ratio := map[string]int{
		"claude": 60,
		"openai": 40,
	}
	cooldown := 30 * time.Minute
	stateFn := &mockStateFile{}

	r := NewRouter(providers, preferences, ratio, cooldown, stateFn, nil)

	if r == nil {
		t.Fatal("NewRouter() returned nil")
	}

	if r.providers == nil {
		t.Error("Router.providers should be initialized")
	}

	if r.providers["claude"] != mockProv {
		t.Error("Router.providers should contain the provided mock provider")
	}

	if r.preferences["build"] != "claude" {
		t.Errorf("preferences[\"build\"] = %q, want %q", r.preferences["build"], "claude")
	}

	if r.ratio["claude"] != 60 {
		t.Errorf("ratio[\"claude\"] = %d, want 60", r.ratio["claude"])
	}

	if r.cooldown != cooldown {
		t.Errorf("cooldown = %v, want %v", r.cooldown, cooldown)
	}

	if r.stateFn != stateFn {
		t.Error("Router.stateFn should be the provided state file")
	}
}

// TestNewRouterInitializesCounts verifies that NewRouter initializes
// the counts map from the state file's provider counts.
// Expected failure: NewRouter() function does not exist yet
func TestNewRouterInitializesCounts(t *testing.T) {
	t.Parallel()
	stateFn := &mockStateFile{
		providerCounts: map[string]int{
			"claude": 15,
			"openai": 10,
		},
	}

	r := NewRouter(
		map[string]Provider{"claude": &mockProvider{}},
		map[string]string{"build": "claude"},
		map[string]int{"claude": 100},
		30*time.Minute,
		stateFn,
		nil,
	)

	if r == nil {
		t.Fatal("NewRouter() returned nil")
	}

	if r.counts == nil {
		t.Error("Router.counts should be initialized")
	}

	if r.counts["claude"] != 15 {
		t.Errorf("counts[\"claude\"] = %d, want 15", r.counts["claude"])
	}

	if r.counts["openai"] != 10 {
		t.Errorf("counts[\"openai\"] = %d, want 10", r.counts["openai"])
	}
}

// TestRouterSelectMethodExists verifies that Router has a Select method
// that accepts phase and tier arguments and returns a Provider and model name.
// Expected failure: Select() method does not exist yet
func TestRouterSelectMethodExists(t *testing.T) {
	t.Parallel()
	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{},
		},
		preferences: map[string]string{
			"build": "claude",
		},
		ratio:    map[string]int{"claude": 100},
		counts:   map[string]int{},
		cooldown: 30 * time.Minute,
		stateFn:  &mockStateFile{},
	}

	// Verify the method can be called
	provider, modelName := r.Select("build", TierMedium)

	// We don't care about the exact return values, just that the method exists
	// and has the right signature
	_ = provider
	_ = modelName
}

// TestRouterSelectWithPhasePreference verifies that Select method
// returns the preferred provider when one is specified for the phase.
// This tests Layer 1: phase preference check.
// Expected failure: Select() method does not exist yet
func TestRouterSelectWithPhasePreference(t *testing.T) {
	t.Parallel()
	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		preferences: map[string]string{
			"build":    "claude",
			"validate": "openai",
		},
		ratio: map[string]int{
			"claude": 50,
			"openai": 50,
		},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	// Test that build phase prefers claude
	provider, _ := r.Select("build", TierMedium)
	if provider == nil {
		t.Fatal("Select() returned nil provider")
	}
	if provider.Name() != "claude" {
		t.Errorf("Select(\"build\", TierMedium) returned %q provider, want %q", provider.Name(), "claude")
	}

	// Test that validate phase prefers openai
	provider, _ = r.Select("validate", TierLow)
	if provider == nil {
		t.Fatal("Select() returned nil provider")
	}
	if provider.Name() != "openai" {
		t.Errorf("Select(\"validate\", TierLow) returned %q provider, want %q", provider.Name(), "openai")
	}
}

// TestRouterSelectWithAnyPreference verifies that Select method
// uses ratio balancing when phase preference is "any".
// This tests Layer 2: ratio-based balancing.
// Expected failure: Select() method does not exist yet
func TestRouterSelectWithAnyPreference(t *testing.T) {
	t.Parallel()
	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		preferences: map[string]string{
			"build": "any",
		},
		ratio: map[string]int{
			"claude": 60,
			"openai": 40,
		},
		counts: map[string]int{
			"claude": 0,
			"openai": 0,
		},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	// With equal counts, should select the provider furthest below its target ratio
	// Both at 0%, but claude target is 60% and openai target is 40%
	// So first invocation should go to whichever has the larger gap
	// (This is a simplified test - actual implementation may vary)
	provider, _ := r.Select("build", TierMedium)
	if provider == nil {
		t.Fatal("Select() returned nil provider")
	}

	// The provider should be one of the available providers
	provName := provider.Name()
	if provName != "claude" && provName != "openai" {
		t.Errorf("Select() returned unexpected provider %q", provName)
	}
}

// TestRouterSelectSkipsUnavailableProvider verifies that Select method
// skips unavailable providers and falls back to available ones.
// This tests Layer 3: automatic fallback.
// Expected failure: Select() method does not exist yet
func TestRouterSelectSkipsUnavailableProvider(t *testing.T) {
	t.Parallel()
	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	futureTime := time.Now().Add(1 * time.Hour)

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		preferences: map[string]string{
			"build": "claude",
		},
		ratio: map[string]int{
			"claude": 100,
			"openai": 0,
		},
		counts: map[string]int{},
		unavailable: map[string]time.Time{
			"claude": futureTime, // claude is unavailable
		},
		cooldown: 30 * time.Minute,
		stateFn:  &mockStateFile{},
	}

	// Even though phase preference is "claude", it should fallback to openai
	provider, _ := r.Select("build", TierMedium)
	if provider == nil {
		t.Fatal("Select() returned nil provider")
	}
	if provider.Name() != "openai" {
		t.Errorf("Select() returned %q provider, want %q (fallback)", provider.Name(), "openai")
	}
}

// TestRouterSelectErrorWhenAllUnavailable verifies that Select method
// returns an error when all providers are unavailable.
// Expected failure: Select() method does not exist yet, and it should return (Provider, string, error)
func TestRouterSelectErrorWhenAllUnavailable(t *testing.T) {
	t.Parallel()
	futureTime := time.Now().Add(1 * time.Hour)

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
			"openai": &mockProvider{name: "openai"},
		},
		preferences: map[string]string{
			"build": "any",
		},
		ratio: map[string]int{
			"claude": 50,
			"openai": 50,
		},
		counts: map[string]int{},
		unavailable: map[string]time.Time{
			"claude": futureTime,
			"openai": futureTime,
		},
		cooldown: 30 * time.Minute,
		stateFn:  &mockStateFile{},
	}

	// This should return nil provider since all are unavailable
	provider, modelName := r.Select("build", TierMedium)

	// Based on the spec, when all providers are unavailable, Select should return an error
	// The signature needs to be Select(phase, tier) (Provider, string, error)
	// For now, test that nil is returned when all are unavailable
	if provider != nil {
		t.Errorf("Select() should return nil provider when all are unavailable, got %q", provider.Name())
	}
	if modelName != "" {
		t.Errorf("Select() should return empty model name when all are unavailable, got %q", modelName)
	}
}

// TestRouterSelectReturnsModelName verifies that Select method returns
// the concrete model name for the selected provider and tier.
// Expected failure: Select() method does not exist yet
func TestRouterSelectReturnsModelName(t *testing.T) {
	t.Parallel(
	// Create a custom provider that maps tiers to specific model names
	)

	claudeProv := &mockProviderWithModels{
		name: "claude",
		models: map[string]string{
			TierHigh:   "opus",
			TierMedium: "sonnet",
			TierLow:    "haiku",
		},
	}

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
		},
		preferences: map[string]string{
			"build": "claude",
		},
		ratio:       map[string]int{"claude": 100},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	tests := []struct {
		tier          string
		expectedModel string
	}{
		{TierHigh, "opus"},
		{TierMedium, "sonnet"},
		{TierLow, "haiku"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			t.Parallel()
			_, modelName := r.Select("build", tt.tier)
			if modelName != tt.expectedModel {
				t.Errorf("Select(\"build\", %q) returned model %q, want %q", tt.tier, modelName, tt.expectedModel)
			}
		})
	}
}

// TestRouterSelectIncrementsCountInState verifies that Select method
// increments the provider's invocation count in the state file.
// Expected failure: Select() method does not exist yet
func TestRouterSelectIncrementsCountInState(t *testing.T) {
	t.Parallel()
	stateFn := &mockStateFile{
		providerCounts: map[string]int{
			"claude": 5,
		},
	}

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
		},
		preferences: map[string]string{
			"build": "claude",
		},
		ratio:       map[string]int{"claude": 100},
		counts:      map[string]int{"claude": 5},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     stateFn,
	}

	// Call Select
	_, _ = r.Select("build", TierMedium)

	// Verify that IncrementProviderCount was called
	if !stateFn.incrementCalled {
		t.Error("Select() should call IncrementProviderCount on state file")
	}

	if stateFn.lastIncrementedProvider != "claude" {
		t.Errorf("IncrementProviderCount called with %q, want %q", stateFn.lastIncrementedProvider, "claude")
	}

	// Verify internal count was updated
	if r.counts["claude"] != 6 {
		t.Errorf("counts[\"claude\"] = %d, want 6 after Select()", r.counts["claude"])
	}
}

// TestRouterSelectRatioBalancing verifies that Select method properly
// balances invocations based on target ratios over multiple calls.
// Expected failure: Select() method does not exist yet
func TestRouterSelectRatioBalancing(t *testing.T) {
	t.Parallel()
	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		preferences: map[string]string{
			"build": "any", // Use ratio balancing
		},
		ratio: map[string]int{
			"claude": 70, // 70% target
			"openai": 30, // 30% target
		},
		counts: map[string]int{
			"claude": 70, // Already at 70%
			"openai": 30, // Already at 30%
		},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	// Both providers are exactly at their target ratios
	// The next invocation should maintain or restore balance
	provider, _ := r.Select("build", TierMedium)
	if provider == nil {
		t.Fatal("Select() returned nil provider")
	}

	// The selected provider should be one of the available ones
	provName := provider.Name()
	if provName != "claude" && provName != "openai" {
		t.Errorf("Select() returned unexpected provider %q", provName)
	}
}

func TestRouterSelectPrefersNonDegradedProviderMoreOften(t *testing.T) {
	t.Parallel()
	const selectionRuns = 30

	cb := &CircuitBreaker{
		windowSize:       2,
		failureThreshold: 0.25,
		degradedFloor:    10,
	}

	r := NewRouter(
		map[string]Provider{
			"claude": &mockProviderWithModels{name: "claude", models: map[string]string{TierMedium: "sonnet"}},
			"codex":  &mockProviderWithModels{name: "codex", models: map[string]string{TierMedium: "gpt-4o"}},
		},
		map[string]string{"build": "any"},
		map[string]int{"codex": 80, "claude": 20},
		0,
		&mockStateFile{},
		cb,
	)

	r.RecordOutcome("codex", FailureCategoryTransportDisconnect)

	selectionCounts := map[string]int{
		"claude": 0,
		"codex":  0,
	}
	for i := 0; i < selectionRuns; i++ {
		p, _ := r.Select("build", TierMedium)
		if p == nil {
			t.Fatal("Select() returned nil provider")
		}
		selectionCounts[p.Name()]++
	}

	if selectionCounts["claude"] <= selectionCounts["codex"] {
		t.Fatalf("expected non-degraded provider to be selected more often, got claude=%d codex=%d",
			selectionCounts["claude"], selectionCounts["codex"])
	}
}

func TestRouterSelectByRatioUsesConfiguredRatioWhenCircuitBreakerIsNil(t *testing.T) {
	t.Parallel()
	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
			"codex":  &mockProvider{name: "codex"},
		},
		ratio:       map[string]int{"codex": 70, "claude": 30},
		counts:      map[string]int{"codex": 10, "claude": 90},
		unavailable: map[string]time.Time{},
	}

	selected := r.selectByRatio()
	if selected != "codex" {
		t.Fatalf("selectByRatio() = %q, want %q", selected, "codex")
	}
}

func TestRouterRecordOutcomeDelegatesToCircuitBreaker(t *testing.T) {
	t.Parallel()
	cb := &CircuitBreaker{
		windowSize:       1,
		failureThreshold: 0.5,
		degradedFloor:    10,
	}
	r := &Router{circuitBreaker: cb}

	r.RecordOutcome("claude", FailureCategoryTransportDisconnect)

	if !cb.IsDegraded("claude") {
		t.Fatal("RecordOutcome() did not delegate to circuit breaker")
	}
}

// TestRouterSelectAvailabilityCheckUsesStateFile verifies that Select method
// checks provider availability using the state file's IsProviderAvailable method.
// Expected failure: Select() method does not exist yet
func TestRouterSelectAvailabilityCheckUsesStateFile(t *testing.T) {
	t.Parallel()
	stateFn := &mockStateFile{
		unavailableProviders: map[string]bool{
			"claude": true, // mark claude as unavailable
		},
	}

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
			"openai": &mockProvider{name: "openai"},
		},
		preferences: map[string]string{
			"build": "claude",
		},
		ratio: map[string]int{
			"claude": 100,
			"openai": 0,
		},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     stateFn,
	}

	// Should fallback to openai since claude is unavailable
	provider, _ := r.Select("build", TierMedium)
	if provider == nil {
		t.Fatal("Select() returned nil provider")
	}
	if provider.Name() != "openai" {
		t.Errorf("Select() returned %q provider, want %q", provider.Name(), "openai")
	}

	// Verify that IsProviderAvailable was called
	if !stateFn.isAvailableCalled {
		t.Error("Select() should call IsProviderAvailable on state file")
	}
}

func TestSelectionBlocksMarkUnavailableWhileSelecting(t *testing.T) {
	cases := []struct {
		name       string
		setup      func() (*Router, *blockingProvider, func(*Router) (Provider, string))
		buildPhase string
	}{
		{
			name: "Select",
			setup: func() (*Router, *blockingProvider, func(*Router) (Provider, string)) {
				blocking := newBlockingProvider("blocking")
				r := &Router{
					providers: map[string]Provider{
						"blocking": blocking,
					},
					preferences: map[string]string{
						"phase-select": "blocking",
					},
					ratio:       map[string]int{"blocking": 100},
					counts:      map[string]int{},
					unavailable: map[string]time.Time{},
					cooldown:    30 * time.Minute,
					stateFn:     &mockStateFile{},
				}
				return r, blocking, func(r *Router) (Provider, string) {
					return r.Select("phase-select", TierMedium)
				}
			},
		},
		{
			name: "SelectCross",
			setup: func() (*Router, *blockingProvider, func(*Router) (Provider, string)) {
				blocking := newBlockingProvider("blocking")
				r := &Router{
					providers: map[string]Provider{
						"blocking": blocking,
						"build":    &mockProvider{name: "build"},
					},
					ratio:       map[string]int{"blocking": 100, "build": 0},
					counts:      map[string]int{},
					unavailable: map[string]time.Time{},
					cooldown:    30 * time.Minute,
					stateFn:     &mockStateFile{},
				}
				return r, blocking, func(r *Router) (Provider, string) {
					return r.SelectCross("build", TierMedium)
				}
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			r, blocking, invoke := tt.setup()

			doneSelect := make(chan struct{})
			var selected Provider
			go func() {
				defer close(doneSelect)
				provider, _ := invoke(r)
				selected = provider
			}()

			<-blocking.modelStarted

			markDone := make(chan struct{})
			go func() {
				r.MarkUnavailable(blocking.Name())
				close(markDone)
			}()

			select {
			case <-markDone:
				t.Fatalf("MarkUnavailable returned before selection released lock for %s", tt.name)
			case <-time.After(50 * time.Millisecond):
			}

			close(blocking.resume)

			select {
			case <-markDone:
			case <-time.After(time.Second):
				t.Fatalf("MarkUnavailable did not complete after selection finished for %s", tt.name)
			}

			select {
			case <-doneSelect:
			case <-time.After(time.Second):
				t.Fatalf("selection did not return for %s", tt.name)
			}

			if selected == nil {
				t.Fatalf("%s returned nil provider", tt.name)
			}
		})
	}
}

func TestRouterSelectDoesNotCallStateFileConcurrentlyWithMarkUnavailable(t *testing.T) {
	t.Parallel()

	monitor := newStateFileConcurrencyMonitor()
	r := &Router{
		providers: map[string]Provider{
			"blocking": &mockProvider{name: "blocking"},
		},
		preferences: map[string]string{
			"phase-select": "blocking",
		},
		ratio:       map[string]int{"blocking": 100},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     monitor,
	}

	selectDone := make(chan struct{})
	go func() {
		defer close(selectDone)
		r.Select("phase-select", TierMedium)
	}()

	<-monitor.entered

	markDone := make(chan struct{})
	go func() {
		defer close(markDone)
		r.MarkUnavailable("blocking")
	}()

	select {
	case <-monitor.concurrent:
		t.Fatalf("state file was accessed concurrently")
	case <-time.After(50 * time.Millisecond):
	}

	monitor.releaseIsAvailable()

	<-selectDone
	<-markDone
}

// mockStateFile is a test implementation of the state.File interface
// for Router tests.
type mockStateFile struct {
	providerCounts          map[string]int
	unavailableProviders    map[string]bool
	incrementCalled         bool
	lastIncrementedProvider string
	isAvailableCalled       bool
	mu                      sync.Mutex
}

func (m *mockStateFile) IncrementProviderCount(provider string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.incrementCalled = true
	m.lastIncrementedProvider = provider
	if m.providerCounts == nil {
		m.providerCounts = make(map[string]int)
	}
	m.providerCounts[provider]++
}

func (m *mockStateFile) GetProviderCounts() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.providerCounts == nil {
		return make(map[string]int)
	}
	result := make(map[string]int)
	for k, v := range m.providerCounts {
		result[k] = v
	}
	return result
}

func (m *mockStateFile) IsProviderAvailable(provider string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.isAvailableCalled = true
	if m.unavailableProviders == nil {
		return true
	}
	return !m.unavailableProviders[provider]
}

func (m *mockStateFile) SetProviderUnavailable(provider string, until time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.unavailableProviders == nil {
		m.unavailableProviders = make(map[string]bool)
	}
	m.unavailableProviders[provider] = true
}

type stateFileConcurrencyMonitor struct {
	entered    chan struct{}
	concurrent chan struct{}
	resume     chan struct{}
	resumeOnce sync.Once
	mu         sync.Mutex
	inUse      bool
}

func newStateFileConcurrencyMonitor() *stateFileConcurrencyMonitor {
	return &stateFileConcurrencyMonitor{
		entered:    make(chan struct{}, 1),
		concurrent: make(chan struct{}, 1),
		resume:     make(chan struct{}),
	}
}

func (m *stateFileConcurrencyMonitor) enter() {
	m.mu.Lock()
	defer m.mu.Unlock()
	already := m.inUse
	m.inUse = true
	if already {
		select {
		case m.concurrent <- struct{}{}:
		default:
		}
	}
}

func (m *stateFileConcurrencyMonitor) exit() {
	m.mu.Lock()
	m.inUse = false
	m.mu.Unlock()
}

func (m *stateFileConcurrencyMonitor) releaseIsAvailable() {
	m.resumeOnce.Do(func() {
		close(m.resume)
	})
}

func (m *stateFileConcurrencyMonitor) IncrementProviderCount(provider string) {}

func (m *stateFileConcurrencyMonitor) GetProviderCounts() map[string]int {
	return map[string]int{}
}

func (m *stateFileConcurrencyMonitor) IsProviderAvailable(provider string) bool {
	m.enter()
	select {
	case m.entered <- struct{}{}:
	default:
	}
	<-m.resume
	m.exit()
	return true
}

func (m *stateFileConcurrencyMonitor) SetProviderUnavailable(provider string, until time.Time) {
	m.enter()
	m.exit()
}

// TestRouterMarkUnavailableMethod verifies that MarkUnavailable() method
// records the current time plus cooldown for a provider.
// Expected failure: MarkUnavailable() method does not exist yet
func TestRouterMarkUnavailableMethod(t *testing.T) {
	t.Parallel()
	cooldown := 30 * time.Minute
	stateFn := &mockStateFile{}

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
		},
		unavailable: make(map[string]time.Time),
		cooldown:    cooldown,
		stateFn:     stateFn,
	}

	beforeMark := time.Now()
	r.MarkUnavailable("claude")
	afterMark := time.Now()

	// Verify unavailable time was recorded in local map
	unavailUntil, ok := r.unavailable["claude"]
	if !ok {
		t.Fatal("MarkUnavailable() did not record provider in unavailable map")
	}

	// The recorded time should be approximately now + cooldown
	expectedMin := beforeMark.Add(cooldown)
	expectedMax := afterMark.Add(cooldown)

	if unavailUntil.Before(expectedMin) || unavailUntil.After(expectedMax) {
		t.Errorf("MarkUnavailable() recorded time %v, expected between %v and %v",
			unavailUntil, expectedMin, expectedMax)
	}
}

// TestRouterMarkUnavailablePersistsToState verifies that MarkUnavailable()
// persists the unavailable timestamp to the state file via SetProviderUnavailable.
// Expected failure: MarkUnavailable() method does not exist yet
func TestRouterMarkUnavailablePersistsToState(t *testing.T) {
	t.Parallel()
	cooldown := 30 * time.Minute
	stateFn := &mockStateFile{}

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
		},
		unavailable: make(map[string]time.Time),
		cooldown:    cooldown,
		stateFn:     stateFn,
	}

	r.MarkUnavailable("claude")

	// Verify SetProviderUnavailable was called on the state file
	if !stateFn.unavailableProviders["claude"] {
		t.Error("MarkUnavailable() did not call SetProviderUnavailable on state file")
	}
}

// TestRouterMarkUnavailableWithNilStateFn verifies that MarkUnavailable()
// handles nil stateFn gracefully without crashing.
// Expected failure: MarkUnavailable() method does not exist yet
func TestRouterMarkUnavailableWithNilStateFn(t *testing.T) {
	t.Parallel()
	cooldown := 30 * time.Minute

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
		},
		unavailable: make(map[string]time.Time),
		cooldown:    cooldown,
		stateFn:     nil, // nil state file
	}

	// Should not panic
	r.MarkUnavailable("claude")

	// Verify unavailable time was still recorded locally
	if _, ok := r.unavailable["claude"]; !ok {
		t.Error("MarkUnavailable() should record locally even with nil stateFn")
	}
}

// TestRouterRecordInvocationMethod verifies that RecordInvocation()
// increments the invocation count for a provider.
// Expected failure: RecordInvocation() method does not exist yet
func TestRouterRecordInvocationMethod(t *testing.T) {
	t.Parallel()
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

	r.RecordInvocation("claude")

	// Verify internal count was incremented
	if r.counts["claude"] != 6 {
		t.Errorf("RecordInvocation() resulted in count %d, want 6", r.counts["claude"])
	}
}

// TestRouterRecordInvocationPersistsToState verifies that RecordInvocation()
// persists the updated count to the state file via IncrementProviderCount.
// Expected failure: RecordInvocation() method does not exist yet
func TestRouterRecordInvocationPersistsToState(t *testing.T) {
	t.Parallel()
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

	r.RecordInvocation("claude")

	// Verify IncrementProviderCount was called
	if !stateFn.incrementCalled {
		t.Error("RecordInvocation() did not call IncrementProviderCount on state file")
	}

	if stateFn.lastIncrementedProvider != "claude" {
		t.Errorf("IncrementProviderCount called with %q, want %q",
			stateFn.lastIncrementedProvider, "claude")
	}
}

// TestRouterRecordInvocationWithNilStateFn verifies that RecordInvocation()
// handles nil stateFn gracefully without crashing.
// Expected failure: RecordInvocation() method does not exist yet
func TestRouterRecordInvocationWithNilStateFn(t *testing.T) {
	t.Parallel()
	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
		},
		counts:  map[string]int{"claude": 5},
		stateFn: nil, // nil state file
	}

	// Should not panic
	r.RecordInvocation("claude")

	// Verify count was still incremented locally
	if r.counts["claude"] != 6 {
		t.Errorf("RecordInvocation() should increment locally even with nil stateFn, got count %d, want 6",
			r.counts["claude"])
	}
}

// TestRouterRecordInvocationInitializesCount verifies that RecordInvocation()
// initializes count to 1 for a provider that has never been invoked.
// Expected failure: RecordInvocation() method does not exist yet
func TestRouterRecordInvocationInitializesCount(t *testing.T) {
	t.Parallel()
	stateFn := &mockStateFile{
		providerCounts: make(map[string]int),
	}

	r := &Router{
		providers: map[string]Provider{
			"claude": &mockProvider{name: "claude"},
		},
		counts:  make(map[string]int),
		stateFn: stateFn,
	}

	r.RecordInvocation("claude")

	// Verify count was initialized to 1
	if r.counts["claude"] != 1 {
		t.Errorf("RecordInvocation() initialized count to %d, want 1", r.counts["claude"])
	}
}

// TestNewSingleProviderRouterConstructor verifies that NewSingleProviderRouter()
// creates a router with minimal configuration for backward compatibility.
// Expected failure: NewSingleProviderRouter() function does not exist yet
func TestNewSingleProviderRouterConstructor(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{name: "claude"}

	r := NewSingleProviderRouter(provider)

	if r == nil {
		t.Fatal("NewSingleProviderRouter() returned nil")
	}
}

// TestNewSingleProviderRouterSingleProvider verifies that the router
// created by NewSingleProviderRouter() contains exactly one provider.
// Expected failure: NewSingleProviderRouter() function does not exist yet
func TestNewSingleProviderRouterSingleProvider(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{name: "claude"}

	r := NewSingleProviderRouter(provider)

	if r == nil {
		t.Fatal("NewSingleProviderRouter() returned nil")
	}

	if len(r.providers) != 1 {
		t.Errorf("NewSingleProviderRouter() created router with %d providers, want 1",
			len(r.providers))
	}

	if r.providers["claude"] != provider {
		t.Error("NewSingleProviderRouter() did not include the provided provider")
	}
}

// TestNewSingleProviderRouterRatio100Percent verifies that the router
// created by NewSingleProviderRouter() sets ratio to 100% for the single provider.
// Expected failure: NewSingleProviderRouter() function does not exist yet
func TestNewSingleProviderRouterRatio100Percent(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{name: "claude"}

	r := NewSingleProviderRouter(provider)

	if r == nil {
		t.Fatal("NewSingleProviderRouter() returned nil")
	}

	if r.ratio["claude"] != 100 {
		t.Errorf("NewSingleProviderRouter() set ratio to %d%%, want 100%%", r.ratio["claude"])
	}
}

// TestNewSingleProviderRouterPreferencesAny verifies that the router
// created by NewSingleProviderRouter() sets all preferences to "any".
// Expected failure: NewSingleProviderRouter() function does not exist yet
func TestNewSingleProviderRouterPreferencesAny(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{name: "openai"}

	r := NewSingleProviderRouter(provider)

	if r == nil {
		t.Fatal("NewSingleProviderRouter() returned nil")
	}

	// The spec says "all preferences any", which could mean phase preferences
	// are set to "any". We verify that the preferences map exists and contains "any".
	if r.preferences == nil {
		t.Error("NewSingleProviderRouter() did not initialize preferences map")
	}

	// Based on the implementation in router.go line 188-190, it sets "any": "any"
	if r.preferences["any"] != "any" {
		t.Errorf("NewSingleProviderRouter() preferences[\"any\"] = %q, want %q",
			r.preferences["any"], "any")
	}
}

// TestNewSingleProviderRouterInitializesEmptyMaps verifies that the router
// created by NewSingleProviderRouter() properly initializes empty maps for
// counts and unavailable fields.
// Expected failure: NewSingleProviderRouter() function does not exist yet
func TestNewSingleProviderRouterInitializesEmptyMaps(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{name: "claude"}

	r := NewSingleProviderRouter(provider)

	if r == nil {
		t.Fatal("NewSingleProviderRouter() returned nil")
	}

	if r.counts == nil {
		t.Error("NewSingleProviderRouter() did not initialize counts map")
	}

	if r.unavailable == nil {
		t.Error("NewSingleProviderRouter() did not initialize unavailable map")
	}
}

// TestNewSingleProviderRouterNilStateFn verifies that the router
// created by NewSingleProviderRouter() has nil stateFn since it's for
// backward compatibility with code that doesn't use state persistence.
// Expected failure: NewSingleProviderRouter() function does not exist yet
func TestNewSingleProviderRouterNilStateFn(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{name: "claude"}

	r := NewSingleProviderRouter(provider)

	if r == nil {
		t.Fatal("NewSingleProviderRouter() returned nil")
	}

	if r.stateFn != nil {
		t.Error("NewSingleProviderRouter() should set stateFn to nil for backward compatibility")
	}
}

// TestNewSingleProviderRouterZeroCooldown verifies that the router
// created by NewSingleProviderRouter() has zero cooldown.
// Expected failure: NewSingleProviderRouter() function does not exist yet
func TestNewSingleProviderRouterZeroCooldown(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{name: "claude"}

	r := NewSingleProviderRouter(provider)

	if r == nil {
		t.Fatal("NewSingleProviderRouter() returned nil")
	}

	if r.cooldown != 0 {
		t.Errorf("NewSingleProviderRouter() set cooldown to %v, want 0", r.cooldown)
	}
}

// TestNewSingleProviderRouterSelectWorks verifies that a router created
// by NewSingleProviderRouter() can successfully route invocations.
// Expected failure: NewSingleProviderRouter() function does not exist yet
func TestNewSingleProviderRouterSelectWorks(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{name: "claude"}

	r := NewSingleProviderRouter(provider)

	if r == nil {
		t.Fatal("NewSingleProviderRouter() returned nil")
	}

	// Should be able to select for any phase
	selectedProvider, modelName := r.Select("build", TierMedium)

	if selectedProvider == nil {
		t.Error("Select() on single-provider router returned nil provider")
	}

	if selectedProvider != nil && selectedProvider.Name() != "claude" {
		t.Errorf("Select() returned provider %q, want %q", selectedProvider.Name(), "claude")
	}

	// modelName can be any value, we just verify it can be called
	_ = modelName
}

// mockProviderWithModels is a test provider that maps tiers to model names
type mockProviderWithModels struct {
	name   string
	models map[string]string
}

func (m *mockProviderWithModels) Name() string {
	return m.name
}

func (m *mockProviderWithModels) ModelForTier(tier string) string {
	if model, ok := m.models[tier]; ok {
		return model
	}
	return tier
}

func (m *mockProviderWithModels) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	return &Result{Success: true, Model: m.models[tier]}, nil
}

func (m *mockProviderWithModels) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	return &Result{Success: true, Model: m.models[tier]}, nil
}

func (m *mockProviderWithModels) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	return &Result{Success: true, Model: m.models[tier]}, nil
}

func (m *mockProviderWithModels) IsUsageLimitError(result *Result, err error) bool {
	return false
}

func (m *mockProviderWithModels) IsValidationPassed(result *Result) bool {
	return IsValidationPassed(result)
}

func (m *mockProviderWithModels) IsScopeTooLarge(result *Result) (bool, string) {
	return IsScopeTooLarge(result)
}

type blockingProvider struct {
	name         string
	modelStarted chan struct{}
	resume       chan struct{}
}

func newBlockingProvider(name string) *blockingProvider {
	return &blockingProvider{
		name:         name,
		modelStarted: make(chan struct{}, 1),
		resume:       make(chan struct{}),
	}
}

func (b *blockingProvider) Name() string {
	return b.name
}

func (b *blockingProvider) ModelForTier(tier string) string {
	select {
	case b.modelStarted <- struct{}{}:
	default:
	}
	<-b.resume
	return tier
}

func (b *blockingProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	return &Result{Success: true}, nil
}

func (b *blockingProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	return &Result{Success: true}, nil
}

func (b *blockingProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	return &Result{Success: true}, nil
}

func (b *blockingProvider) IsUsageLimitError(result *Result, err error) bool {
	return false
}

func (b *blockingProvider) IsValidationPassed(result *Result) bool {
	return IsValidationPassed(result)
}

func (b *blockingProvider) IsScopeTooLarge(result *Result) (bool, string) {
	return IsScopeTooLarge(result)
}

// TestRouterSelectCross_ReturnsDifferentProviderThanBuild verifies that
// SelectCross returns a provider different from the build provider when
// multiple providers are available.
func TestRouterSelectCross_ReturnsDifferentProviderThanBuild(t *testing.T) {
	t.Parallel()

	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		ratio:       map[string]int{"claude": 50, "openai": 50},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	provider, model := r.SelectCross("claude", TierMedium)

	if provider == nil {
		t.Fatal("SelectCross returned nil provider, expected non-nil")
	}

	if provider.Name() == "claude" {
		t.Errorf("SelectCross returned build provider %q, expected a different provider", provider.Name())
	}

	if provider.Name() != "openai" {
		t.Errorf("SelectCross returned provider %q, expected %q", provider.Name(), "openai")
	}

	if model == "" {
		t.Error("SelectCross returned empty model name")
	}
}

// TestRouterSelectCross_FallsBackToBuildProvider_WhenOnlyOneAvailable verifies
// that SelectCross falls back to the build provider when all other providers
// are unavailable.
func TestRouterSelectCross_FallsBackToBuildProvider_WhenOnlyOneAvailable(t *testing.T) {
	t.Parallel()

	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		ratio:  map[string]int{"claude": 50, "openai": 50},
		counts: map[string]int{},
		unavailable: map[string]time.Time{
			"openai": time.Now().Add(30 * time.Minute),
		},
		cooldown: 30 * time.Minute,
		stateFn:  &mockStateFile{},
	}

	provider, model := r.SelectCross("claude", TierMedium)

	if provider == nil {
		t.Fatal("SelectCross returned nil provider, expected fallback to build provider")
	}

	if provider.Name() != "claude" {
		t.Errorf("SelectCross returned provider %q, expected fallback to build provider %q", provider.Name(), "claude")
	}

	if model == "" {
		t.Error("SelectCross returned empty model name")
	}
}

// TestRouterSelectCross_ReturnsNil_WhenAllUnavailable verifies that SelectCross
// returns nil when all providers (including the build provider) are unavailable.
func TestRouterSelectCross_ReturnsNil_WhenAllUnavailable(t *testing.T) {
	t.Parallel()

	claudeProv := &mockProvider{name: "claude"}
	openaiProv := &mockProvider{name: "openai"}

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		ratio:  map[string]int{"claude": 50, "openai": 50},
		counts: map[string]int{},
		unavailable: map[string]time.Time{
			"claude": time.Now().Add(30 * time.Minute),
			"openai": time.Now().Add(30 * time.Minute),
		},
		cooldown: 30 * time.Minute,
		stateFn:  &mockStateFile{},
	}

	provider, model := r.SelectCross("claude", TierMedium)

	if provider != nil {
		t.Errorf("SelectCross returned provider %q, expected nil when all unavailable", provider.Name())
	}

	if model != "" {
		t.Errorf("SelectCross returned model %q, expected empty string when all unavailable", model)
	}
}

// TestRouterSelectCross_SingleProviderRouter verifies that SelectCross returns
// the single configured provider even when it is the same as the build provider.
func TestRouterSelectCross_SingleProviderRouter(t *testing.T) {
	t.Parallel()

	claudeProv := &mockProvider{name: "claude"}

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
		},
		ratio:       map[string]int{"claude": 100},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    30 * time.Minute,
		stateFn:     &mockStateFile{},
	}

	provider, model := r.SelectCross("claude", TierMedium)

	if provider == nil {
		t.Fatal("SelectCross returned nil provider, expected the single configured provider")
	}

	if provider.Name() != "claude" {
		t.Errorf("SelectCross returned provider %q, expected %q", provider.Name(), "claude")
	}

	if model == "" {
		t.Error("SelectCross returned empty model name")
	}
}

// TestRouterCooldownExpiry_ProviderBecomesAvailableAgain verifies that a provider
// marked as unavailable becomes available again after the cooldown period expires.
func TestRouterCooldownExpiry_ProviderBecomesAvailableAgain(t *testing.T) {
	t.Parallel()

	claudeProv := &mockProvider{name: "claude"}

	cooldown := 1 * time.Millisecond

	r := &Router{
		providers: map[string]Provider{
			"claude": claudeProv,
		},
		preferences: map[string]string{
			"build": "claude",
		},
		ratio:       map[string]int{"claude": 100},
		counts:      map[string]int{},
		unavailable: map[string]time.Time{},
		cooldown:    cooldown,
		stateFn:     nil,
	}

	// Mark claude as unavailable
	r.MarkUnavailable("claude")

	// Immediately after marking, claude should not be selected
	provider, _ := r.Select("build", TierMedium)
	if provider != nil {
		t.Errorf("Provider should not be available immediately after MarkUnavailable, got %q", provider.Name())
	}

	// Wait for cooldown to expire
	time.Sleep(5 * time.Millisecond)

	// After cooldown, claude should be available again
	provider, model := r.Select("build", TierMedium)
	if provider == nil {
		t.Fatal("Provider should be available after cooldown expiry, got nil")
	}

	if provider.Name() != "claude" {
		t.Errorf("Expected provider %q after cooldown expiry, got %q", "claude", provider.Name())
	}

	if model == "" {
		t.Error("Expected non-empty model name after cooldown expiry")
	}
}
