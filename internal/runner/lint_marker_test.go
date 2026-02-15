package runner

import "testing"

// TestRunnerLintBaselineMarkerExists verifies that the sentinel constant
// exists to indicate lint baseline has been cleaned.
func TestRunnerLintBaselineMarkerExists(t *testing.T) {
	// This test will fail until the marker is defined
	_ = RunnerLintBaselineAcceptanceMarker
}
