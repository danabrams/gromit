package provider

import (
	"sort"
	"sync"

	"github.com/danabrams/gromit/internal/config"
)

const (
	defaultCircuitBreakerWindowSize        = 10
	defaultCircuitBreakerFailureThreshold  = 0.3
	defaultCircuitBreakerDegradedFloor     = 20
	defaultCircuitBreakerRecoverySuccesses = 5
)

// NewCircuitBreaker creates a CircuitBreaker from config, or returns nil if disabled.
// When config is nil or disabled=false, returns nil (circuit-breaker is optional).
func NewCircuitBreaker(cfg *config.CircuitBreakerConfig) *CircuitBreaker {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	return &CircuitBreaker{
		windowSize:        cfg.WindowSize,
		failureThreshold:  cfg.FailureThreshold,
		degradedFloor:     cfg.DegradedFloor,
		recoverySuccesses: cfg.RecoverySuccesses,
	}
}

// CircuitBreaker tracks recent provider outcomes and temporarily degrades
// routing ratio for providers with elevated transport disconnect failures.
type CircuitBreaker struct {
	windowSize        int
	failureThreshold  float64
	degradedFloor     int
	recoverySuccesses int

	mu                   sync.Mutex
	windows              map[string]*outcomeWindow
	degraded             map[string]bool
	consecutiveSuccesses map[string]int
}

type outcomeWindow struct {
	outcomes []bool
	next     int
	count    int
	failures int
}

func newOutcomeWindow(size int) *outcomeWindow {
	return &outcomeWindow{outcomes: make([]bool, size)}
}

func (w *outcomeWindow) record(isTransportDisconnect bool) {
	if len(w.outcomes) == 0 {
		return
	}

	if w.count == len(w.outcomes) {
		if w.outcomes[w.next] {
			w.failures--
		}
	} else {
		w.count++
	}

	w.outcomes[w.next] = isTransportDisconnect
	if isTransportDisconnect {
		w.failures++
	}

	w.next = (w.next + 1) % len(w.outcomes)
}

func (w *outcomeWindow) failureRatio() float64 {
	if w.count == 0 {
		return 0
	}
	return float64(w.failures) / float64(w.count)
}

// RecordOutcome appends a provider outcome and updates degraded/recovery state.
func (cb *CircuitBreaker) RecordOutcome(providerName string, failureCategory string) {
	if cb == nil || providerName == "" {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	window := cb.ensureProvider(providerName)
	isTransportDisconnect := failureCategory == FailureCategoryTransportDisconnect
	isSuccess := failureCategory == FailureCategoryNone
	window.record(isTransportDisconnect)

	if cb.degraded[providerName] {
		if isSuccess {
			cb.consecutiveSuccesses[providerName]++
			if cb.consecutiveSuccesses[providerName] >= cb.effectiveRecoverySuccesses() {
				cb.degraded[providerName] = false
				cb.consecutiveSuccesses[providerName] = 0
			}
			return
		}

		cb.consecutiveSuccesses[providerName] = 0
		return
	}

	if window.failureRatio() > cb.effectiveFailureThreshold() {
		cb.degraded[providerName] = true
		cb.consecutiveSuccesses[providerName] = 0
	}
}

// EffectiveRatio returns the adjusted ratio map after applying degraded floors
// and redistributing freed ratios to healthy providers.
func (cb *CircuitBreaker) EffectiveRatio(configured map[string]int) map[string]int {
	if len(configured) == 0 {
		return map[string]int{}
	}

	if cb == nil {
		return copyRatioMap(configured)
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	providers := make([]string, 0, len(configured))
	for provider := range configured {
		providers = append(providers, provider)
	}
	sort.Strings(providers)

	result := make(map[string]int, len(configured))
	floor := cb.effectiveDegradedFloor()
	freed := 0
	healthyBase := 0

	for _, provider := range providers {
		configuredRatio := configured[provider]
		if cb.degraded != nil && cb.degraded[provider] {
			if configuredRatio > floor {
				freed += configuredRatio - floor
				result[provider] = floor
			} else {
				result[provider] = configuredRatio
			}
			continue
		}

		result[provider] = configuredRatio
		healthyBase += configuredRatio
	}

	if freed > 0 && healthyBase > 0 {
		distributed := 0
		for _, provider := range providers {
			if cb.degraded != nil && cb.degraded[provider] {
				continue
			}
			configuredRatio := configured[provider]
			extra := freed * configuredRatio / healthyBase
			result[provider] += extra
			distributed += extra
		}

		remainder := freed - distributed
		for _, provider := range providers {
			if remainder == 0 {
				break
			}
			if cb.degraded != nil && cb.degraded[provider] {
				continue
			}
			result[provider]++
			remainder--
		}
	}

	return result
}

// IsDegraded reports whether the provider is currently in degraded mode.
func (cb *CircuitBreaker) IsDegraded(providerName string) bool {
	if cb == nil || providerName == "" {
		return false
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.degraded != nil && cb.degraded[providerName]
}

func (cb *CircuitBreaker) ensureProvider(providerName string) *outcomeWindow {
	if cb.windows == nil {
		cb.windows = make(map[string]*outcomeWindow)
	}
	if cb.degraded == nil {
		cb.degraded = make(map[string]bool)
	}
	if cb.consecutiveSuccesses == nil {
		cb.consecutiveSuccesses = make(map[string]int)
	}

	windowSize := cb.effectiveWindowSize()
	window, ok := cb.windows[providerName]
	if ok && len(window.outcomes) == windowSize {
		return window
	}

	window = newOutcomeWindow(windowSize)
	cb.windows[providerName] = window
	return window
}

func (cb *CircuitBreaker) effectiveWindowSize() int {
	if cb.windowSize <= 0 {
		return defaultCircuitBreakerWindowSize
	}
	return cb.windowSize
}

func (cb *CircuitBreaker) effectiveFailureThreshold() float64 {
	if cb.failureThreshold <= 0 {
		return defaultCircuitBreakerFailureThreshold
	}
	return cb.failureThreshold
}

func (cb *CircuitBreaker) effectiveDegradedFloor() int {
	if cb.degradedFloor <= 0 {
		return defaultCircuitBreakerDegradedFloor
	}
	return cb.degradedFloor
}

func (cb *CircuitBreaker) effectiveRecoverySuccesses() int {
	if cb.recoverySuccesses <= 0 {
		return defaultCircuitBreakerRecoverySuccesses
	}
	return cb.recoverySuccesses
}

func copyRatioMap(src map[string]int) map[string]int {
	if src == nil {
		return map[string]int{}
	}

	dst := make(map[string]int, len(src))
	for key, value := range src {
		dst[key] = value
	}

	return dst
}
