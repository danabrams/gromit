package runner

import "testing"

// TestLogFunctionBestEffort documents that the log() helper function
// performs best-effort logging and explicitly discards write errors.
func TestLogFunctionBestEffort(t *testing.T) {
	// This test documents the pattern: log() is a best-effort helper,
	// so fmt.Fprint errors are explicitly discarded rather than checked.
	// The implementation must use: _, _ = fmt.Fprint(...)
}
