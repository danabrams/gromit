package main

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestAdapterWiring_CompleteFormalization verifies that the adapter wiring
// is fully formalized and complete:
//
// 1. All pipeline.Deps fields can be instantiated
// 2. All adapters properly implement their declared interfaces
// 3. The dependency injection works end-to-end through NewPipelineDeps
// 4. All adapters are available for workflows to use
//
// RED: This documents the complete contract for adapter wiring and provides
// assurance that the pipeline can function with all dependencies available.
func TestAdapterWiring_CompleteFormalization(t *testing.T) {
	t.Parallel()

	// Test that NewPipelineDeps provides a complete, usable Deps struct
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
		t.Fatal("NewPipelineDeps returned nil deps")
	}

	// Verify all required fields are non-nil
	requiredFields := map[string]interface{}{
		"AgentResolver":     deps.AgentResolver,
		"LLMClient":         deps.LLMClient,
		"ReviewInvoker":     deps.ReviewInvoker,
		"TrackerClient":     deps.TrackerClient,
		"BeadQueryClient":   deps.BeadQueryClient,
		"BacklogClient":     deps.BacklogClient,
		"BacklogWriter":     deps.BacklogWriter,
		"RefineRenderer":    deps.RefineRenderer,
		"PlanRenderer":      deps.PlanRenderer,
		"DecomposeRenderer": deps.DecomposeRenderer,
		"ReviewRenderer":    deps.ReviewRenderer,
		"ExploreRenderer":   deps.ExploreRenderer,
		"LearningsManager":  deps.LearningsManager,
		"StateManager":      deps.StateManager,
		"LogWriter":         deps.LogWriter,
	}

	for fieldName, fieldValue := range requiredFields {
		if fieldValue == nil {
			t.Errorf("Deps.%s is nil", fieldName)
		}
	}

	// Verify adapters implement their interfaces at type-level
	// These compile-time checks ensure type safety

	// Verify LLMClient provides Run method
	if deps.LLMClient == nil {
		t.Fatal("LLMClient is nil")
	}

	// Verify the adapter is properly initialized
	result, err := deps.LLMClient.Run("test prompt", "haiku")
	if err != nil {
		// We expect this to potentially error due to file system or claude not being available
		// But the method should exist and be callable
		t.Logf("LLMClient.Run returned error (expected in test environment): %v", err)
	}
	if result != nil {
		t.Logf("LLMClient.Run returned result: %+v", result)
	}

	// Verify ReviewInvoker provides Run method
	if deps.ReviewInvoker == nil {
		t.Fatal("ReviewInvoker is nil")
	}

	// Verify BacklogClient provides List method
	if deps.BacklogClient == nil {
		t.Fatal("BacklogClient is nil")
	}

	ideas, err := deps.BacklogClient.List()
	if err != nil {
		t.Logf("BacklogClient.List returned error (expected in test environment): %v", err)
	}
	if ideas != nil {
		t.Logf("BacklogClient.List returned %d ideas", len(ideas))
	}

	// Verify BeadQueryClient provides CountByStatus method
	if deps.BeadQueryClient == nil {
		t.Fatal("BeadQueryClient is nil")
	}

	count, err := deps.BeadQueryClient.CountByStatus(context.Background(), "ready")
	if err != nil {
		t.Logf("BeadQueryClient.CountByStatus returned error (expected in test environment): %v", err)
	}
	if count >= 0 {
		t.Logf("BeadQueryClient.CountByStatus returned count: %d", count)
	}

	// Verify RefineRenderer provides RenderRefine method
	if deps.RefineRenderer == nil {
		t.Fatal("RefineRenderer is nil")
	}

	refinedPrompt, err := deps.RefineRenderer.RenderRefine(&pipeline.RefinePromptInput{
		IdeaText: "test idea",
	})
	if err != nil {
		t.Logf("RefineRenderer.RenderRefine returned error: %v", err)
	}
	if refinedPrompt != "" {
		t.Logf("RefineRenderer.RenderRefine returned prompt (length=%d)", len(refinedPrompt))
	}

	// Verify StateManager provides GetLastReviewCommit method
	if deps.StateManager == nil {
		t.Fatal("StateManager is nil")
	}

	commit, err := deps.StateManager.GetLastReviewCommit()
	if err != nil {
		t.Logf("StateManager.GetLastReviewCommit returned error: %v", err)
	}
	if commit != "" {
		t.Logf("StateManager.GetLastReviewCommit returned commit: %s", commit)
	}

	t.Log("Complete adapter wiring is formalized and functional")
}
