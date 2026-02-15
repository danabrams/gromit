package methodology

import "testing"

// TestExecutorLogBestEffort documents that the Executor.log() helper
// performs best-effort logging and explicitly discards write errors.
func TestExecutorLogBestEffort(t *testing.T) {
	// This test documents the pattern: log() is a best-effort helper,
	// so fmt.Fprintf errors are explicitly discarded rather than checked.
	// The implementation must use: _, _ = fmt.Fprintf(...)
}
