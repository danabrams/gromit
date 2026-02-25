package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// TestNewRunnerWithDeps_ConstructsWithRouterOnly verifies that NewRunnerWithDeps
// can construct a Runner using only a Router dependency, without requiring Claude.
func TestNewRunnerWithDeps_ConstructsWithRouterOnly(t *testing.T) {
	// Create a minimal router for testing
	router := &provider.Router{}

	deps := &Deps{
		Router: router,
	}

	orch, err := NewRunnerWithDeps(deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps(deps) error = %v, want nil", err)
	}

	if orch == nil {
		t.Error("NewRunnerWithDeps returned nil Orchestrator, want non-nil")
	}
}
