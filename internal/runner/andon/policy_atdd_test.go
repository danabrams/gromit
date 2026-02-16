package andon

import (
	"testing"
	"time"
)

func TestChooseNextActionPure_EnforcesL1AndL2Bounds(t *testing.T) {
	thresholds := DefaultThresholdDefinition()

	tests := []struct {
		name  string
		input PolicyInput
		want  PolicyDecision
	}{
		{
			name: "L1 below bounds stays in L1 retry",
			input: PolicyInput{
				State:      RecoveryState{Class: FailureClassTransient, Level: LevelL1, L1Attempts: thresholds.L1MaxRetries - 1},
				L1Elapsed:  thresholds.L1MaxDuration - time.Second,
				Thresholds: thresholds,
			},
			want: PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry},
		},
		{
			name: "L1 retry cap escalates to L2",
			input: PolicyInput{
				State:      RecoveryState{Class: FailureClassTransient, Level: LevelL1, L1Attempts: thresholds.L1MaxRetries},
				L1Elapsed:  time.Second,
				Thresholds: thresholds,
			},
			want: PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate},
		},
		{
			name: "L2 timebox exhaustion stops line at L3",
			input: PolicyInput{
				State:      RecoveryState{Class: FailureClassQuality, Level: LevelL2},
				L2Elapsed:  thresholds.L2MaxDuration + time.Second,
				Thresholds: thresholds,
			},
			want: PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChooseNextActionPure(tt.input)
			if got != tt.want {
				t.Fatalf("ChooseNextActionPure(%+v) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestChooseNextActionPure_UsesDefaultBoundsWhenThresholdsUnset(t *testing.T) {
	input := PolicyInput{
		State: RecoveryState{
			Class:      FailureClassTransient,
			Level:      LevelL1,
			L1Attempts: 1,
		},
		L1Elapsed: 30 * time.Second,
	}

	got := ChooseNextActionPure(input)
	want := PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry}
	if got != want {
		t.Fatalf("ChooseNextActionPure(%+v) = %+v, want %+v", input, got, want)
	}
}
