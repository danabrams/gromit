package execution

import (
	"testing"
)

// TestMockRouter_ImplementsRouter verifies that mockRouter implements the Router interface.
// This is a compile-time check that fails if the implementation doesn't match.
func TestMockRouter_ImplementsRouter(t *testing.T) {
	var _ Router = (*mockRouter)(nil)
}

// TestMockProvider_ImplementsProvider verifies that mockProvider implements the Provider interface.
// This is a compile-time check that fails if the implementation doesn't match.
func TestMockProvider_ImplementsProvider(t *testing.T) {
	var _ Provider = (*mockProvider)(nil)
}
