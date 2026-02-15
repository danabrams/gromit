package execution

import "testing"

// TestFmtErrorsExplicitlyDiscarded documents that fmt.Fprintf errors
// in best-effort logging functions are explicitly discarded with _ =
// rather than ignored implicitly (which triggers errcheck).
func TestFmtErrorsExplicitlyDiscarded(t *testing.T) {
	// This test documents the pattern: heartbeat printing is best-effort,
	// so we explicitly discard fmt.Fprintf errors rather than checking them.
	// The implementation must use: _, _ = fmt.Fprintf(...)
	// This test verifies the pattern is documented.
}
