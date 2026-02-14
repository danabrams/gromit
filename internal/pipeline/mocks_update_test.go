package pipeline

import (
	"testing"
)

// TestMocks_SatisfyTypedInterfaces verifies all mock types implement pipeline interfaces with typed signatures
// Expected failure: testPromptRenderer methods still use interface{} instead of typed input structs (RefinePromptInput, PlanPromptInput, DecomposePromptInput, ExplorePromptInput)
func TestMocks_SatisfyTypedInterfaces(t *testing.T) {
	// Verify mocks implement interfaces with typed signatures
	var _ ClaudeClient = (*testClaudeClient)(nil)
	var _ BeadClient = (*testBeadClient)(nil)
	var _ PromptRenderer = (*testPromptRenderer)(nil)
	var _ LogWriter = (*testLogWriter)(nil)

	// Specifically verify PromptRenderer has typed methods
	renderer := &testPromptRenderer{}

	// These calls should compile with typed inputs (not interface{})
	_, err := renderer.RenderRefine(&RefinePromptInput{})
	if err != nil {
		// Expected - mock returns error
	}

	_, err = renderer.RenderPlan(&PlanPromptInput{})
	if err != nil {
		// Expected - mock returns error
	}

	_, err = renderer.RenderDecompose(&DecomposePromptInput{})
	if err != nil {
		// Expected - mock returns error
	}

	_, err = renderer.RenderExplore(&ExplorePromptInput{})
	if err != nil {
		// Expected - mock returns error
	}
}

// TestDecomposeAcceptanceMocks_UseTypedStructs verifies decompose test mocks return typed structs not interface{}
// Expected failure: decomposeAcceptanceClaudeClient.Run returns interface{} instead of *ClaudeRunResult
func TestDecomposeAcceptanceMocks_UseTypedStructs(t *testing.T) {
	// Verify Claude mock returns ClaudeRunResult
	claudeMock := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output:   "test output",
			}, nil
		},
	}

	result, err := claudeMock.Run("test", "opus")
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Should be able to access typed fields directly
	if !result.Success {
		t.Errorf("result.Success = false, want true")
	}
	if result.ExitCode != 0 {
		t.Errorf("result.ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Output != "test output" {
		t.Errorf("result.Output = %q, want 'test output'", result.Output)
	}

	// Verify Bead mock returns BeadInfo
	beadMock := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{
				ID:       "bead-123",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}, nil
		},
	}

	beadResult, err := beadMock.CreateWithDepsAndDescription("Test", 1, []string{"test"}, []string{"criterion"}, []string{"dep-1"}, "desc")
	if err != nil {
		t.Fatalf("CreateWithDepsAndDescription() failed: %v", err)
	}

	// Should be able to access typed fields directly
	if beadResult.ID != "bead-123" {
		t.Errorf("beadResult.ID = %q, want 'bead-123'", beadResult.ID)
	}
	if beadResult.Title != "Test" {
		t.Errorf("beadResult.Title = %q, want 'Test'", beadResult.Title)
	}
}

// TestReviewAcceptanceMocks_UseTypedStructs verifies review test mocks use typed structs
// Expected failure: reviewAcceptanceMockClaudeClient.runFunc signature needs to match ClaudeClient interface (no timeout parameter)
func TestReviewAcceptanceMocks_UseTypedStructs(t *testing.T) {
	// Verify Claude mock returns ClaudeRunResult
	// Note: The mock's runFunc should match the ClaudeClient.Run signature exactly
	claudeMock := &reviewAcceptanceMockClaudeClient{}

	result, err := claudeMock.Run("test", "sonnet")
	if err != nil {
		// Expected - mock may not be fully implemented
	}

	// When properly implemented, result should be typed
	if result != nil {
		// Should be able to access typed fields directly
		_ = result.Success
		_ = result.ExitCode
		_ = result.Output
	}

	// Verify PromptRenderer mock uses typed input
	rendererMock := &reviewAcceptanceMockPromptRenderer{
		renderThoroughReviewFunc: func(input *ThoroughReviewPromptInput) (string, error) {
			// Should receive typed struct with FromCommit and Diff fields
			if input.FromCommit == "" {
				return "", nil
			}
			return "# Review for " + input.FromCommit, nil
		},
	}

	prompt, err := rendererMock.RenderThoroughReview(&ThoroughReviewPromptInput{
		FromCommit: "abc123",
		Diff:       "diff content",
	})
	if err != nil {
		t.Fatalf("RenderThoroughReview() failed: %v", err)
	}

	if prompt != "# Review for abc123" {
		t.Errorf("prompt = %q, want to contain commit", prompt)
	}
}

// TestDecomposeAcceptanceBeadDef_DoesNotExist verifies the temporary struct is deleted
// Expected failure: decomposeAcceptanceBeadDef struct still exists in decompose_test.go at line 984
func TestDecomposeAcceptanceBeadDef_DoesNotExist(t *testing.T) {
	// This test references decomposeAcceptanceBeadDef which should be deleted
	// The test will fail to compile once the struct is removed (which is the goal)

	// Verify the struct is gone by trying to use it
	var _ decomposeAcceptanceBeadDef

	// After implementation, this line will cause a compile error: "undefined: decomposeAcceptanceBeadDef"
	// That compile error is the proof that the struct was deleted successfully
}

// TestDecomposeMocks_UseBeadInfoDirectly verifies decompose mocks use BeadInfo not custom struct
// Expected failure: decomposeAcceptanceBeadClient might still use decomposeAcceptanceBeadDef internally
func TestDecomposeMocks_UseBeadInfoDirectly(t *testing.T) {
	// Verify mocks use BeadInfo, not decomposeAcceptanceBeadDef
	beadMock := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			// Should return *BeadInfo directly, not wrap in custom struct
			return &BeadInfo{
				ID:       "bead-1",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}, nil
		},
	}

	result, err := beadMock.CreateWithDepsAndDescription("Task", 1, nil, nil, nil, "")
	if err != nil {
		t.Fatalf("CreateWithDepsAndDescription() failed: %v", err)
	}

	// Result should be *BeadInfo with direct field access (no type assertion needed)
	if result.ID != "bead-1" {
		t.Errorf("ID = %q, want 'bead-1'", result.ID)
	}
	if result.Title != "Task" {
		t.Errorf("Title = %q, want 'Task'", result.Title)
	}
}

// TestPromptRenderer_AcceptsTypedInputStructs verifies PromptRenderer methods accept typed structs
// Expected failure: PromptRenderer interface methods still use interface{} instead of typed input structs
func TestPromptRenderer_AcceptsTypedInputStructs(t *testing.T) {
	// Create a mock renderer that expects typed inputs
	renderer := &testPromptRenderer{}

	// RenderRefine should accept RefinePromptInput
	refineInput := &RefinePromptInput{}
	_, err := renderer.RenderRefine(refineInput)
	if err != nil {
		// Expected - mock not implemented
	}

	// RenderPlan should accept PlanPromptInput
	planInput := &PlanPromptInput{}
	_, err = renderer.RenderPlan(planInput)
	if err != nil {
		// Expected - mock not implemented
	}

	// RenderDecompose should accept DecomposePromptInput
	decomposeInput := &DecomposePromptInput{}
	_, err = renderer.RenderDecompose(decomposeInput)
	if err != nil {
		// Expected - mock not implemented
	}

	// RenderExplore should accept ExplorePromptInput
	exploreInput := &ExplorePromptInput{}
	_, err = renderer.RenderExplore(exploreInput)
	if err != nil {
		// Expected - mock not implemented
	}

	// RenderThoroughReview already accepts typed input
	reviewInput := &ThoroughReviewPromptInput{
		FromCommit: "abc123",
		Diff:       "diff",
	}
	_, err = renderer.RenderThoroughReview(reviewInput)
	if err != nil {
		// Expected - mock not implemented
	}
}

// TestClaudeClient_ReturnsTypedStruct verifies ClaudeClient.Run returns *ClaudeRunResult
// Expected failure: ClaudeClient interface or mocks return interface{} instead of *ClaudeRunResult
func TestClaudeClient_ReturnsTypedStruct(t *testing.T) {
	client := &testClaudeClient{}

	// Run should return (*ClaudeRunResult, error) not (interface{}, error)
	result, err := client.Run("prompt", "opus")

	// Should be able to check if result is nil without type assertion
	if result != nil {
		t.Errorf("expected nil result from empty mock, got %v", result)
	}

	if err != nil {
		// Mock returns nil, nil - no error expected
	}

	// Test with actual implementation
	client2 := &testClaudeClient{}
	result2, err2 := client2.Run("test", "haiku")
	if err2 != nil {
		t.Fatalf("Run() failed: %v", err2)
	}

	// Should be able to access typed fields without casting
	if result2 == nil {
		// Expected from current mock
	} else {
		// If implementation were complete, we could access fields directly:
		_ = result2.Success
		_ = result2.ExitCode
		_ = result2.Output
	}
}

// TestBeadClient_ReturnsTypedStruct verifies BeadClient methods return *BeadInfo
// Expected failure: BeadClient interface or mocks return interface{} instead of *BeadInfo
func TestBeadClient_ReturnsTypedStruct(t *testing.T) {
	client := &testBeadClient{}

	// Ready should return (*BeadInfo, error) not (interface{}, error)
	result, err := client.Ready()
	if result != nil {
		t.Errorf("expected nil result from empty mock, got %v", result)
	}
	if err != nil {
		// Mock returns nil, nil
	}

	// Show should return (*BeadInfo, error) not (interface{}, error)
	showResult, err := client.Show("bead-1")
	if showResult != nil {
		t.Errorf("expected nil result from empty mock, got %v", showResult)
	}
	if err != nil {
		// Mock returns nil, nil
	}

	// Create should return (*BeadInfo, error) not (interface{}, error)
	createResult, err := client.Create("Task", 1, []string{"label"}, []string{"output"})
	if createResult != nil {
		t.Errorf("expected nil result from empty mock, got %v", createResult)
	}
	if err != nil {
		// Mock returns nil, nil
	}

	// CreateWithDepsAndDescription should return (*BeadInfo, error) not (interface{}, error)
	createWithDepsResult, err := client.CreateWithDepsAndDescription("Task", 1, []string{"label"}, []string{"criterion"}, []string{"dep"}, "desc")
	if createWithDepsResult != nil {
		t.Errorf("expected nil result from empty mock, got %v", createWithDepsResult)
	}
	if err != nil {
		// Mock returns nil, nil
	}

	// All methods should return *BeadInfo directly, no type assertions needed
}

// TestMocksUpdate_LogWriterAcceptsAny verifies LogWriter.Write uses any parameter (as documented exception)
// Expected failure: This test verifies LogWriter.Write accepts any type as per the documented exception
func TestMocksUpdate_LogWriterAcceptsAny(t *testing.T) {
	writer := &testLogWriter{}

	// Write should accept any type (documented exception to "no interface{}" rule)
	err := writer.Write(map[string]interface{}{"type": "test"})
	if err != nil {
		// Mock returns nil
	}

	// Should accept any type
	err = writer.Write("string entry")
	if err != nil {
		// Mock returns nil
	}

	err = writer.Write(123)
	if err != nil {
		// Mock returns nil
	}

	// LogWriter is write-only, so any is appropriate
}

// TestAllMocks_NoTypeAssertionsNeeded verifies all mock returns are typed and need no assertions
// Expected failure: Mocks return interface{} requiring runtime type assertions instead of concrete types
func TestAllMocks_NoTypeAssertionsNeeded(t *testing.T) {
	// Test ClaudeClient returns typed result
	claudeMock := &testClaudeClient{}
	claudeResult, _ := claudeMock.Run("test", "opus")
	if claudeResult != nil {
		// Direct field access without type assertion
		_ = claudeResult.Success
		_ = claudeResult.ExitCode
		_ = claudeResult.Output
	}

	// Test BeadClient returns typed results
	beadMock := &testBeadClient{}

	readyResult, _ := beadMock.Ready()
	if readyResult != nil {
		// Direct field access without type assertion
		_ = readyResult.ID
		_ = readyResult.Title
		_ = readyResult.Priority
		_ = readyResult.Labels
	}

	showResult, _ := beadMock.Show("bead-1")
	if showResult != nil {
		_ = showResult.ID
		_ = showResult.Title
	}

	createResult, _ := beadMock.Create("Task", 1, []string{"label"}, nil)
	if createResult != nil {
		_ = createResult.ID
		_ = createResult.Priority
	}

	createWithDepsResult, _ := beadMock.CreateWithDepsAndDescription("Task", 1, nil, nil, nil, "")
	if createWithDepsResult != nil {
		_ = createWithDepsResult.ID
		_ = createWithDepsResult.Labels
	}

	// Test PromptRenderer accepts typed inputs (will fail until types exist)
	rendererMock := &testPromptRenderer{}

	refineInput := &RefinePromptInput{}
	_, _ = rendererMock.RenderRefine(refineInput)

	planInput := &PlanPromptInput{}
	_, _ = rendererMock.RenderPlan(planInput)

	decomposeInput := &DecomposePromptInput{}
	_, _ = rendererMock.RenderDecompose(decomposeInput)

	exploreInput := &ExplorePromptInput{}
	_, _ = rendererMock.RenderExplore(exploreInput)

	reviewInput := &ThoroughReviewPromptInput{FromCommit: "abc", Diff: "diff"}
	_, _ = rendererMock.RenderThoroughReview(reviewInput)
}

// TestReviewMocks_NoTimeoutInRunFunc verifies review test mocks match interface signature
// Expected failure: reviewAcceptanceMockClaudeClient.runFunc has timeout parameter but ClaudeClient.Run does not
func TestReviewMocks_NoTimeoutInRunFunc(t *testing.T) {
	// The reviewAcceptanceMockClaudeClient.runFunc should match ClaudeClient.Run signature
	// ClaudeClient.Run is: Run(prompt string, model string) (*ClaudeRunResult, error)
	// The mock's runFunc should NOT have a timeout parameter

	// This mock should compile without timeout parameter
	mockClaude := &reviewAcceptanceMockClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output:   "test",
			}, nil
		},
	}

	result, err := mockClaude.Run("prompt", "opus")
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify typed result
	if !result.Success {
		t.Errorf("result.Success = false, want true")
	}
	if result.Output != "test" {
		t.Errorf("result.Output = %q, want 'test'", result.Output)
	}
}
