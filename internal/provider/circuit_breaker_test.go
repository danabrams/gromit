package provider

import "testing"

type circuitBreakerStep struct {
	provider string
	category string
	want     map[string]bool
}

func TestCircuitBreakerStateTransitions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		cb   *CircuitBreaker
		seq  []circuitBreakerStep
	}{
		{
			name: "degrades_when_transport_disconnect_ratio_exceeds_threshold",
			cb: &CircuitBreaker{
				windowSize:        5,
				failureThreshold:  0.5,
				recoverySuccesses: 2,
			},
			seq: []circuitBreakerStep{
				{provider: "claude", category: FailureCategoryNone, want: map[string]bool{"claude": false}},
				{provider: "claude", category: FailureCategoryTransportDisconnect, want: map[string]bool{"claude": false}},
				{provider: "claude", category: FailureCategoryTransportDisconnect, want: map[string]bool{"claude": true}},
			},
		},
		{
			name: "non_transport_failures_do_not_trigger_degradation",
			cb: &CircuitBreaker{
				windowSize:       3,
				failureThreshold: 0.2,
			},
			seq: []circuitBreakerStep{
				{provider: "codex", category: FailureCategoryOther, want: map[string]bool{"codex": false}},
				{provider: "codex", category: FailureCategoryRateLimited, want: map[string]bool{"codex": false}},
				{provider: "codex", category: FailureCategoryAuth, want: map[string]bool{"codex": false}},
			},
		},
		{
			name: "recovers_after_configured_consecutive_successes",
			cb: &CircuitBreaker{
				windowSize:        4,
				failureThreshold:  0.3,
				recoverySuccesses: 3,
			},
			seq: []circuitBreakerStep{
				{provider: "claude", category: FailureCategoryTransportDisconnect, want: map[string]bool{"claude": true}},
				{provider: "claude", category: FailureCategoryNone, want: map[string]bool{"claude": true}},
				{provider: "claude", category: FailureCategoryOther, want: map[string]bool{"claude": true}},
				{provider: "claude", category: FailureCategoryNone, want: map[string]bool{"claude": true}},
				{provider: "claude", category: FailureCategoryNone, want: map[string]bool{"claude": true}},
				{provider: "claude", category: FailureCategoryNone, want: map[string]bool{"claude": false}},
			},
		},
		{
			name: "providers_are_tracked_independently",
			cb: &CircuitBreaker{
				windowSize:       3,
				failureThreshold: 0.4,
			},
			seq: []circuitBreakerStep{
				{provider: "claude", category: FailureCategoryTransportDisconnect, want: map[string]bool{"claude": true, "codex": false}},
				{provider: "codex", category: FailureCategoryNone, want: map[string]bool{"claude": true, "codex": false}},
				{provider: "codex", category: FailureCategoryTransportDisconnect, want: map[string]bool{"claude": true, "codex": true}},
			},
		},
		{
			name: "sliding_window_evicts_old_outcomes",
			cb: &CircuitBreaker{
				windowSize:        3,
				failureThreshold:  0.8,
				recoverySuccesses: 4,
			},
			seq: []circuitBreakerStep{
				{provider: "claude", category: FailureCategoryTransportDisconnect, want: map[string]bool{"claude": true}},
				{provider: "claude", category: FailureCategoryNone, want: map[string]bool{"claude": true}},
				{provider: "claude", category: FailureCategoryNone, want: map[string]bool{"claude": true}},
				{provider: "claude", category: FailureCategoryNone, want: map[string]bool{"claude": true}},
				{provider: "claude", category: FailureCategoryNone, want: map[string]bool{"claude": false}},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for i, step := range tc.seq {
				tc.cb.Record(step.provider, step.category)

				for providerName, wantDegraded := range step.want {
					if got := tc.cb.IsDegraded(providerName); got != wantDegraded {
						t.Fatalf("step %d provider %q IsDegraded() = %t, want %t", i, providerName, got, wantDegraded)
					}
				}
			}
		})
	}
}

func TestCircuitBreakerEffectiveRatio(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		cb              *CircuitBreaker
		provider        string
		configuredRatio int
		seedDegraded    bool
		want            int
	}{
		{
			name:            "nil_circuit_breaker_passes_through",
			cb:              nil,
			provider:        "claude",
			configuredRatio: 60,
			want:            60,
		},
		{
			name: "healthy_provider_passes_through",
			cb: &CircuitBreaker{
				windowSize:       3,
				failureThreshold: 0.6,
				degradedFloor:    20,
			},
			provider:        "claude",
			configuredRatio: 60,
			want:            60,
		},
		{
			name: "degraded_provider_returns_floor",
			cb: &CircuitBreaker{
				windowSize:       3,
				failureThreshold: 0.3,
				degradedFloor:    20,
			},
			provider:        "claude",
			configuredRatio: 60,
			seedDegraded:    true,
			want:            20,
		},
		{
			name: "degraded_floor_is_returned_even_if_higher_than_configured_ratio",
			cb: &CircuitBreaker{
				windowSize:       3,
				failureThreshold: 0.3,
				degradedFloor:    20,
			},
			provider:        "claude",
			configuredRatio: 10,
			seedDegraded:    true,
			want:            20,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.cb != nil && tc.seedDegraded {
				tc.cb.Record(tc.provider, FailureCategoryTransportDisconnect)
			}

			if got := tc.cb.EffectiveRatio(tc.provider, tc.configuredRatio); got != tc.want {
				t.Fatalf("EffectiveRatio(%q, %d) = %d, want %d", tc.provider, tc.configuredRatio, got, tc.want)
			}
		})
	}
}

func TestCircuitBreakerNilSafetyAndDefaults(t *testing.T) {
	t.Parallel()

	var nilCB *CircuitBreaker
	nilCB.Record("claude", FailureCategoryTransportDisconnect)

	if got := nilCB.EffectiveRatio("claude", 55); got != 55 {
		t.Fatalf("nil EffectiveRatio() = %d, want 55", got)
	}
	if nilCB.IsDegraded("claude") {
		t.Fatal("nil IsDegraded() = true, want false")
	}

	defaultCB := &CircuitBreaker{}
	defaultCB.Record("claude", FailureCategoryTransportDisconnect)
	if !defaultCB.IsDegraded("claude") {
		t.Fatal("default CircuitBreaker should degrade after transport_disconnect at default threshold")
	}

	for i := 0; i < defaultCircuitBreakerRecoverySuccesses; i++ {
		defaultCB.Record("claude", FailureCategoryNone)
	}
	if defaultCB.IsDegraded("claude") {
		t.Fatal("default CircuitBreaker should recover after default recovery successes")
	}
}
