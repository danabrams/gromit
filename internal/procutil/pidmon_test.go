package procutil

import (
	"testing"
)

func TestPIDPressureLive(t *testing.T) {
	current, max, err := PIDPressure()
	if err != nil {
		t.Skipf("cgroup PID files not available in this environment: %v", err)
	}
	if current <= 0 {
		t.Errorf("current = %d, want > 0 (at least this process)", current)
	}
	// max == 0 means unlimited, which is valid.
	if max < 0 {
		t.Errorf("max = %d, want >= 0", max)
	}
}
