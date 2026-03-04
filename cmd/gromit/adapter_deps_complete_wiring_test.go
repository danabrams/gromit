package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestAdapterDepsWiring_AllFieldsWiredInNewPipelineDeps verifies that
// NewPipelineDeps initializes ALL required fields of pipeline.Deps, ensuring
// no field is left nil. This is a RED test that documents the complete
// wiring contract.
//
// If this test fails, it means NewPipelineDeps is not properly wiring
// one or more required dependencies.
func TestAdapterDepsWiring_AllFieldsWiredInNewPipelineDeps(t *testing.T) {
	t.Parallel()

	deps, err := NewPipelineDeps(&config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Flags:   []string{},
			Timeout: 60,
		},
	}, ".gromit")

	if err != nil {
		t.Fatalf("NewPipelineDeps failed: %v", err)
	}

	if deps == nil {
		t.Fatal("NewPipelineDeps returned nil")
	}

	// Check all required fields are non-nil
	checks := []struct {
		name  string
		field interface{}
	}{
		{"AgentResolver", deps.AgentResolver},
		{"LLMClient", deps.LLMClient},
		{"ReviewInvoker", deps.ReviewInvoker},
		{"TrackerClient", deps.TrackerClient},
		{"BeadQueryClient", deps.BeadQueryClient},
		{"BacklogClient", deps.BacklogClient},
		{"BacklogWriter", deps.BacklogWriter},
		{"RefineRenderer", deps.RefineRenderer},
		{"PlanRenderer", deps.PlanRenderer},
		{"DecomposeRenderer", deps.DecomposeRenderer},
		{"ReviewRenderer", deps.ReviewRenderer},
		{"ExploreRenderer", deps.ExploreRenderer},
		{"LearningsManager", deps.LearningsManager},
		{"StateManager", deps.StateManager},
		{"LogWriter", deps.LogWriter},
	}

	for _, check := range checks {
		if check.field == nil {
			t.Errorf("Deps.%s is nil - not wired in NewPipelineDeps", check.name)
		}
	}

	t.Log("All pipeline.Deps fields are properly wired in NewPipelineDeps")
}
