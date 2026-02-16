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
