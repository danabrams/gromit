//go:build acceptance

package andon

import (
	"testing"
	"time"
)

// Expected failure: PolicyEvaluation.Path and DecisionPath* constants do not exist yet.
func TestEvaluateFailure_UsesClassifiedDecisionPathAtPublicEntryPoint(t *testing.T) {
	now, thresholds := setupPolicyEvaluationAcceptance(t)

	tests := []struct {
		name     string
		signal   FailureSignal
		state    RecoveryState
		wantPath DecisionPath
		want     PolicyDecision
	}{
		{
			name:     "Transient timeout routes through transient decision path",
			signal:   FailureSignal{Kind: FailureKindTimeout, Output: "context deadline exceeded"},
			state:    RecoveryState{Level: LevelL1, L1Attempts: 0, L1Started: now},
			wantPath: DecisionPathTransientL1Retry,
			want:     PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry},
		},
		{
			name:     "Workflow failure routes through workflow escalation path",
			signal:   FailureSignal{Kind: FailureKindWorkflow, Output: "missing bd sync before close"},
			state:    RecoveryState{Level: LevelL1, L1Attempts: 1, L1Started: now},
			wantPath: DecisionPathWorkflowEscalateAfterDeterministicAttempt,
			want:     PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate},
		},
		{
			name:     "Quality-gate failure routes through quality timebox path",
			signal:   FailureSignal{Kind: FailureKindQualityGate, Output: "go test ./... failed"},
			state:    RecoveryState{Level: LevelL2, L2Started: now.Add(-16 * time.Minute)},
			wantPath: DecisionPathQualityStopLineAfterTimebox,
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
		},
		{
			name:     "Intent ambiguity routes through assumption-exhausted path",
			signal:   FailureSignal{Kind: FailureKindAmbiguousIntent, Output: "requirements conflict"},
			state:    RecoveryState{Level: LevelL1, AssumptionsUsed: thresholds.MaxAssumptions},
			wantPath: DecisionPathIntentEscalateAfterAssumptionBudget,
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate},
		},
		{
			name:     "Integrity signal routes through immediate stop-line path",
			signal:   FailureSignal{Kind: FailureKindIntegrity, Output: "state divergence detected"},
			state:    RecoveryState{Level: LevelL1},
			wantPath: DecisionPathDataImmediateStopLine,
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateFailure(tt.signal, tt.state, thresholds, now)
			if got.Decision != tt.want {
				t.Fatalf("EvaluateFailure(...).Decision = %+v, want %+v", got.Decision, tt.want)
			}
			if got.Path != tt.wantPath {
				t.Fatalf("EvaluateFailure(...).Path = %q, want %q", got.Path, tt.wantPath)
			}
		})
	}
}

// Expected failure: DecisionPathForClass helper does not exist yet and class-to-path mapping is not exposed.
func TestEvaluateClassifiedFailure_HasExplicitDecisionPathPerFailureClass(t *testing.T) {
	now, thresholds := setupPolicyEvaluationAcceptance(t)

	representativeState := func(class FailureClass) RecoveryState {
		switch class {
		case FailureClassTransient:
			return RecoveryState{Level: LevelL1, L1Attempts: 0, L1Started: now}
		case FailureClassWorkflow:
			return RecoveryState{Level: LevelL1, L1Attempts: 1, L1Started: now}
		case FailureClassQuality:
			return RecoveryState{Level: LevelL2, L2Started: now.Add(-16 * time.Minute)}
		case FailureClassIntent:
			return RecoveryState{Level: LevelL1, AssumptionsUsed: thresholds.MaxAssumptions}
		case FailureClassData:
			return RecoveryState{Level: LevelL1}
		default:
			return RecoveryState{Level: LevelL1}
		}
	}

	for _, class := range AllFailureClasses() {
		t.Run(string(class), func(t *testing.T) {
			classification := PolicyClassification{Class: class}
			got := EvaluateClassifiedFailure(classification, representativeState(class), thresholds, now)

			wantPath := DecisionPathForClass(class)
			if got.Path != wantPath {
				t.Fatalf("EvaluateClassifiedFailure(%q,...).Path = %q, want %q", class, got.Path, wantPath)
			}
		})
	}
}

// Expected failure: DecisionPathWorkflowFallbackForUnknownKind constant and Path field do not exist yet.
func TestEvaluateFailure_UnknownSignalRemainsDeterministicWithWorkflowFallbackPath(t *testing.T) {
	now, thresholds := setupPolicyEvaluationAcceptance(t)

	signal := FailureSignal{Kind: FailureKind("new_kind_not_yet_classified"), Output: "unrecognized failure envelope"}
	state := RecoveryState{Level: LevelL1, L1Attempts: 1, L1Started: now}

	first := EvaluateFailure(signal, state, thresholds, now)
	second := EvaluateFailure(signal, state, thresholds, now)

	if first != second {
		t.Fatalf("EvaluateFailure(...) must be deterministic for unknown kinds: first=%+v second=%+v", first, second)
	}
	if first.Class != FailureClassWorkflow {
		t.Fatalf("EvaluateFailure(...).Class = %q, want workflow fallback %q", first.Class, FailureClassWorkflow)
	}
	if first.Path != DecisionPathWorkflowFallbackForUnknownKind {
		t.Fatalf("EvaluateFailure(...).Path = %q, want %q", first.Path, DecisionPathWorkflowFallbackForUnknownKind)
	}
}
