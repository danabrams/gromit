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
