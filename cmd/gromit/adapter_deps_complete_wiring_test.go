package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
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

// TestAdapterDepsWiring_TypesMatchInterfaces verifies that each field
// in pipeline.Deps is assigned an adapter that implements the required interface.
//
// This test validates compile-time interface compatibility.
func TestAdapterDepsWiring_TypesMatchInterfaces(t *testing.T) {
	t.Parallel()

	// These are compile-time type assertions - if they fail, the code won't compile
	// This test documents the contract

	// From adapters.go
	var _ pipeline.LLMClient = (*claudeClientAdapter)(nil)
	var _ pipeline.ReviewInvoker = (*claudeClientAdapter)(nil)
	var _ pipeline.LLMClient = (*llmRouterClientAdapter)(nil)
	var _ pipeline.ReviewInvoker = (*llmRouterClientAdapter)(nil)
	var _ pipeline.TrackerClient = (*trackerClientAdapter)(nil)
	var _ pipeline.BacklogClient = (*backlogClientAdapter)(nil)
	var _ pipeline.BeadQueryClient = (*beadQueryClientAdapter)(nil)

	// From cli_adapters.go
	var _ pipeline.RefineRenderer = (*refinePromptRenderer)(nil)
	var _ pipeline.PlanRenderer = (*planPromptRenderer)(nil)
	var _ pipeline.DecomposeRenderer = (*decomposePromptRenderer)(nil)
	var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
	var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)
	var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)
	var _ pipeline.LearningsManager = (*cliLearningsManager)(nil)
	var _ pipeline.StateManager = (*cliStateManager)(nil)
	var _ pipeline.LogWriter = (*cliLogWriter)(nil)

	// AgentResolver is from agent.Resolver which should implement pipeline.AgentResolver
	t.Log("All adapter types properly implement their interface contracts")
}
