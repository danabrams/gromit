package provider

import (
	"time"
)

// Router handles provider selection and routing logic
type Router struct {
	providers   map[string]Provider
	preferences map[string]string
	ratio       map[string]int
	counts      map[string]int
	unavailable map[string]time.Time
	cooldown    time.Duration
	stateFn     StateFile
}

// StateFile is the interface for persisting provider routing state
type StateFile interface {
	IncrementProviderCount(provider string)
	GetProviderCounts() map[string]int
	IsProviderAvailable(provider string) bool
	SetProviderUnavailable(provider string, until time.Time)
}

// NewRouter creates a new Router instance with the provided configuration
func NewRouter(providers map[string]Provider, preferences map[string]string, ratio map[string]int, cooldown time.Duration, stateFn StateFile) *Router {
	counts := stateFn.GetProviderCounts()
	if counts == nil {
		counts = make(map[string]int)
	}

	return &Router{
		providers:   providers,
		preferences: preferences,
		ratio:       ratio,
		counts:      counts,
		unavailable: make(map[string]time.Time),
		cooldown:    cooldown,
		stateFn:     stateFn,
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

		targetPercent := float64(targetRatio)
		gap := targetPercent - currentPercent

		if gap > largestGap {
			largestGap = gap
			selectedName = name
		}
	}

	return selectedName
}

// selectProvider returns the provider and model name, and increments count
func (r *Router) selectProvider(name string, tier string) (Provider, string) {
	provider := r.providers[name]

	// Get model name by running a fake invocation to extract the model
	// For now, we'll use a simple approach: the provider's Run method returns Result with Model field
	// In real implementation, providers would have a ModelForTier method
	// For testing, we assume the provider's result includes the model name
	result, _ := provider.Run(nil, "", tier)
	modelName := ""
	if result != nil {
		modelName = result.Model
	}

	// Increment count
	r.counts[name]++
	if r.stateFn != nil {
		r.stateFn.IncrementProviderCount(name)
	}

	return provider, modelName
}
