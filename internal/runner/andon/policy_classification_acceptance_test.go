//go:build acceptance

package andon

import (
	"testing"
	"time"
)

func setupPolicyEvaluationAcceptance(t *testing.T) (time.Time, AndonThresholds) {
	t.Helper()
	return time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC), DefaultThresholdDefinition()
}

// smoke-matrix: move | rationale: Policy classification and decision selection are deterministic logic with comprehensive unit coverage. | destination: internal/runner/andon/policy_test.go:TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass
func TestEvaluateFailure_ClassifiesAndSelectsDecisionForAllClasses(t *testing.T) {
	now, thresholds := setupPolicyEvaluationAcceptance(t)

	tests := []struct {
		name   string
		signal FailureSignal
		state  RecoveryState
		want   PolicyEvaluation
	}{
		{
			name: "Transient maps timeout signal and keeps retrying in L1",
			signal: FailureSignal{
				Kind:   FailureKindTimeout,
				Output: "context deadline exceeded",
			},
			state: RecoveryState{
				Level:      LevelL1,
				L1Attempts: 0,
				L1Started:  now,
			},
			want: PolicyEvaluation{
				Class:    FailureClassTransient,
				Decision: PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry},
			},
		},
		{
			name: "Workflow maps workflow signal and escalates from L1 after deterministic attempt",
			signal: FailureSignal{
				Kind:   FailureKindWorkflow,
				Output: "bd sync required before close",
			},
			state: RecoveryState{
				Level:      LevelL1,
				L1Attempts: 1,
				L1Started:  now,
			},
			want: PolicyEvaluation{
				Class:    FailureClassWorkflow,
				Decision: PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate},
			},
		},
		{
			name: "Quality maps quality signal and stops line from L2 after timebox",
			signal: FailureSignal{
				Kind:   FailureKindQualityGate,
				Output: "go test ./... failed",
			},
			state: RecoveryState{
				Level:     LevelL2,
				L2Started: now.Add(-16 * time.Minute),
			},
			want: PolicyEvaluation{
				Class:    FailureClassQuality,
				Decision: PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
			},
		},
		{
			name: "Intent maps ambiguous signal and escalates to L3 after assumptions exhausted",
			signal: FailureSignal{
				Kind:   FailureKindAmbiguousIntent,
				Output: "spec leaves behavior undefined",
			},
			state: RecoveryState{
				Level:           LevelL1,
				AssumptionsUsed: thresholds.MaxAssumptions,
			},
			want: PolicyEvaluation{
				Class:    FailureClassIntent,
				Decision: PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate},
			},
		},
		{
			name: "Data maps integrity signal and triggers immediate stop-line",
			signal: FailureSignal{
				Kind:   FailureKindIntegrity,
				Output: "state divergence detected",
			},
			state: RecoveryState{
				Level: LevelL1,
			},
			want: PolicyEvaluation{
				Class:    FailureClassData,
				Decision: PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateFailure(tt.signal, tt.state, thresholds, now)
			if got.Class != tt.want.Class {
				t.Fatalf("EvaluateFailure(...).Class = %q, want %q", got.Class, tt.want.Class)
			}
			if got.Decision != tt.want.Decision {
				t.Fatalf("EvaluateFailure(...).Decision = %+v, want %+v", got.Decision, tt.want.Decision)
			}
		})
	}
}

// smoke-matrix: move | rationale: L1/L2 boundary enforcement is a pure policy rule covered by targeted unit tests. | destination: internal/runner/andon/policy_test.go:TestEvaluateFailure_EnforcesL1ToL2BoundaryAtRetryCap
func TestEvaluateFailure_EnforcesL1L2BoundaryAtPublicEntryPoint(t *testing.T) {
	now, thresholds := setupPolicyEvaluationAcceptance(t)

	signal := FailureSignal{
		Kind:   FailureKindTimeout,
		Output: "transient timeout",
	}
	state := RecoveryState{
		Level:      LevelL1,
		L1Attempts: thresholds.L1MaxRetries,
		L1Started:  now,
	}

	got := EvaluateFailure(signal, state, thresholds, now)
	if got.Class != FailureClassTransient {
		t.Fatalf("EvaluateFailure(...).Class = %q, want %q", got.Class, FailureClassTransient)
	}
	if got.Decision.NextLevel != LevelL2 {
		t.Fatalf("EvaluateFailure(...).Decision.NextLevel = %q, want %q", got.Decision.NextLevel, LevelL2)
	}
	if got.Decision.Action != DecisionEscalate {
		t.Fatalf("EvaluateFailure(...).Decision.Action = %q, want %q", got.Decision.Action, DecisionEscalate)
	}
}
