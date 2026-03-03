package failurephase

import "testing"

// TestPrelaunchConstantExists verifies that the Prelaunch constant is defined
// with the expected string value "prelaunch".
func TestPrelaunchConstantExists(t *testing.T) {
	t.Parallel()
	if Prelaunch != "prelaunch" {
		t.Errorf("Prelaunch = %q, want %q", Prelaunch, "prelaunch")
	}
}

func TestLocalGateConstantExists(t *testing.T) {
	t.Parallel()
	if LocalGate != "local_gate" {
		t.Errorf("LocalGate = %q, want %q", LocalGate, "local_gate")
	}
}

func TestWiringGateConstantExists(t *testing.T) {
	t.Parallel()
	if WiringGate != "wiring_gate" {
		t.Errorf("WiringGate = %q, want %q", WiringGate, "wiring_gate")
	}
}

func TestRegressionGateConstantExists(t *testing.T) {
	t.Parallel()
	if RegressionGate != "regression_gate" {
		t.Errorf("RegressionGate = %q, want %q", RegressionGate, "regression_gate")
	}
}
