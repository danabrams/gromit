package routing

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/danabrams/gromit/internal/v2/llmtypes"
)

var (
	ErrNoProviders    = errors.New("routing: no providers configured")
	ErrAllUnavailable = errors.New("routing: all providers unavailable")
)

// RouterConfig holds the configuration for the Router.
type RouterConfig struct {
	// Providers maps provider name to LLMProvider implementation.
	Providers map[string]llmtypes.LLMProvider
	// PhasePreferences maps phase name to preferred provider name.
	PhasePreferences map[string]string
	// Ratio maps provider name to relative invocation weight.
	Ratio map[string]int
	// Cooldown is the duration before an unavailable provider is re-enabled.
	Cooldown time.Duration
	// Models maps provider name to tier-to-model mapping (e.g. {"claude": {"low": "claude-haiku-4-5-20251001"}}).
	Models map[string]map[string]string
	// NowFunc overrides the clock used for cooldown checks. If nil, defaults to time.Now.
	NowFunc func() time.Time
}

// Router selects providers for LLM invocations using phase preferences,
// ratio balancing, and fallback with cooldown.
type Router struct {
	providers        map[string]llmtypes.LLMProvider
	phasePreferences map[string]string
	ratio            map[string]int
	counts           map[string]int
	unavailable      map[string]time.Time
	cooldown         time.Duration
	models           map[string]map[string]string
	nowFunc          func() time.Time
	mu               sync.Mutex
}

// NewRouter constructs a Router from the provided config.
func NewRouter(cfg RouterConfig) *Router {
	providers := cfg.Providers
	if providers == nil {
		providers = map[string]llmtypes.LLMProvider{}
	}
	phasePreferences := cfg.PhasePreferences
	if phasePreferences == nil {
		phasePreferences = map[string]string{}
	}
	ratio := cfg.Ratio
	if ratio == nil {
		ratio = map[string]int{}
	}
	models := cfg.Models
	if models == nil {
		models = map[string]map[string]string{}
	}
	nowFunc := cfg.NowFunc
	if nowFunc == nil {
		nowFunc = time.Now
	}
	return &Router{
		providers:        providers,
		phasePreferences: phasePreferences,
		ratio:            ratio,
		counts:           map[string]int{},
		unavailable:      map[string]time.Time{},
		cooldown:         cfg.Cooldown,
		models:           models,
		nowFunc:          nowFunc,
	}
}

// Select picks an LLMProvider and model name for the given phase and tier.
// Phase preferences are checked first, then ratio balancing among available providers.
// Returns an error when no providers are configured or all are unavailable.
// Automatically records the invocation for ratio balancing.
func (r *Router) Select(phase, tier string) (llmtypes.LLMProvider, string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.providers) == 0 {
		return nil, "", "", ErrNoProviders
	}

	now := r.nowFunc()

	// Phase preference: if a specific provider is pinned for this phase, use it.
	if preferred, ok := r.phasePreferences[phase]; ok && preferred != "" {
		if p, exists := r.providers[preferred]; exists {
			if !r.isUnavailable(preferred, now) {
				r.counts[preferred]++
				model := ResolveModel(tier, r.models[preferred])
				return p, model, preferred, nil
			}
		}
	}

	// Ratio balancing: pick the available provider most under-served relative to its weight.
	chosen := r.selectByRatio(now)
	if chosen == "" {
		return nil, "", "", ErrAllUnavailable
	}
	r.counts[chosen]++
	model := ResolveModel(tier, r.models[chosen])
	return r.providers[chosen], model, chosen, nil
}

// MarkUnavailable marks the named provider as unavailable for the cooldown duration.
func (r *Router) MarkUnavailable(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	until := r.nowFunc().Add(r.cooldown)
	r.unavailable[name] = until
}

// IsUnavailable reports whether the named provider is currently in cooldown.
func (r *Router) IsUnavailable(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isUnavailable(name, r.nowFunc())
}

// isUnavailable reports whether the named provider is currently in cooldown.
// Caller must hold r.mu.
func (r *Router) isUnavailable(name string, now time.Time) bool {
	until, exists := r.unavailable[name]
	if !exists {
		return false
	}
	if now.After(until) {
		delete(r.unavailable, name)
		return false
	}
	return true
}

// selectByRatio picks the available provider most under-served relative to its weight.
// Returns empty string when all providers are unavailable.
// Caller must hold r.mu.
func (r *Router) selectByRatio(now time.Time) string {
	type candidate struct {
		name  string
		score float64
	}
	// Sort provider names for deterministic tie-breaking.
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)

	var best candidate
	found := false
	for _, name := range names {
		if r.isUnavailable(name, now) {
			continue
		}
		weight := r.ratio[name]
		if weight <= 0 {
			weight = 1
		}
		count := r.counts[name]
		// Score: lower count relative to weight is preferred (most under-served).
		score := float64(count) / float64(weight)
		if !found || score < best.score {
			best = candidate{name: name, score: score}
			found = true
		}
	}
	if !found {
		return ""
	}
	return best.name
}
