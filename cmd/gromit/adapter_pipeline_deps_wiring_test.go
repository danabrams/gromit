package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

// TestPipelineDepsWiring_AllAdaptersImplementInterfaces verifies that all required
// pipeline.Deps interfaces have corresponding adapter implementations available for wiring.
func TestPipelineDepsWiring_AllAdaptersImplementInterfaces(t *testing.T) {
	t.Parallel()

	// Verify each pipeline.Deps field can be satisfied by an adapter type.
	// This confirms all adapters exist and implement their required interfaces.
	// These blank identifier assignments compile-time verify interface compliance.

	// AgentResolver - provided by agent.NewResolver from internal/agent package

	var _ pipeline.LLMClient = (*claudeClientAdapter)(nil)
	var _ pipeline.ReviewInvoker = (*claudeClientAdapter)(nil)
	var _ pipeline.TrackerClient = (*trackerClientAdapter)(nil)
	var _ pipeline.BacklogClient = (*backlogClientAdapter)(nil)
	var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)
	var _ pipeline.RefineRenderer = (*refinePromptRenderer)(nil)
	var _ pipeline.PlanRenderer = (*planPromptRenderer)(nil)
	var _ pipeline.DecomposeRenderer = (*decomposePromptRenderer)(nil)
	var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
	var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)
	var _ pipeline.LearningsManager = (*cliLearningsManager)(nil)
	var _ pipeline.StateManager = (*cliStateManager)(nil)
	var _ pipeline.LogWriter = (*cliLogWriter)(nil)

	t.Log("All pipeline.Deps interfaces can be satisfied by adapters")
}
