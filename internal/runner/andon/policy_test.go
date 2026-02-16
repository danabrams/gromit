package andon

import (
	"testing"
	"time"
)

// TestDefaultThresholds_SpecAligned verifies the default Andon bounds used by the
// policy for L1/L2 autonomy.
func TestDefaultThresholds_SpecAligned(t *testing.T) {
	thresholds := DefaultThresholds()

	if thresholds.L1MaxRetries != 2 {
		t.Fatalf("L1MaxRetries = %d, want 2", thresholds.L1MaxRetries)
	}
	if thresholds.L1MaxDuration != 2*time.Minute {
		t.Fatalf("L1MaxDuration = %v, want %v", thresholds.L1MaxDuration, 2*time.Minute)
	}
	if thresholds.L2MaxDuration != 15*time.Minute {
		t.Fatalf("L2MaxDuration = %v, want %v", thresholds.L2MaxDuration, 15*time.Minute)
	}
	if thresholds.MaxAssumptions != 2 {
		t.Fatalf("MaxAssumptions = %d, want 2", thresholds.MaxAssumptions)
	}
}

// TestLevels_IncludeL1ToL4 verifies all Andon levels are represented.
func TestLevels_IncludeL1ToL4(t *testing.T) {
	levels := []AndonLevel{LevelL1, LevelL2, LevelL3, LevelL4}

	if len(levels) != 4 {
		t.Fatalf("levels length = %d, want 4", len(levels))
	}

	for i, level := range levels {
		if level == "" {
			t.Fatalf("levels[%d] is empty", i)
		}
	}
}

// TestClassifyFailure_SupportsAllAndonClasses verifies class routing entry-point
// coverage for all Andon failure classes in the spec.
func TestClassifyFailure_SupportsAllAndonClasses(t *testing.T) {
	tests := []struct {
		name   string
		signal FailureSignal
		want   FailureClass
	}{
		{
			name: "transient class",
			signal: FailureSignal{
				Kind:   FailureKindTimeout,
				Output: "context deadline exceeded",
			},
			want: FailureClassTransient,
		},
		{
			name: "workflow class",
			signal: FailureSignal{
				Kind:   FailureKindWorkflow,
				Output: "bd sync required before close",
			},
			want: FailureClassWorkflow,
		},
		{
			name: "quality class",
			signal: FailureSignal{
				Kind:   FailureKindQualityGate,
				Output: "go test ./... failed",
			},
			want: FailureClassQuality,
		},
		{
			name: "intent class",
			signal: FailureSignal{
				Kind:   FailureKindAmbiguousIntent,
				Output: "spec leaves behavior undefined",
			},
			want: FailureClassIntent,
		},
		{
			name: "data class",
			signal: FailureSignal{
				Kind:   FailureKindIntegrity,
				Output: "state divergence detected",
			},
			want: FailureClassData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyFailure(tt.signal)
			if got != tt.want {
				t.Fatalf("ClassifyFailure() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestChooseNextAction_EnforcesL1AndL2Bounds verifies L1 and L2 thresholds are
// represented and enforced in policy decisions.
func TestChooseNextAction_EnforcesL1AndL2Bounds(t *testing.T) {
	thresholds := DefaultThresholds()
	now := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)

	t.Run("L1 within retry/time bounds stays in L1", func(t *testing.T) {
		state := RecoveryState{
			Class:      FailureClassTransient,
			Level:      LevelL1,
			L1Attempts: 1,
			L1Started:  now.Add(-1 * time.Minute),
		}

		decision := ChooseNextAction(state, thresholds, now)
		if decision.NextLevel != LevelL1 {
			t.Fatalf("NextLevel = %q, want %q", decision.NextLevel, LevelL1)
		}
		if decision.Action != DecisionRetry {
			t.Fatalf("Action = %q, want %q", decision.Action, DecisionRetry)
		}
	})

	t.Run("L1 retry cap escalates to L2", func(t *testing.T) {
		state := RecoveryState{
			Class:      FailureClassTransient,
			Level:      LevelL1,
			L1Attempts: thresholds.L1MaxRetries,
			L1Started:  now.Add(-1 * time.Minute),
		}

		decision := ChooseNextAction(state, thresholds, now)
		if decision.NextLevel != LevelL2 {
			t.Fatalf("NextLevel = %q, want %q", decision.NextLevel, LevelL2)
		}
		if decision.Action != DecisionEscalate {
			t.Fatalf("Action = %q, want %q", decision.Action, DecisionEscalate)
		}
	})

	t.Run("L2 timebox exhaustion escalates to L3", func(t *testing.T) {
		state := RecoveryState{
			Class:     FailureClassQuality,
			Level:     LevelL2,
			L2Started: now.Add(-16 * time.Minute),
		}

		decision := ChooseNextAction(state, thresholds, now)
		if decision.NextLevel != LevelL3 {
			t.Fatalf("NextLevel = %q, want %q", decision.NextLevel, LevelL3)
		}
		if decision.Action != DecisionStopLine {
			t.Fatalf("Action = %q, want %q", decision.Action, DecisionStopLine)
		}
	})
}

// TestChooseNextAction_HasDecisionPathPerFailureClass verifies at least one policy
// decision path for each Andon failure class.
func TestChooseNextAction_HasDecisionPathPerFailureClass(t *testing.T) {
	thresholds := DefaultThresholds()
	now := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		state RecoveryState
		want  PolicyDecision
	}{
		{
			name: "Transient retries in L1 when below limits",
			state: RecoveryState{
				Class:      FailureClassTransient,
				Level:      LevelL1,
				L1Attempts: 0,
				L1Started:  now,
			},
			want: PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry},
		},
		{
			name: "Workflow escalates from L1 to L2 after deterministic repair",
			state: RecoveryState{
				Class:      FailureClassWorkflow,
				Level:      LevelL1,
				L1Attempts: 1,
				L1Started:  now,
			},
			want: PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate},
		},
		{
			name: "Quality in L2 beyond 15 minutes escalates to L3",
			state: RecoveryState{
				Class:     FailureClassQuality,
				Level:     LevelL2,
				L2Started: now.Add(-16 * time.Minute),
			},
			want: PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
		},
		{
			name: "Intent unresolved after assumption budget escalates to L3",
			state: RecoveryState{
				Class:           FailureClassIntent,
				Level:           LevelL1,
				AssumptionsUsed: thresholds.MaxAssumptions,
			},
			want: PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate},
		},
		{
			name: "Data integrity risk triggers immediate stop-line",
			state: RecoveryState{
				Class: FailureClassData,
				Level: LevelL1,
			},
			want: PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := ChooseNextAction(tt.state, thresholds, now)
			if decision.NextLevel != tt.want.NextLevel {
				t.Fatalf("NextLevel = %q, want %q", decision.NextLevel, tt.want.NextLevel)
			}
			if decision.Action != tt.want.Action {
				t.Fatalf("Action = %q, want %q", decision.Action, tt.want.Action)
			}
		})
	}
}
