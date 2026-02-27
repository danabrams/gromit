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

// TestAdapterMethodSignatures_MatchInterfaceContracts verifies that each adapter's
// method signatures match the pipeline interface contracts exactly, ensuring proper type compatibility.
func TestAdapterMethodSignatures_MatchInterfaceContracts(t *testing.T) {
	t.Parallel()

	// This test documents the expected method signatures for each adapter's interface implementation.
	// Any changes to these signatures would be compile-time errors.

	// LLMClient interface: Run(prompt string, model string) (*LLMRunResult, error)
	var adapter1 pipeline.LLMClient = (*claudeClientAdapter)(nil)
	_ = adapter1

	// ReviewInvoker interface: Run(prompt string, model string) (*LLMRunResult, error)
	var adapter2 pipeline.ReviewInvoker = (*claudeClientAdapter)(nil)
	_ = adapter2

	// TrackerClient interface: Ready, Show, Create, CreateWithDepsAndDescription, Close
	var adapter3 pipeline.TrackerClient = (*trackerClientAdapter)(nil)
	_ = adapter3

	// BacklogClient interface: List, Get, Add, Update
	var adapter4 pipeline.BacklogClient = (*backlogClientAdapter)(nil)
	_ = adapter4

	// BacklogWriter interface: Add, Update
	var adapter5 pipeline.BacklogWriter = (*cliBacklogClient)(nil)
	_ = adapter5

	// RefineRenderer interface: RenderRefine(*RefinePromptInput) (string, error)
	var adapter6 pipeline.RefineRenderer = (*refinePromptRenderer)(nil)
	_ = adapter6

	// PlanRenderer interface: RenderPlan(*PlanPromptInput) (string, error)
	var adapter7 pipeline.PlanRenderer = (*planPromptRenderer)(nil)
	_ = adapter7

	// DecomposeRenderer interface: RenderDecompose(*DecomposePromptInput) (string, error)
	var adapter8 pipeline.DecomposeRenderer = (*decomposePromptRenderer)(nil)
	_ = adapter8

	// ReviewRenderer interface: RenderThoroughReview(*ThoroughReviewPromptInput) (string, error)
	var adapter9 pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
	_ = adapter9

	// ExploreRenderer interface: RenderExplore(*ExplorePromptInput) (string, error)
	var adapter10 pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)
	_ = adapter10

	// LearningsManager interface: Add(content string) error
	var adapter11 pipeline.LearningsManager = (*cliLearningsManager)(nil)
	_ = adapter11

	// StateManager interface: GetLastReviewCommit, SetLastReviewCommit
	var adapter12 pipeline.StateManager = (*cliStateManager)(nil)
	_ = adapter12

	// LogWriter interface: Write(*LogEntry) error
	var adapter13 pipeline.LogWriter = (*cliLogWriter)(nil)
	_ = adapter13

	t.Log("All adapter method signatures match their interface contracts")
}
