package execution

import (
	"testing"
)

// TestMockOverwriteWriter_ImplementsOverwriteWriter verifies that mockOverwriteWriter
// implements the OverwriteWriter interface.
// This is a compile-time check that fails if the implementation doesn't match.
func TestMockOverwriteWriter_ImplementsOverwriteWriter(t *testing.T) {
	t.Parallel()
	var _ OverwriteWriter = (*mockOverwriteWriter)(nil)
}
