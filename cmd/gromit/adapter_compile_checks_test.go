package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestAdapterCompileTimeChecks verifies that adapter types implement their intended interfaces.
// This ensures all adapters have compile-time interface compliance checks via var _ declarations.
// If an adapter doesn't implement its interface, the compiler will error here first.
func TestAdapterCompileTimeChecks(t *testing.T) {
	t.Parallel()
	// These assignments verify the adapters implement their interfaces.
	// The var _ InterfaceType = (*ConcreteType)(nil) pattern in the source provides
	// clearer compile-time verification and improved IDE support.

	// Adapters in cmd/gromit/adapters.go
	var _ pipeline.LLMClient = (*claudeClientAdapter)(nil)
	var _ pipeline.ReviewInvoker = (*claudeClientAdapter)(nil)
	var _ pipeline.LLMClient = (*llmRouterClientAdapter)(nil)
	var _ pipeline.ReviewInvoker = (*llmRouterClientAdapter)(nil)
	var _ pipeline.TrackerClient = (*trackerClientAdapter)(nil)
	var _ pipeline.BacklogClient = (*backlogClientAdapter)(nil)

	// Adapters in cmd/gromit/cli_adapters.go
	var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
	var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)
	var _ pipeline.PlanRenderer = (*planPromptRenderer)(nil)
	var _ pipeline.RefineRenderer = (*refinePromptRenderer)(nil)
	var _ pipeline.DecomposeRenderer = (*decomposePromptRenderer)(nil)
	var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)
	var _ pipeline.LearningsManager = (*cliLearningsManager)(nil)
	var _ pipeline.LogWriter = (*cliLogWriter)(nil)
	var _ pipeline.StateManager = (*cliStateManager)(nil)

	// Adapters in cmd/gromit/cli_adapters.go (legacy runner)
	var _ learnings.ClaudeRunner = (*pipelineLearningsRunnerAdapter)(nil)

	t.Log("All adapter compile-time checks verified")
}
