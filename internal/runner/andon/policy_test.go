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
