package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// TestRunnerHasRouterField verifies that the Runner struct has a router field
func TestRunnerHasRouterField(t *testing.T) {
	// Create a minimal runner with nil router
	r := &Runner{
		router: nil,
	}

	// Verify the field exists by assigning to it
	mockRouter := &provider.Router{}
	r.router = mockRouter

	if r.router != mockRouter {
		t.Errorf("Expected router field to be assignable and retrievable")
	}
}

// TestDepsHasRouterField verifies that the Deps struct has a Router field
func TestDepsHasRouterField(t *testing.T) {
	// Create a Deps struct with a Router
	mockRouter := &provider.Router{}
	deps := Deps{
		Router: mockRouter,
	}

	if deps.Router != mockRouter {
		t.Errorf("Expected Router field to be assignable and retrievable")
	}
}
