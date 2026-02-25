package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/runner/execution"
)

// TestSyncWriter_ImplementsExecutionOverwriteWriter verifies that syncWriter implements
// the execution.OverwriteWriter interface.
// This is a compile-time check that fails if the implementation doesn't match.
func TestSyncWriter_ImplementsExecutionOverwriteWriter(t *testing.T) {
	var _ execution.OverwriteWriter = (*syncWriter)(nil)
}
