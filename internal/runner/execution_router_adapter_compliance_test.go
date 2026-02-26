package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/runner/execution"
)

// TestExecutionRouterAdapter_ImplementsExecutionRouter verifies that
// executionRouterAdapter implements the execution.Router interface.
// This is a compile-time check that fails if the implementation doesn't match.
func TestExecutionRouterAdapter_ImplementsExecutionRouter(t *testing.T) {
	t.Parallel()
	var _ execution.Router = (*executionRouterAdapter)(nil)
}
