package provider

import (
	"reflect"
	"testing"
)

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
				tc.cb.RecordOutcome(step.provider, step.category)

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

	type cbFactory func() *CircuitBreaker

	testCases := []struct {
		name       string
		cb         cbFactory
		configured map[string]int
		want       map[string]int
	}{
		{
			name: "nil_circuit_breaker_passes_through",
			cb: func() *CircuitBreaker {
				return nil
			},
			configured: map[string]int{"claude": 60},
			want:       map[string]int{"claude": 60},
		},
		{
			name: "healthy_provider_passes_through",
			cb: func() *CircuitBreaker {
				return &CircuitBreaker{
					windowSize:       3,
					failureThreshold: 0.6,
					degradedFloor:    20,
				}
			},
			configured: map[string]int{"claude": 60},
			want:       map[string]int{"claude": 60},
		},
		{
			name: "degraded_provider_drops_to_floor",
			cb: func() *CircuitBreaker {
				cb := &CircuitBreaker{degradedFloor: 20}
				cb.degraded = map[string]bool{"claude": true}
				return cb
			},
			configured: map[string]int{"claude": 60},
			want:       map[string]int{"claude": 20},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cb := tc.cb()
			got := cb.EffectiveRatio(copyRatioMap(tc.configured))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("EffectiveRatio(%v) = %#v, want %#v", tc.configured, got, tc.want)
			}
		})
	}
}

func TestCircuitBreakerEffectiveRatioRedistributesFreedRatio(t *testing.T) {
	t.Parallel()

	cb := &CircuitBreaker{
		degradedFloor: 20,
	}
	cb.degraded = map[string]bool{
		"codex": true,
	}

	configured := map[string]int{
		"codex":  60,
		"claude": 40,
	}

	got := cb.EffectiveRatio(configured)
	want := map[string]int{
		"codex":  20,
		"claude": 80,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveRatio() = %#v, want %#v", got, want)
	}
}

func TestCircuitBreakerNilSafetyAndDefaults(t *testing.T) {
	t.Parallel()

	var nilCB *CircuitBreaker
	nilCB.RecordOutcome("claude", FailureCategoryTransportDisconnect)

	if got := nilCB.EffectiveRatio(map[string]int{"claude": 55}); !reflect.DeepEqual(got, map[string]int{"claude": 55}) {
		t.Fatalf("nil EffectiveRatio() = %#v, want %#v", got, map[string]int{"claude": 55})
	}
	if nilCB.IsDegraded("claude") {
		t.Fatal("nil IsDegraded() = true, want false")
	}

	defaultCB := &CircuitBreaker{}
	defaultCB.RecordOutcome("claude", FailureCategoryTransportDisconnect)
	if !defaultCB.IsDegraded("claude") {
		t.Fatal("default CircuitBreaker should degrade after transport_disconnect at default threshold")
	}

	for i := 0; i < defaultCircuitBreakerRecoverySuccesses; i++ {
		defaultCB.RecordOutcome("claude", FailureCategoryNone)
	}
	if defaultCB.IsDegraded("claude") {
		t.Fatal("default CircuitBreaker should recover after default recovery successes")
	}
}
