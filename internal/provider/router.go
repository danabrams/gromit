package provider

import (
	"time"
)

// Router handles provider selection and routing logic
type Router struct {
	providers      map[string]Provider
	preferences    map[string]string
	ratio          map[string]int
	counts         map[string]int
	unavailable    map[string]time.Time
	cooldown       time.Duration
	stateFn        StateFile
	circuitBreaker *CircuitBreaker
}

// StateFile is the interface for persisting provider routing state
type StateFile interface {
	IncrementProviderCount(provider string)
	GetProviderCounts() map[string]int
	IsProviderAvailable(provider string) bool
	SetProviderUnavailable(provider string, until time.Time)
}

// NewRouter creates a new Router instance with the provided configuration
func NewRouter(
	providers map[string]Provider,
	preferences map[string]string,
	ratio map[string]int,
	cooldown time.Duration,
	stateFn StateFile,
	circuitBreaker *CircuitBreaker,
) *Router {
	counts := make(map[string]int)
	if stateFn != nil {
		counts = stateFn.GetProviderCounts()
		if counts == nil {
			counts = make(map[string]int)
		}
	}

	return &Router{
		providers:      providers,
		preferences:    preferences,
		ratio:          ratio,
		counts:         counts,
		unavailable:    make(map[string]time.Time),
		cooldown:       cooldown,
		stateFn:        stateFn,
		circuitBreaker: circuitBreaker,
	}
}

// NewSingleProviderRouter creates a minimal router for backward compatibility
// with one provider, all preferences "any", ratio 100%
func NewSingleProviderRouter(p Provider) *Router {
	name := p.Name()
	return &Router{
		providers: map[string]Provider{
			name: p,
		},
		preferences: map[string]string{
			"any": "any",
		},
		ratio: map[string]int{
			name: 100,
		},
		counts:         make(map[string]int),
		unavailable:    make(map[string]time.Time),
		cooldown:       0,
		stateFn:        nil,
		circuitBreaker: nil,
	}
}

// Select picks the provider for a given phase and tier.
// Returns the selected provider and the concrete model name for that tier.
// Returns nil provider and empty model name if all providers are unavailable.
func (r *Router) Select(phase string, tier string) (Provider, string) {
	// Layer 1: Check phase preference
	if pref, ok := r.preferences[phase]; ok && pref != "any" {
		if r.isAvailable(pref) {
			return r.selectProvider(pref, tier)
		}
	}

	// Layer 2: Ratio balancing for "any" preference or fallback
	selectedName := r.selectByRatio()
	if selectedName != "" && r.isAvailable(selectedName) {
		return r.selectProvider(selectedName, tier)
	}

	// Layer 3: Try any available provider
	for name := range r.providers {
		if r.isAvailable(name) {
			return r.selectProvider(name, tier)
		}
	}

	// All providers unavailable
	return nil, ""
}

// isAvailable checks if a provider is currently available
func (r *Router) isAvailable(name string) bool {
	// Check if provider exists
	if _, ok := r.providers[name]; !ok {
		return false
	}

	// Check state file for availability
	if r.stateFn != nil && !r.stateFn.IsProviderAvailable(name) {
		return false
	}

	// Check local unavailable map
	if until, ok := r.unavailable[name]; ok {
		if time.Now().Before(until) {
			return false
		}
		// Cooldown expired, remove from unavailable
		delete(r.unavailable, name)
	}

	return true
}

// selectByRatio selects the provider furthest below its target ratio.
// Compares each available provider's current usage percentage against its target,
// choosing the one with the largest gap (target - current). This maintains the
// configured ratio over time by prioritizing underused providers.
func (r *Router) selectByRatio() string {
	if len(r.ratio) == 0 {
		return ""
	}

	// Calculate total count across all providers
	totalCount := 0
	for _, count := range r.counts {
		totalCount += count
	}

	// Find available provider with largest gap below its target
	var selectedName string
	largestGap := -1.0

	for name, targetRatio := range r.ratio {
		if !r.isAvailable(name) {
			continue
		}

		currentCount := r.counts[name]
		var currentPercent float64
		if totalCount > 0 {
			currentPercent = float64(currentCount) / float64(totalCount) * 100.0
		}

		targetPercent := float64(r.effectiveRatio(name, targetRatio))
		gap := targetPercent - currentPercent

		if gap > largestGap {
			largestGap = gap
			selectedName = name
		}
	}

	return selectedName
}

func (r *Router) effectiveRatio(providerName string, configuredRatio int) int {
	if r == nil {
		return configuredRatio
	}
	return r.circuitBreaker.EffectiveRatio(providerName, configuredRatio)
}

// selectProvider returns the provider and model name, and increments count
func (r *Router) selectProvider(name string, tier string) (Provider, string) {
	provider := r.providers[name]

	// Get model name for the tier
	modelName := provider.ModelForTier(tier)

	// Increment count
	r.counts[name]++
	if r.stateFn != nil {
		r.stateFn.IncrementProviderCount(name)
	}

	return provider, modelName
}

// SelectCross picks a provider different from buildProvider for cross-review routing.
// Returns the first available provider whose name differs from buildProvider.
// Falls back to buildProvider if no other provider is available.
// Returns nil provider and empty model name if no providers are available at all.
func (r *Router) SelectCross(buildProvider string, tier string) (Provider, string) {
	// Try to find an available provider that differs from the build provider
	for name := range r.providers {
		if name != buildProvider && r.isAvailable(name) {
			return r.selectProvider(name, tier)
		}
	}

	// Fall back to the build provider itself if no cross-provider is available
	if r.isAvailable(buildProvider) {
		return r.selectProvider(buildProvider, tier)
	}

	// No providers available at all
	return nil, ""
}

// MarkUnavailable records current time plus cooldown for the specified provider
func (r *Router) MarkUnavailable(name string) {
	until := time.Now().Add(r.cooldown)
	r.unavailable[name] = until

	if r.stateFn != nil {
		r.stateFn.SetProviderUnavailable(name, until)
	}
}

// RecordInvocation increments count and persists to state via stateFn
func (r *Router) RecordInvocation(name string) {
	r.counts[name]++

	if r.stateFn != nil {
		r.stateFn.IncrementProviderCount(name)
	}
}

// RecordOutcome records an invocation outcome in the circuit breaker.
func (r *Router) RecordOutcome(providerName, failureCategory string) {
	if r == nil {
		return
	}
	r.circuitBreaker.Record(providerName, failureCategory)
}
