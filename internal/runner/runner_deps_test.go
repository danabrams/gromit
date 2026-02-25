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

// TestNewRunnerWithDeps_RejectsNilDeps verifies that NewRunnerWithDeps
// returns an error when passed nil deps.
func TestNewRunnerWithDeps_RejectsNilDeps(t *testing.T) {
	orch, err := NewRunnerWithDeps(nil)
	if err == nil {
		t.Error("NewRunnerWithDeps(nil) error = nil, want error")
	}
	if orch != nil {
		t.Error("NewRunnerWithDeps(nil) returned non-nil Orchestrator, want nil")
	}
}

// TestNewRunnerWithDeps_RejectsNilRouter verifies that NewRunnerWithDeps
// returns an error when deps has a nil Router.
func TestNewRunnerWithDeps_RejectsNilRouter(t *testing.T) {
	deps := &Deps{
		Router: nil,
	}

	orch, err := NewRunnerWithDeps(deps)
	if err == nil {
		t.Error("NewRunnerWithDeps with nil Router error = nil, want error")
	}
	if orch != nil {
		t.Error("NewRunnerWithDeps with nil Router returned non-nil Orchestrator, want nil")
	}
}

// TestNewRunnerWithDeps_OrchhestratorSupportsRouterOnlyConfiguration verifies that
// the Orchestrator returned from NewRunnerWithDeps is properly initialized with
// Router-only configuration and no fallback dependencies.
func TestNewRunnerWithDeps_OrchhestratorSupportsRouterOnlyConfiguration(t *testing.T) {
	router := &provider.Router{}
	deps := &Deps{
		Router: router,
	}

	orch, err := NewRunnerWithDeps(deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps error = %v, want nil", err)
	}

	// Verify the Orchestrator has an empty config (Router-only, no Claude or other dependencies)
	if orch.cfg.Gate != nil || orch.cfg.Build != nil || orch.cfg.Validate != nil || orch.cfg.Epilogue != nil {
		t.Error("Orchestrator config should have nil stages for Router-only mode")
	}

	// Verify the Router is stored in deps (no fallback to other providers)
	if deps.Router != router {
		t.Error("Router reference in Deps was modified")
	}
}
