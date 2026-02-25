package execution

import (
	"testing"
)

// TestRouter_ComplianceCheckForNoop verifies that noopRouter implements Router interface.
// This is a compile-time check that fails if the implementation doesn't match.
func TestRouter_ComplianceCheckForNoop(t *testing.T) {
	var _ Router = (*noopRouter)(nil)
}

// TestProvider_ComplianceCheckForNoop verifies that noopProvider implements Provider interface.
// This is a compile-time check that fails if the implementation doesn't match.
func TestProvider_ComplianceCheckForNoop(t *testing.T) {
	var _ Provider = (*noopProvider)(nil)
}

// TestOverwriteWriter_ComplianceCheckForNoop verifies that noopOverwriteWriter implements OverwriteWriter interface.
// This is a compile-time check that fails if the implementation doesn't match.
func TestOverwriteWriter_ComplianceCheckForNoop(t *testing.T) {
	var _ OverwriteWriter = (*noopOverwriteWriter)(nil)
}
