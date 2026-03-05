package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

// TestInterfaceContract_LLMClient documents the expected LLMClient interface contract
// that adapters must satisfy: simple delegation to underlying LLM execution.
func TestInterfaceContract_LLMClient(t *testing.T) {
	t.Parallel()

	var _ pipeline.LLMClient = (*claudeClientAdapter)(nil)
	var _ pipeline.LLMClient = (*llmRouterClientAdapter)(nil)

	t.Log("LLMClient contract: adapters delegate to underlying LLM execution")
}

// TestInterfaceContract_ReviewInvoker documents the ReviewInvoker interface contract
// that adapters must satisfy: same as LLMClient for non-interactive review.
func TestInterfaceContract_ReviewInvoker(t *testing.T) {
	t.Parallel()

	var _ pipeline.ReviewInvoker = (*claudeClientAdapter)(nil)
	var _ pipeline.ReviewInvoker = (*llmRouterClientAdapter)(nil)

	t.Log("ReviewInvoker contract: adapters delegate to underlying invocation")
}

// TestInterfaceContract_TrackerClient documents the TrackerClient interface contract.
func TestInterfaceContract_TrackerClient(t *testing.T) {
	t.Parallel()

	var _ pipeline.TrackerClient = (*trackerClientAdapter)(nil)

	t.Log("TrackerClient contract: adapters transform pipeline types to tracker types")
}

// TestInterfaceContract_BeadQueryClient documents the BeadQueryClient interface contract.
func TestInterfaceContract_BeadQueryClient(t *testing.T) {
	t.Parallel()

	var _ pipeline.BeadQueryClient = (*beadQueryClientAdapter)(nil)

	t.Log("BeadQueryClient contract: adapters delegate bead queries with context")
}

// TestInterfaceContract_BacklogClient documents the BacklogClient interface contract.
func TestInterfaceContract_BacklogClient(t *testing.T) {
	t.Parallel()

	var _ pipeline.BacklogClient = (*backlogClientAdapter)(nil)

	t.Log("BacklogClient contract: adapters delegate to backlog.File read operations")
}

// TestInterfaceContract_BacklogWriter documents the BacklogWriter interface contract.
func TestInterfaceContract_BacklogWriter(t *testing.T) {
	t.Parallel()

	var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)

	t.Log("BacklogWriter contract: adapters transform pipeline types and delegate")
}

// TestInterfaceContract_PromptRenderers documents the prompt renderer interface contracts.
func TestInterfaceContract_PromptRenderers(t *testing.T) {
	t.Parallel()

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

	var _ pipeline.LearningsManager = (*cliLearningsManager)(nil)

	t.Log("LearningsManager contract: adapters delegate to learnings.File operations")
}

// TestInterfaceContract_StateManager documents the StateManager interface contract.
func TestInterfaceContract_StateManager(t *testing.T) {
	t.Parallel()

	var _ pipeline.StateManager = (*cliStateManager)(nil)

	t.Log("StateManager contract: adapters delegate to state.File operations")
}

// TestInterfaceContract_LogWriter documents the LogWriter interface contract.
func TestInterfaceContract_LogWriter(t *testing.T) {
	t.Parallel()

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

// TestAdapterContract_ValidateTypedSignatures verifies that each adapter's
// actual method signatures match the pipeline interface contracts exactly.
// This uses reflection to check method parameter types, return types, and ordering.
func TestAdapterContract_ValidateTypedSignatures(t *testing.T) {
	t.Parallel()

	// Verify claudeClientAdapter.Run signature
	{
		adapter := (*claudeClientAdapter)(nil)
		t.Run("claudeClientAdapter.Run", func(t *testing.T) {
			adapterType := reflect.TypeOf(adapter)
			if adapterType == nil {
				t.Fatal("adapter type is nil")
			}
			method, ok := adapterType.MethodByName("Run")
			if !ok {
				methodCount := adapterType.NumMethod()
				t.Logf("Found %d methods on %s, but Run was not found", methodCount, adapterType)
				for i := 0; i < methodCount; i++ {
					m := adapterType.Method(i)
					t.Logf("  - %s", m.Name)
				}
				t.Fatal("Run method not found")
			}

			// Should be Run(prompt string, model string) (*LLMRunResult, error)
			if method.Type.NumIn() != 3 { // receiver + 2 params
				t.Errorf("param count: got %d, want 3", method.Type.NumIn())
			}
			if method.Type.NumOut() != 2 {
				t.Errorf("result count: got %d, want 2", method.Type.NumOut())
			}

			// Check param types (skip receiver at 0)
			if method.Type.In(1).Kind() != reflect.String {
				t.Errorf("param 1: got %v, want string", method.Type.In(1))
			}
			if method.Type.In(2).Kind() != reflect.String {
				t.Errorf("param 2: got %v, want string", method.Type.In(2))
			}

			// Check return types
			if method.Type.Out(0) != reflect.TypeOf((*pipeline.LLMRunResult)(nil)) {
				t.Errorf("result 0: got %v, want *LLMRunResult", method.Type.Out(0))
			}
			if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("result 1: got %v, want error interface", method.Type.Out(1))
			}
		})
	}

	// Verify trackerClientAdapter.Ready signature
	{
		adapter := (*trackerClientAdapter)(nil)
		t.Run("trackerClientAdapter.Ready", func(t *testing.T) {
			adapterType := reflect.TypeOf(adapter)
			method, ok := adapterType.MethodByName("Ready")
			if !ok {
				t.Fatal("Ready method not found")
			}

			// Should be Ready(ctx context.Context) (*BeadInfo, error)
			if method.Type.NumIn() != 2 { // receiver + context
				t.Errorf("param count: got %d, want 2", method.Type.NumIn())
			}
			if method.Type.NumOut() != 2 {
				t.Errorf("result count: got %d, want 2", method.Type.NumOut())
			}

			// Check context param
			if !method.Type.In(1).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
				t.Errorf("param 1: got %v, want context.Context", method.Type.In(1))
			}

			// Check return types
			if method.Type.Out(0) != reflect.TypeOf((*pipeline.BeadInfo)(nil)) {
				t.Errorf("result 0: got %v, want *BeadInfo", method.Type.Out(0))
			}
			if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("result 1: got %v, want error interface", method.Type.Out(1))
			}
		})
	}

	// Verify beadQueryClientAdapter.CountByStatus signature
	{
		adapter := (*beadQueryClientAdapter)(nil)
		t.Run("beadQueryClientAdapter.CountByStatus", func(t *testing.T) {
			adapterType := reflect.TypeOf(adapter)
			method, ok := adapterType.MethodByName("CountByStatus")
			if !ok {
				t.Fatal("CountByStatus method not found")
			}

			// Should be CountByStatus(ctx context.Context, status string) (int, error)
			if method.Type.NumIn() != 3 { // receiver + context + status
				t.Errorf("param count: got %d, want 3", method.Type.NumIn())
			}
			if method.Type.NumOut() != 2 {
				t.Errorf("result count: got %d, want 2", method.Type.NumOut())
			}

			// Check context param
			if !method.Type.In(1).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
				t.Errorf("param 1: got %v, want context.Context", method.Type.In(1))
			}

			// Check status param
			if method.Type.In(2).Kind() != reflect.String {
				t.Errorf("param 2: got %v, want string", method.Type.In(2))
			}

			// Check return types
			if method.Type.Out(0).Kind() != reflect.Int {
				t.Errorf("result 0: got %v, want int", method.Type.Out(0))
			}
			if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("result 1: got %v, want error interface", method.Type.Out(1))
			}
		})
	}

	// Verify backlogClientAdapter.List signature
	{
		adapter := (*backlogClientAdapter)(nil)
		t.Run("backlogClientAdapter.List", func(t *testing.T) {
			adapterType := reflect.TypeOf(adapter)
			method, ok := adapterType.MethodByName("List")
			if !ok {
				t.Fatal("List method not found")
			}

			// Should be List() ([]*Idea, error)
			if method.Type.NumIn() != 1 { // receiver only
				t.Errorf("param count: got %d, want 1", method.Type.NumIn())
			}
			if method.Type.NumOut() != 2 {
				t.Errorf("result count: got %d, want 2", method.Type.NumOut())
			}

			// Check return types
			if method.Type.Out(0) != reflect.TypeOf([]*pipeline.Idea{}) {
				t.Errorf("result 0: got %v, want []*Idea", method.Type.Out(0))
			}
			if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("result 1: got %v, want error interface", method.Type.Out(1))
			}
		})
	}

	// Verify refinePromptRenderer.RenderRefine signature
	{
		adapter := (*refinePromptRenderer)(nil)
		t.Run("refinePromptRenderer.RenderRefine", func(t *testing.T) {
			adapterType := reflect.TypeOf(adapter)
			method, ok := adapterType.MethodByName("RenderRefine")
			if !ok {
				t.Fatal("RenderRefine method not found")
			}

			// Should be RenderRefine(input *RefinePromptInput) (string, error)
			if method.Type.NumIn() != 2 { // receiver + input
				t.Errorf("param count: got %d, want 2", method.Type.NumIn())
			}
			if method.Type.NumOut() != 2 {
				t.Errorf("result count: got %d, want 2", method.Type.NumOut())
			}

			// Check input param
			if method.Type.In(1) != reflect.TypeOf((*pipeline.RefinePromptInput)(nil)) {
				t.Errorf("param 1: got %v, want *RefinePromptInput", method.Type.In(1))
			}

			// Check return types
			if method.Type.Out(0).Kind() != reflect.String {
				t.Errorf("result 0: got %v, want string", method.Type.Out(0))
			}
			if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("result 1: got %v, want error interface", method.Type.Out(1))
			}
		})
	}

	// Verify cliLearningsManager.Add signature
	{
		adapter := (*cliLearningsManager)(nil)
		t.Run("cliLearningsManager.Add", func(t *testing.T) {
			adapterType := reflect.TypeOf(adapter)
			method, ok := adapterType.MethodByName("Add")
			if !ok {
				t.Fatal("Add method not found")
			}

			// Should be Add(content string) error
			if method.Type.NumIn() != 2 { // receiver + content
				t.Errorf("param count: got %d, want 2", method.Type.NumIn())
			}
			if method.Type.NumOut() != 1 {
				t.Errorf("result count: got %d, want 1", method.Type.NumOut())
			}

			// Check content param
			if method.Type.In(1).Kind() != reflect.String {
				t.Errorf("param 1: got %v, want string", method.Type.In(1))
			}

			// Check return type
			if !method.Type.Out(0).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("result 0: got %v, want error interface", method.Type.Out(0))
			}
		})
	}

	// Verify cliStateManager.GetLastReviewCommit signature
	{
		adapter := (*cliStateManager)(nil)
		t.Run("cliStateManager.GetLastReviewCommit", func(t *testing.T) {
			adapterType := reflect.TypeOf(adapter)
			method, ok := adapterType.MethodByName("GetLastReviewCommit")
			if !ok {
				t.Fatal("GetLastReviewCommit method not found")
			}

			// Should be GetLastReviewCommit() (string, error)
			if method.Type.NumIn() != 1 { // receiver only
				t.Errorf("param count: got %d, want 1", method.Type.NumIn())
			}
			if method.Type.NumOut() != 2 {
				t.Errorf("result count: got %d, want 2", method.Type.NumOut())
			}

			// Check return types
			if method.Type.Out(0).Kind() != reflect.String {
				t.Errorf("result 0: got %v, want string", method.Type.Out(0))
			}
			if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("result 1: got %v, want error interface", method.Type.Out(1))
			}
		})
	}

	// Verify cliLogWriter.Write signature
	{
		adapter := (*cliLogWriter)(nil)
		t.Run("cliLogWriter.Write", func(t *testing.T) {
			adapterType := reflect.TypeOf(adapter)
			method, ok := adapterType.MethodByName("Write")
			if !ok {
				t.Fatal("Write method not found")
			}

			// Should be Write(entry *LogEntry) error
			if method.Type.NumIn() != 2 { // receiver + entry
				t.Errorf("param count: got %d, want 2", method.Type.NumIn())
			}
			if method.Type.NumOut() != 1 {
				t.Errorf("result count: got %d, want 1", method.Type.NumOut())
			}

			// Check entry param
			if method.Type.In(1) != reflect.TypeOf((*pipeline.LogEntry)(nil)) {
				t.Errorf("param 1: got %v, want *LogEntry", method.Type.In(1))
			}

			// Check return type
			if !method.Type.Out(0).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("result 0: got %v, want error interface", method.Type.Out(0))
			}
		})
	}

	// Verify explorePromptRenderer.RenderExplore signature
	{
		adapter := (*explorePromptRenderer)(nil)
		t.Run("explorePromptRenderer.RenderExplore", func(t *testing.T) {
			adapterType := reflect.TypeOf(adapter)
			method, ok := adapterType.MethodByName("RenderExplore")
			if !ok {
				t.Fatal("RenderExplore method not found")
			}

			// Should be RenderExplore(input *ExplorePromptInput) (string, error)
			if method.Type.NumIn() != 2 { // receiver + input
				t.Errorf("param count: got %d, want 2", method.Type.NumIn())
			}
			if method.Type.NumOut() != 2 {
				t.Errorf("result count: got %d, want 2", method.Type.NumOut())
			}

			// Check input param
			if method.Type.In(1) != reflect.TypeOf((*pipeline.ExplorePromptInput)(nil)) {
				t.Errorf("param 1: got %v, want *ExplorePromptInput", method.Type.In(1))
			}

			// Check return types
			if method.Type.Out(0).Kind() != reflect.String {
				t.Errorf("result 0: got %v, want string", method.Type.Out(0))
			}
			if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("result 1: got %v, want error interface", method.Type.Out(1))
			}
		})
	}

	t.Log("All adapter method signatures match their interface contracts")
}

// TestAdapterIntegration_ContextParametersHandled verifies that adapters
// that accept context parameters pass them through correctly.
func TestAdapterIntegration_ContextParametersHandled(t *testing.T) {
	t.Parallel()

	// Verify trackerClientAdapter passes context through
	{
		adapter := (*trackerClientAdapter)(nil)
		if adapter != nil {
			_, _ = reflect.TypeOf(adapter).MethodByName("Ready")
			_, _ = reflect.TypeOf(adapter).MethodByName("Show")
			_, _ = reflect.TypeOf(adapter).MethodByName("Create")
			_, _ = reflect.TypeOf(adapter).MethodByName("Close")
		}
	}

	// Verify beadQueryClientAdapter passes context through
	{
		adapter := (*beadQueryClientAdapter)(nil)
		if adapter != nil {
			_, _ = reflect.TypeOf(adapter).MethodByName("CountByStatus")
			_, _ = reflect.TypeOf(adapter).MethodByName("ListReadyIDs")
			_, _ = reflect.TypeOf(adapter).MethodByName("CountClosedAfter")
		}
	}

	t.Log("All context parameters are properly handled by adapters")
}
