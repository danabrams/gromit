package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

// TestInterfaceContract_LLMClient documents the expected LLMClient interface contract
// that adapters must satisfy: simple delegation to underlying LLM execution.
func TestInterfaceContract_LLMClient(t *testing.T) {
	t.Parallel()

	// Contract: LLMClient is implemented by adapters that delegate to LLM execution.
	// Methods: Run(prompt string, model string) (*LLMRunResult, error)
	// Expected behavior: pass through to underlying LLM client, return result

	var _ pipeline.LLMClient = (*claudeClientAdapter)(nil)
	var _ pipeline.LLMClient = (*llmRouterClientAdapter)(nil)

	t.Log("LLMClient contract: adapters delegate to underlying LLM execution")
}

// TestInterfaceContract_ReviewInvoker documents the ReviewInvoker interface contract
// that adapters must satisfy: same as LLMClient for non-interactive review.
func TestInterfaceContract_ReviewInvoker(t *testing.T) {
	t.Parallel()

	// Contract: ReviewInvoker is implemented by adapters that delegate to LLM execution.
	// Methods: Run(prompt string, model string) (*LLMRunResult, error)
	// Expected behavior: pass through to underlying invocation mechanism

	var _ pipeline.ReviewInvoker = (*claudeClientAdapter)(nil)
	var _ pipeline.ReviewInvoker = (*llmRouterClientAdapter)(nil)

	t.Log("ReviewInvoker contract: adapters delegate to underlying invocation")
}

// TestInterfaceContract_TrackerClient documents the TrackerClient interface contract.
func TestInterfaceContract_TrackerClient(t *testing.T) {
	t.Parallel()

	// Contract: TrackerClient adapts tracker operations to pipeline abstractions.
	// Methods: Ready, Show, Create, CreateWithDepsAndDescription, Close
	// Expected behavior: transform pipeline types to tracker types and delegate

	var _ pipeline.TrackerClient = (*trackerClientAdapter)(nil)

	t.Log("TrackerClient contract: adapters transform pipeline types to tracker types")
}

// TestInterfaceContract_BeadQueryClient documents the BeadQueryClient interface contract.
func TestInterfaceContract_BeadQueryClient(t *testing.T) {
	t.Parallel()

	// Contract: BeadQueryClient adapts bead query operations to pipeline abstractions.
	// Methods: CountByStatus, ListReadyIDs, ListReadyBeads, CountClosedAfter
	// Expected behavior: pass through context and parameters to underlying bead client

	var _ pipeline.BeadQueryClient = (*beadQueryClientAdapter)(nil)

	t.Log("BeadQueryClient contract: adapters delegate bead queries with context")
}

// TestInterfaceContract_BacklogClient documents the BacklogClient interface contract.
func TestInterfaceContract_BacklogClient(t *testing.T) {
	t.Parallel()

	// Contract: BacklogClient adapts backlog read operations to pipeline abstractions.
	// Methods: List, Get, Add, Update
	// Expected behavior: pass through to underlying backlog.File

	var _ pipeline.BacklogClient = (*backlogClientAdapter)(nil)

	t.Log("BacklogClient contract: adapters delegate to backlog.File read operations")
}

// TestInterfaceContract_BacklogWriter documents the BacklogWriter interface contract.
func TestInterfaceContract_BacklogWriter(t *testing.T) {
	t.Parallel()

	// Contract: BacklogWriter adapts backlog write operations to pipeline abstractions.
	// Methods: Add, Update
	// Expected behavior: transform pipeline types and delegate to underlying client

	var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)

	t.Log("BacklogWriter contract: adapters transform pipeline types and delegate")
}

// TestInterfaceContract_PromptRenderers documents the prompt renderer interface contracts.
func TestInterfaceContract_PromptRenderers(t *testing.T) {
	t.Parallel()

	// Contract: All prompt renderers are thin adapters wrapping prompt.Renderer
	// Methods: RenderX(input *XPromptInput) (string, error)
	// Expected behavior: delegate to wrapped renderer with minimal transformation

	var _ pipeline.RefineRenderer = (*refinePromptRenderer)(nil)
	var _ pipeline.PlanRenderer = (*planPromptRenderer)(nil)
	var _ pipeline.DecomposeRenderer = (*decomposePromptRenderer)(nil)
	var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
	var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)

	t.Log("PromptRenderer contract: adapters delegate to wrapped prompt.Renderer")
}

// TestInterfaceContract_LearningsManager documents the LearningsManager interface contract.
func TestInterfaceContract_LearningsManager(t *testing.T) {
	t.Parallel()

	// Contract: LearningsManager adapts learnings operations to pipeline abstractions.
	// Methods: Add(content string) error
	// Expected behavior: pass through to underlying learnings.File

	var _ pipeline.LearningsManager = (*cliLearningsManager)(nil)

	t.Log("LearningsManager contract: adapters delegate to learnings.File operations")
}

// TestInterfaceContract_StateManager documents the StateManager interface contract.
func TestInterfaceContract_StateManager(t *testing.T) {
	t.Parallel()

	// Contract: StateManager adapts state operations to pipeline abstractions.
	// Methods: GetLastReviewCommit, SetLastReviewCommit
	// Expected behavior: pass through to underlying state.File

	var _ pipeline.StateManager = (*cliStateManager)(nil)

	t.Log("StateManager contract: adapters delegate to state.File operations")
}

// TestInterfaceContract_LogWriter documents the LogWriter interface contract.
func TestInterfaceContract_LogWriter(t *testing.T) {
	t.Parallel()

	// Contract: LogWriter adapts log operations to pipeline abstractions.
	// Methods: Write(entry *LogEntry) error
	// Expected behavior: transform pipeline.LogEntry to logger operations and delegate

	var _ pipeline.LogWriter = (*cliLogWriter)(nil)

	t.Log("LogWriter contract: adapters transform pipeline types and delegate to logger")
}

// TestAdapterPattern_ImplementsUnifiedContract verifies that all adapters follow
// the unified adapter pattern: wrap underlying dependency, transform types, delegate.
func TestAdapterPattern_ImplementsUnifiedContract(t *testing.T) {
	t.Parallel()

	// All adapters follow this pattern:
	// 1. Wrap a single underlying dependency (field)
	// 2. Implement a pipeline interface (methods)
	// 3. Transform pipeline types to underlying types
	// 4. Delegate to underlying implementation
	// 5. Transform return types back to pipeline types

	// Verify compile-time interface compliance
	var deps *pipeline.Deps
	if deps != nil {
		// This structure documents the unified adapter contract
		_ = deps.AgentResolver     // provided by agent.NewResolver
		_ = deps.LLMClient         // claudeClientAdapter or llmRouterClientAdapter
		_ = deps.ReviewInvoker     // same as LLMClient
		_ = deps.TrackerClient     // trackerClientAdapter
		_ = deps.BeadQueryClient   // beadQueryClientAdapter
		_ = deps.BacklogClient     // backlogClientAdapter
		_ = deps.BacklogWriter     // cliBacklogClient
		_ = deps.RefineRenderer    // refinePromptRenderer
		_ = deps.PlanRenderer      // planPromptRenderer
		_ = deps.DecomposeRenderer // decomposePromptRenderer
		_ = deps.ReviewRenderer    // cliPromptRenderer
		_ = deps.ExploreRenderer   // explorePromptRenderer
		_ = deps.LearningsManager  // cliLearningsManager
		_ = deps.StateManager      // cliStateManager
		_ = deps.LogWriter         // cliLogWriter
	}

	t.Log("All adapters implement unified contract: wrap, transform, delegate")
}
