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

// TestAdapterWiring_ImplementsCompletePipelineDepsContract verifies that the complete
// pipeline.Deps dependency contract is properly formalized with adapters available for each field.
// This documents the adapter wiring requirements for pipeline.Deps construction.
func TestAdapterWiring_ImplementsCompletePipelineDepsContract(t *testing.T) {
	t.Parallel()

	// This test formalizes the complete pipeline.Deps contract:
	// Each field in pipeline.Deps has a corresponding adapter that implements its interface.
	// The wiring is verified at compile-time through type assertions.

	// The pipeline.Deps contract consists of:
	// 1. AgentResolver - agent.NewResolver(cfg) -> pipeline.AgentResolver
	// 2. LLMClient - claudeClientAdapter or llmRouterClientAdapter
	// 3. ReviewInvoker - claudeClientAdapter or llmRouterClientAdapter
	// 4. TrackerClient - trackerClientAdapter wrapping tracker.Client
	// 5. BacklogClient - backlogClientAdapter wrapping backlog.File
	// 6. BacklogWriter - cliBacklogClient wrapping bead.Client
	// 7. RefineRenderer - refinePromptRenderer wrapping prompt.Renderer
	// 8. PlanRenderer - planPromptRenderer wrapping prompt.Renderer
	// 9. DecomposeRenderer - decomposePromptRenderer wrapping prompt.Renderer
	// 10. ReviewRenderer - cliPromptRenderer wrapping prompt.Renderer
	// 11. ExploreRenderer - explorePromptRenderer wrapping prompt.Renderer
	// 12. LearningsManager - cliLearningsManager wrapping learnings operations
	// 13. StateManager - cliStateManager wrapping state.File operations
	// 14. LogWriter - cliLogWriter wrapping logger operations

	// Verify adapters are available for all Deps fields through type assignments
	var adapter1 pipeline.LLMClient = (*claudeClientAdapter)(nil)
	var adapter2 pipeline.ReviewInvoker = (*claudeClientAdapter)(nil)
	var adapter3 pipeline.TrackerClient = (*trackerClientAdapter)(nil)
	var adapter4 pipeline.BacklogClient = (*backlogClientAdapter)(nil)
	var adapter5 pipeline.BacklogWriter = (*cliBacklogClient)(nil)
	var adapter6 pipeline.RefineRenderer = (*refinePromptRenderer)(nil)
	var adapter7 pipeline.PlanRenderer = (*planPromptRenderer)(nil)
	var adapter8 pipeline.DecomposeRenderer = (*decomposePromptRenderer)(nil)
	var adapter9 pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
	var adapter10 pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)
	var adapter11 pipeline.LearningsManager = (*cliLearningsManager)(nil)
	var adapter12 pipeline.StateManager = (*cliStateManager)(nil)
	var adapter13 pipeline.LogWriter = (*cliLogWriter)(nil)

	_ = adapter1
	_ = adapter2
	_ = adapter3
	_ = adapter4
	_ = adapter5
	_ = adapter6
	_ = adapter7
	_ = adapter8
	_ = adapter9
	_ = adapter10
	_ = adapter11
	_ = adapter12
	_ = adapter13

	t.Log("pipeline.Deps contract is fully formalized with all adapters available")
}
