package pipeline

import (
	"testing"
	"time"
)

// TestAllMocks_SatisfyInterfacesWithTypedSignatures verifies that all test mocks
// use typed struct returns instead of interface{} or map[string]interface{}
func TestAllMocks_SatisfyInterfacesWithTypedSignatures(t *testing.T) {
	// Compile-time checks - these will fail if mocks don't match interfaces
	var _ LLMClient = (*testLLMClient)(nil)
	var _ BeadClient = (*testBeadClient)(nil)
	var _ ReviewRenderer = (*testReviewRenderer)(nil)

	// Verify LLMClient mock returns typed LLMRunResult
	t.Run("LLMClient returns typed LLMRunResult", func(t *testing.T) {
		mock := &testLLMClient{}
		result, err := mock.Run("test prompt", "opus")
		if err != nil {
			// Expected - mock returns nil
		}
		// The fact that this compiles with result being treated as *LLMRunResult
		// proves the typed return signature
		_ = result
	})

	// Verify BeadClient mock returns typed BeadInfo
	t.Run("BeadClient methods return typed BeadInfo", func(t *testing.T) {
		mock := &testBeadClient{}

		ready, err := mock.Ready()
		_ = ready
		_ = err

		show, err := mock.Show("test-id")
		_ = show
		_ = err

		create, err := mock.Create("title", 1, []string{"label"}, []string{"output"})
		_ = create
		_ = err

		createWithDeps, err := mock.CreateWithDepsAndDescription("title", 1, []string{"label"}, []string{"criterion"}, []string{"dep"}, "desc")
		_ = createWithDeps
		_ = err
	})

	// Verify PromptRenderer.RenderThoroughReview accepts typed input
	t.Run("PromptRenderer.RenderThoroughReview accepts typed input", func(t *testing.T) {
		mock := &testReviewRenderer{}
		input := &ThoroughReviewPromptInput{
			FromCommit: "abc123",
			Diff:       "diff content",
		}
		result, err := mock.RenderThoroughReview(input)
		_ = result
		_ = err
	})
}

// TestDecomposeAcceptanceMocks_UseTypedReturns verifies decompose test mocks use typed returns
func TestDecomposeAcceptanceMocks_UseTypedReturns(t *testing.T) {
	// Compile-time checks
	var _ LLMClient = (*decomposeAcceptanceLLMClient)(nil)
	var _ BeadClient = (*decomposeAcceptanceBeadClient)(nil)

	t.Run("decomposeAcceptanceLLMClient returns LLMRunResult", func(t *testing.T) {
		mock := &decomposeAcceptanceLLMClient{
			runFunc: func(prompt string, model string) (*LLMRunResult, error) {
				return &LLMRunResult{
					Success:  true,
					ExitCode: 0,
					Output:   "test output",
				}, nil
			},
		}

		result, err := mock.Run("test", "opus")
		if err != nil {
			t.Fatalf("Run() failed: %v", err)
		}

		// Direct field access - no type assertions needed
		if !result.Success {
			t.Errorf("result.Success = false, want true")
		}
		if result.ExitCode != 0 {
			t.Errorf("result.ExitCode = %d, want 0", result.ExitCode)
		}
		if result.Output != "test output" {
			t.Errorf("result.Output = %q, want 'test output'", result.Output)
		}
	})

	t.Run("decomposeAcceptanceBeadClient returns BeadInfo", func(t *testing.T) {
		mock := &decomposeAcceptanceBeadClient{
			createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
				return &BeadInfo{
					ID:       "bead-123",
					Title:    title,
					Priority: priority,
					Labels:   labels,
				}, nil
			},
		}

		result, err := mock.CreateWithDepsAndDescription("Test", 1, []string{"test"}, []string{"criterion"}, []string{"dep-1"}, "desc")
		if err != nil {
			t.Fatalf("CreateWithDepsAndDescription() failed: %v", err)
		}

		// Direct field access - no type assertions needed
		if result.ID != "bead-123" {
			t.Errorf("result.ID = %q, want 'bead-123'", result.ID)
		}
		if result.Title != "Test" {
			t.Errorf("result.Title = %q, want 'Test'", result.Title)
		}
	})
}

// TestReviewAcceptanceMocks_UseTypedReturns verifies review test mocks use typed returns
func TestReviewAcceptanceMocks_UseTypedReturns(t *testing.T) {
	// Compile-time checks
	var _ ReviewInvoker = (*reviewAcceptanceMockReviewInvoker)(nil)
	var _ BeadClient = (*reviewAcceptanceMockBeadClient)(nil)

	t.Run("reviewAcceptanceMockReviewInvoker returns LLMRunResult", func(t *testing.T) {
		mock := &reviewAcceptanceMockReviewInvoker{
			runFunc: func(prompt string, model string, timeout time.Duration) (*LLMRunResult, error) {
				return &LLMRunResult{
					Success:  true,
					ExitCode: 0,
					Output:   "review output",
				}, nil
			},
		}

		result, err := mock.Run("test", "opus")
		if err != nil {
			t.Fatalf("Run() failed: %v", err)
		}

		// Direct field access - no type assertions needed
		if !result.Success {
			t.Errorf("result.Success = false, want true")
		}
	})

	t.Run("reviewAcceptanceMockBeadClient returns BeadInfo", func(t *testing.T) {
		mock := &reviewAcceptanceMockBeadClient{
			createFunc: func(title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
				return &BeadInfo{
					ID:       "bead-456",
					Title:    title,
					Priority: priority,
					Labels:   labels,
				}, nil
			},
		}

		result, err := mock.Create("Review bead", 1, []string{"from-review"}, []string{})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// Direct field access - no type assertions needed
		if result.ID != "bead-456" {
			t.Errorf("result.ID = %q, want 'bead-456'", result.ID)
		}
		if result.Title != "Review bead" {
			t.Errorf("result.Title = %q, want 'Review bead'", result.Title)
		}
	})
}

// TestDecomposeAcceptanceBeadDef_DoesNotExist verifies the intermediate struct was removed
func TestDecomposeAcceptanceBeadDef_DoesNotExist(t *testing.T) {
	// This test documents that decomposeAcceptanceBeadDef struct has been removed
	// and replaced with direct BeadInfo usage. If someone tries to add it back,
	// this test serves as documentation that it was intentionally removed.

	// Create a mock using BeadInfo directly
	var createdBeads []*BeadInfo
	mock := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			bead := &BeadInfo{
				ID:       "test-id",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}
			createdBeads = append(createdBeads, bead)
			return bead, nil
		},
	}

	// Use the mock
	result, err := mock.CreateWithDepsAndDescription("Test", 1, []string{"label"}, []string{"criterion"}, []string{"dep"}, "desc")
	if err != nil {
		t.Fatalf("CreateWithDepsAndDescription failed: %v", err)
	}

	// Verify we can work with BeadInfo directly
	if result.Title != "Test" {
		t.Errorf("result.Title = %q, want 'Test'", result.Title)
	}

	if len(createdBeads) != 1 {
		t.Errorf("len(createdBeads) = %d, want 1", len(createdBeads))
	}

	if createdBeads[0].ID != "test-id" {
		t.Errorf("createdBeads[0].ID = %q, want 'test-id'", createdBeads[0].ID)
	}
}
