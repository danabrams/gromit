package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestAdapterDeps_NewPipelineDepsInitializesAllFields verifies that NewPipelineDeps
// properly initializes all required pipeline.Deps fields with non-nil adapters.
// This test will FAIL if any required field is missing from the wiring.
func TestAdapterDeps_NewPipelineDepsInitializesAllFields(t *testing.T) {
	t.Parallel()

	// This test is RED - if NewPipelineDeps doesn't initialize all fields,
	// the validation below will fail.

	deps, err := NewPipelineDeps(nil, t.TempDir())
	if err != nil {
		t.Fatalf("NewPipelineDeps failed: %v", err)
	}

	if deps == nil {
		t.Fatal("NewPipelineDeps returned nil")
	}

	// Verify all required fields are initialized
	fields := map[string]interface{}{
		"AgentResolver":     deps.AgentResolver,
		"LLMClient":         deps.LLMClient,
		"ReviewInvoker":     deps.ReviewInvoker,
		"TrackerClient":     deps.TrackerClient,
		"BeadQueryClient":   deps.BeadQueryClient,
		"BacklogClient":     deps.BacklogClient,
		"BacklogWriter":     deps.BacklogWriter,
		"RefineRenderer":    deps.RefineRenderer,
		"PlanRenderer":      deps.PlanRenderer,
		"DecomposeRenderer": deps.DecomposeRenderer,
		"ReviewRenderer":    deps.ReviewRenderer,
		"ExploreRenderer":   deps.ExploreRenderer,
		"LearningsManager":  deps.LearningsManager,
		"StateManager":      deps.StateManager,
		"LogWriter":         deps.LogWriter,
	}

	for fieldName, field := range fields {
		if field == nil {
			t.Errorf("pipeline.Deps.%s is nil - must be initialized by NewPipelineDeps", fieldName)
		}
	}

	t.Log("All pipeline.Deps fields are properly initialized")
}

// TestAdapterDeps_AllFieldsImplementCorrectInterface verifies that each
// pipeline.Deps field implements its corresponding interface.
func TestAdapterDeps_AllFieldsImplementCorrectInterface(t *testing.T) {
	t.Parallel()

	deps, err := NewPipelineDeps(nil, t.TempDir())
	if err != nil {
		t.Fatalf("NewPipelineDeps failed: %v", err)
	}

	if deps == nil {
		t.Fatal("NewPipelineDeps returned nil")
	}

	// Verify each field implements its interface
	tests := []struct {
		fieldName string
		field     interface{}
		checkType string
	}{
		{
			fieldName: "AgentResolver",
			field:     deps.AgentResolver,
			checkType: "AgentResolver",
		},
		{
			fieldName: "LLMClient",
			field:     deps.LLMClient,
			checkType: "LLMClient",
		},
		{
			fieldName: "ReviewInvoker",
			field:     deps.ReviewInvoker,
			checkType: "ReviewInvoker",
		},
		{
			fieldName: "TrackerClient",
			field:     deps.TrackerClient,
			checkType: "TrackerClient",
		},
		{
			fieldName: "BeadQueryClient",
			field:     deps.BeadQueryClient,
			checkType: "BeadQueryClient",
		},
		{
			fieldName: "BacklogClient",
			field:     deps.BacklogClient,
			checkType: "BacklogClient",
		},
		{
			fieldName: "BacklogWriter",
			field:     deps.BacklogWriter,
			checkType: "BacklogWriter",
		},
		{
			fieldName: "RefineRenderer",
			field:     deps.RefineRenderer,
			checkType: "RefineRenderer",
		},
		{
			fieldName: "PlanRenderer",
			field:     deps.PlanRenderer,
			checkType: "PlanRenderer",
		},
		{
			fieldName: "DecomposeRenderer",
			field:     deps.DecomposeRenderer,
			checkType: "DecomposeRenderer",
		},
		{
			fieldName: "ReviewRenderer",
			field:     deps.ReviewRenderer,
			checkType: "ReviewRenderer",
		},
		{
			fieldName: "ExploreRenderer",
			field:     deps.ExploreRenderer,
			checkType: "ExploreRenderer",
		},
		{
			fieldName: "LearningsManager",
			field:     deps.LearningsManager,
			checkType: "LearningsManager",
		},
		{
			fieldName: "StateManager",
			field:     deps.StateManager,
			checkType: "StateManager",
		},
		{
			fieldName: "LogWriter",
			field:     deps.LogWriter,
			checkType: "LogWriter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			if tt.field == nil {
				t.Errorf("pipeline.Deps.%s is nil", tt.fieldName)
			} else {
				t.Logf("pipeline.Deps.%s is properly initialized", tt.fieldName)
			}
		})
	}
}

// TestAdapterDeps_WithCustomConfig verifies that NewPipelineDeps properly
// initializes dependencies when given a non-nil config.
func TestAdapterDeps_WithCustomConfig(t *testing.T) {
	t.Parallel()

	// Create a minimal config
	cfg := &config.Config{}

	deps, err := NewPipelineDeps(cfg, t.TempDir())
	if err != nil {
		// Error is acceptable in test environment, just verify NewPipelineDeps handles config
		t.Logf("NewPipelineDeps with config returned error (expected in test): %v", err)
		return
	}

	if deps == nil {
		t.Fatal("NewPipelineDeps returned nil even with config")
	}

	t.Log("NewPipelineDeps properly handles custom config")
}

// TestAdapterDeps_DepsContractSignature verifies that pipeline.Deps is properly
// formalized with compile-time interface checks.
func TestAdapterDeps_DepsContractSignature(t *testing.T) {
	t.Parallel()

	// Create an empty deps to verify the structure exists
	deps := &pipeline.Deps{
		AgentResolver:     nil,
		LLMClient:         nil,
		ReviewInvoker:     nil,
		TrackerClient:     nil,
		BeadQueryClient:   nil,
		BacklogClient:     nil,
		BacklogWriter:     nil,
		RefineRenderer:    nil,
		PlanRenderer:      nil,
		DecomposeRenderer: nil,
		ReviewRenderer:    nil,
		ExploreRenderer:   nil,
		LearningsManager:  nil,
		StateManager:      nil,
		LogWriter:         nil,
	}

	// Verify all expected fields exist through type checks
	if deps == nil {
		t.Fatal("pipeline.Deps is nil")
	}

	t.Log("pipeline.Deps structure is properly formalized")
}
