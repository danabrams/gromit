package andon

import (
	"testing"
	"time"
)

func TestEvaluateFailureWithTrace_ClassToDecisionFlowAcrossPublicEntryPoint(t *testing.T) {
	now := time.Date(2026, time.February, 16, 10, 0, 0, 0, time.UTC)
	thresholds := DefaultThresholds()

	tests := []struct {
		name     string
		signal   FailureSignal
		state    RecoveryState
		want     PolicyDecision
		wantPath DecisionPath
	}{
		{
			name:     "transient remains at L1 when retry budget remains",
			signal:   FailureSignal{Kind: FailureKindTimeout, Output: "timeout"},
			state:    RecoveryState{Level: LevelL1, L1Attempts: 0, L1Started: now.Add(-30 * time.Second)},
			want:     PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry},
			wantPath: DecisionPathTransientL1Retry,
		},
		{
			name:     "workflow escalates from L1 after deterministic attempt",
			signal:   FailureSignal{Kind: FailureKindWorkflow, Output: "workflow"},
			state:    RecoveryState{Level: LevelL1, L1Attempts: 1, L1Started: now.Add(-30 * time.Second)},
			want:     PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate},
			wantPath: DecisionPathWorkflowEscalateAfterDeterministicAttempt,
		},
		{
			name:     "quality stops line when L2 timebox is exhausted",
			signal:   FailureSignal{Kind: FailureKindQualityGate, Output: "quality"},
			state:    RecoveryState{Level: LevelL2, L2Started: now.Add(-20 * time.Minute)},
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
			wantPath: DecisionPathQualityStopLineAfterTimebox,
		},
		{
			name:     "intent escalates when assumption budget is exhausted",
			signal:   FailureSignal{Kind: FailureKindAmbiguousIntent, Output: "intent"},
			state:    RecoveryState{Level: LevelL1, AssumptionsUsed: thresholds.MaxAssumptions},
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate},
			wantPath: DecisionPathIntentEscalateAfterAssumptionBudget,
		},
		{
			name:     "data integrity fails closed immediately",
			signal:   FailureSignal{Kind: FailureKindIntegrity, Output: "integrity"},
			state:    RecoveryState{Level: LevelL1},
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
			wantPath: DecisionPathDataImmediateStopLine,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := EvaluateFailureWithTrace(tt.signal, tt.state, thresholds, now)

			if trace.Decision != tt.want {
				t.Fatalf("EvaluateFailureWithTrace(...).Decision = %+v, want %+v", trace.Decision, tt.want)
			}
			if trace.Path != tt.wantPath {
				t.Fatalf("EvaluateFailureWithTrace(...).Path = %q, want %q", trace.Path, tt.wantPath)
			}
			// Expected failure: DecisionInputSourceTraceClassifier constant does not exist yet.
			if trace.DecisionInputSource != DecisionInputSourceTraceClassifier {
				t.Fatalf("EvaluateFailureWithTrace(...).DecisionInputSource = %q, want %q", trace.DecisionInputSource, DecisionInputSourceTraceClassifier)
			}
		})
	}
}

func TestEvaluateFailureWithTrace_ReportsL1ToL2BoundaryTransition(t *testing.T) {
	now := time.Date(2026, time.February, 16, 10, 0, 0, 0, time.UTC)
	thresholds := DefaultThresholds()

	trace := EvaluateFailureWithTrace(
		FailureSignal{Kind: FailureKindTimeout, Output: "timeout"},
		RecoveryState{Level: LevelL1, L1Attempts: thresholds.L1MaxRetries, L1Started: now.Add(-30 * time.Second)},
		thresholds,
		now,
	)

	if trace.Decision.NextLevel != LevelL2 {
		t.Fatalf("EvaluateFailureWithTrace(...).Decision.NextLevel = %q, want %q", trace.Decision.NextLevel, LevelL2)
	}
	if trace.Decision.Action != DecisionEscalate {
		t.Fatalf("EvaluateFailureWithTrace(...).Decision.Action = %q, want %q", trace.Decision.Action, DecisionEscalate)
	}
	// Expected failure: BoundaryL1ToL2 and BoundaryTransitionType are not implemented on trace output yet.
	if trace.BoundaryTransition != BoundaryL1ToL2 {
		t.Fatalf("EvaluateFailureWithTrace(...).BoundaryTransition = %q, want %q", trace.BoundaryTransition, BoundaryL1ToL2)
	}
}

func TestEvaluateClassifiedFailureWithTrace_UsesClassifiedInputSourceAtPublicEntryPoint(t *testing.T) {
	now := time.Date(2026, time.February, 16, 10, 0, 0, 0, time.UTC)
	thresholds := DefaultThresholds()
	classification := PolicyClassification{Class: FailureClassWorkflow}

	trace := EvaluateClassifiedFailureWithTrace(
		classification,
		RecoveryState{Level: LevelL1, L1Attempts: 1, L1Started: now.Add(-10 * time.Second)},
		thresholds,
		now,
	)

	if trace.Classification.Class != FailureClassWorkflow {
		t.Fatalf("EvaluateClassifiedFailureWithTrace(...).Classification.Class = %q, want %q", trace.Classification.Class, FailureClassWorkflow)
	}
	if trace.Decision != (PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate}) {
		t.Fatalf("EvaluateClassifiedFailureWithTrace(...).Decision = %+v, want workflow escalation", trace.Decision)
	}
	// Expected failure: DecisionInputSourceClassifiedEntry constant does not exist yet.
	if trace.DecisionInputSource != DecisionInputSourceClassifiedEntry {
		t.Fatalf("EvaluateClassifiedFailureWithTrace(...).DecisionInputSource = %q, want %q", trace.DecisionInputSource, DecisionInputSourceClassifiedEntry)
	}
}
