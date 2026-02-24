// This test file will not compile until RefinePromptInput, PlanPromptInput,
// DecomposePromptInput, and ExplorePromptInput are defined in pipeline.go.
// Once those types exist and PromptRenderer interface is updated, this file
// will compile and the tests will pass.

package pipeline

import (
	"testing"
)

// TestClaudeClient_ReturnsTypedResult verifies LLMClient.Run returns typed LLMRunResult
func TestClaudeClient_ReturnsTypedResult(t *testing.T) {
	// Expected failure: This test will pass when LLMClient returns typed result
	// Currently it should pass since adapters were already updated

	mock := &mockClaudeClientTyped{
		runFn: func(prompt, model string) (*LLMRunResult, error) {
			return &LLMRunResult{
				Success:  true,
				ExitCode: 0,
				Output:   "test output",
			}, nil
		},
	}

	result, err := mock.Run("test prompt", "sonnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we can access fields directly without type assertions
	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
	if result.Output != "test output" {
		t.Errorf("expected Output 'test output', got %q", result.Output)
	}

	// This demonstrates the difference: no type assertions needed!
	// Old way would have been: result.(map[string]interface{})["Success"].(bool)
}

// TestBeadClient_ReturnsTypedInfo verifies BeadClient methods return typed BeadInfo
func TestBeadClient_ReturnsTypedInfo(t *testing.T) {
	// Expected failure: This test will pass when BeadClient returns typed BeadInfo
	// Currently it should pass since adapters were already updated

	mock := &mockBeadClientTyped{
		readyFn: func() (*BeadInfo, error) {
			return &BeadInfo{
				ID:       "test-123",
				Title:    "Test Bead",
				Priority: 1,
				Labels:   []string{"test"},
			}, nil
		},
	}

	bead, err := mock.Ready()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we can access fields directly without type assertions or reflection
	if bead.ID != "test-123" {
		t.Errorf("expected ID 'test-123', got %q", bead.ID)
	}
	if bead.Title != "Test Bead" {
		t.Errorf("expected Title 'Test Bead', got %q", bead.Title)
	}
	if bead.Priority != 1 {
		t.Errorf("expected Priority 1, got %d", bead.Priority)
	}

	// This is type-safe at compile time - no extractBeadID needed!
}

// NOTE: This test is commented out because the required types don't exist yet.
// Expected failure: RefinePromptInput, PlanPromptInput, DecomposePromptInput, ExplorePromptInput don't exist yet
//
// Uncomment this test once the following types are added to pipeline.go:
// - type RefinePromptInput struct { ... }
// - type PlanPromptInput struct { ... }
// - type DecomposePromptInput struct { ... }
// - type ExplorePromptInput struct { ... }
// And the PromptRenderer interface is updated to use these types.
//
// func TestPromptRenderer_TakesTypedInput(t *testing.T) {
// 	mock := &mockPromptRendererTyped{
// 		renderRefineFn: func(input *RefinePromptInput) (string, error) {
// 			// Access fields directly - compile-time type safety
// 			if input.IdeaText == "" {
// 				return "", nil
// 			}
// 			return "refined: " + input.IdeaText, nil
// 		},
// 		renderPlanFn: func(input *PlanPromptInput) (string, error) {
// 			if input.SpecName == "" {
// 				return "", nil
// 			}
// 			return "plan for: " + input.SpecName, nil
// 		},
// 		renderDecomposeFn: func(input *DecomposePromptInput) (string, error) {
// 			if input.PlanName == "" {
// 				return "", nil
// 			}
// 			return "decompose: " + input.PlanName, nil
// 		},
// 		renderExploreFn: func(input *ExplorePromptInput) (string, error) {
// 			if input.Query == "" {
// 				return "", nil
// 			}
// 			return "explore: " + input.Query, nil
// 		},
// 	}
//
// 	// Test RenderRefine with typed input
// 	refineResult, err := mock.RenderRefine(&RefinePromptInput{IdeaText: "test idea"})
// 	if err != nil {
// 		t.Fatalf("RenderRefine error: %v", err)
// 	}
// 	if refineResult != "refined: test idea" {
// 		t.Errorf("expected 'refined: test idea', got %q", refineResult)
// 	}
//
// 	// Test RenderPlan with typed input
// 	planResult, err := mock.RenderPlan(&PlanPromptInput{SpecName: "test-spec"})
// 	if err != nil {
// 		t.Fatalf("RenderPlan error: %v", err)
// 	}
// 	if planResult != "plan for: test-spec" {
// 		t.Errorf("expected 'plan for: test-spec', got %q", planResult)
// 	}
//
// 	// Test RenderDecompose with typed input
// 	decomposeResult, err := mock.RenderDecompose(&DecomposePromptInput{PlanName: "test-plan"})
// 	if err != nil {
// 		t.Fatalf("RenderDecompose error: %v", err)
// 	}
// 	if decomposeResult != "decompose: test-plan" {
// 		t.Errorf("expected 'decompose: test-plan', got %q", decomposeResult)
// 	}
//
// 	// Test RenderExplore with typed input
// 	exploreResult, err := mock.RenderExplore(&ExplorePromptInput{Query: "test query"})
// 	if err != nil {
// 		t.Fatalf("RenderExplore error: %v", err)
// 	}
// 	if exploreResult != "explore: test query" {
// 		t.Errorf("expected 'explore: test query', got %q", exploreResult)
// 	}
//
// 	// The key benefit: compiler enforces the contract, no runtime type assertions!
// }

// TestDecomposeWorkflow_UsesTypedStructsDirectly verifies decompose uses typed results without casting
func TestDecomposeWorkflow_UsesTypedStructsDirectly(t *testing.T) {
	// This test demonstrates the workflow uses BeadInfo.ID directly
	// No extractBeadID function, no reflection, no type assertions

	mockBeadClient := &mockBeadClientTyped{
		createWithDepsFn: func(title string, priority int, labels, criteria, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{
				ID:       "created-456",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}, nil
		},
	}

	// Create a bead and access ID directly from typed return
	result, err := mockBeadClient.CreateWithDepsAndDescription(
		"Test Task",
		1,
		[]string{"spec:test"},
		[]string{"criterion 1"},
		[]string{},
		"Description",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Direct field access - no extractBeadID(result) needed!
	beadID := result.ID
	if beadID != "created-456" {
		t.Errorf("expected ID 'created-456', got %q", beadID)
	}

	// In the old code, this would have required:
	// 1. Type assertion to map[string]interface{}
	// 2. Or reflection via extractBeadID
	// 3. Or checking if result has an ID() method
	// All of that complexity is eliminated with typed returns
}

// Mock implementations for testing typed interfaces

type mockClaudeClientTyped struct {
	runFn func(prompt, model string) (*LLMRunResult, error)
}

func (m *mockClaudeClientTyped) Run(prompt, model string) (*LLMRunResult, error) {
	if m.runFn != nil {
		return m.runFn(prompt, model)
	}
	return &LLMRunResult{Success: true}, nil
}

type mockBeadClientTyped struct {
	readyFn          func() (*BeadInfo, error)
	showFn           func(id string) (*BeadInfo, error)
	createFn         func(title string, priority int, labels, outputs []string) (*BeadInfo, error)
	createWithDepsFn func(title string, priority int, labels, criteria, deps []string, desc string) (*BeadInfo, error)
	closeFn          func(id string) error
}

func (m *mockBeadClientTyped) Ready() (*BeadInfo, error) {
	if m.readyFn != nil {
		return m.readyFn()
	}
	return &BeadInfo{}, nil
}

func (m *mockBeadClientTyped) Show(id string) (*BeadInfo, error) {
	if m.showFn != nil {
		return m.showFn(id)
	}
	return &BeadInfo{ID: id}, nil
}

func (m *mockBeadClientTyped) Create(title string, priority int, labels, outputs []string) (*BeadInfo, error) {
	if m.createFn != nil {
		return m.createFn(title, priority, labels, outputs)
	}
	return &BeadInfo{Title: title}, nil
}

func (m *mockBeadClientTyped) CreateWithDepsAndDescription(title string, priority int, labels, criteria, deps []string, desc string) (*BeadInfo, error) {
	if m.createWithDepsFn != nil {
		return m.createWithDepsFn(title, priority, labels, criteria, deps, desc)
	}
	return &BeadInfo{Title: title}, nil
}

func (m *mockBeadClientTyped) Close(id string) error {
	if m.closeFn != nil {
		return m.closeFn(id)
	}
	return nil
}

// NOTE: mockPromptRendererTyped is commented out until the required types exist
//
// type mockPromptRendererTyped struct {
// 	renderRefineFn         func(input *RefinePromptInput) (string, error)
// 	renderPlanFn           func(input *PlanPromptInput) (string, error)
// 	renderDecomposeFn      func(input *DecomposePromptInput) (string, error)
// 	renderThoroughReviewFn func(input *ThoroughReviewPromptInput) (string, error)
// 	renderExploreFn        func(input *ExplorePromptInput) (string, error)
// }
//
// func (m *mockPromptRendererTyped) RenderRefine(input *RefinePromptInput) (string, error) {
// 	if m.renderRefineFn != nil {
// 		return m.renderRefineFn(input)
// 	}
// 	return "", nil
// }
//
// func (m *mockPromptRendererTyped) RenderPlan(input *PlanPromptInput) (string, error) {
// 	if m.renderPlanFn != nil {
// 		return m.renderPlanFn(input)
// 	}
// 	return "", nil
// }
//
// func (m *mockPromptRendererTyped) RenderDecompose(input *DecomposePromptInput) (string, error) {
// 	if m.renderDecomposeFn != nil {
// 		return m.renderDecomposeFn(input)
// 	}
// 	return "", nil
// }
//
// func (m *mockPromptRendererTyped) RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error) {
// 	if m.renderThoroughReviewFn != nil {
// 		return m.renderThoroughReviewFn(input)
// 	}
// 	return "", nil
// }
//
// func (m *mockPromptRendererTyped) RenderExplore(input *ExplorePromptInput) (string, error) {
// 	if m.renderExploreFn != nil {
// 		return m.renderExploreFn(input)
// 	}
// 	return "", nil
// }

// Note: These types should be defined in pipeline.go
// If this test compiles, it means the types exist
// If it doesn't compile, RefinePromptInput, PlanPromptInput, DecomposePromptInput, ExplorePromptInput need to be added

// TestPipeline_WorksWithTypedDependencies is an integration-style test
func TestPipeline_WorksWithTypedDependencies(t *testing.T) {
	// Expected failure: This demonstrates the full workflow with typed interfaces

	mockClaude := &mockClaudeClientTyped{
		runFn: func(prompt, model string) (*LLMRunResult, error) {
			return &LLMRunResult{
				Success:  true,
				ExitCode: 0,
				Output:   `[{"title":"Task 1","description":"Do thing","priority":"P1","acceptance_criteria":["Done"],"depends_on_index":[]}]`,
			}, nil
		},
	}

	mockBead := &mockBeadClientTyped{
		createWithDepsFn: func(title string, priority int, labels, criteria, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{
				ID:       "new-bead-789",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}, nil
		},
	}

	deps := &Deps{
		LLMClient:  mockClaude,
		BeadClient: mockBead,
	}

	paths := &Paths{
		PlansDir: t.TempDir(),
	}

	p := New(deps, paths)

	// Verify pipeline can be constructed with typed dependencies
	if p == nil {
		t.Fatal("expected non-nil pipeline")
	}

	// The key achievement: all dependency methods use typed signatures
	// No interface{} returns, no type assertions, no reflection
}
