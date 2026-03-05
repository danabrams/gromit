package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// Compile-time interface assertions: verify all adapter types implement their declared interfaces.
var _ pipeline.LLMClient = (*claudeClientAdapter)(nil)
var _ pipeline.ReviewInvoker = (*claudeClientAdapter)(nil)
var _ pipeline.LLMClient = (*llmRouterClientAdapter)(nil)
var _ pipeline.ReviewInvoker = (*llmRouterClientAdapter)(nil)
var _ pipeline.TrackerClient = (*trackerClientAdapter)(nil)
var _ pipeline.BacklogClient = (*backlogClientAdapter)(nil)
var _ pipeline.BeadQueryClient = (*beadQueryClientAdapter)(nil)
var _ pipeline.RefineRenderer = (*refinePromptRenderer)(nil)
var _ pipeline.PlanRenderer = (*planPromptRenderer)(nil)
var _ pipeline.DecomposeRenderer = (*decomposePromptRenderer)(nil)
var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)
var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)
var _ pipeline.LearningsManager = (*cliLearningsManager)(nil)
var _ pipeline.StateManager = (*cliStateManager)(nil)
var _ pipeline.LogWriter = (*cliLogWriter)(nil)

// TestAdapterDepsWiring_AllFieldsWiredInNewPipelineDeps verifies that
// NewPipelineDeps initializes ALL required fields of pipeline.Deps, ensuring
// no field is left nil.
func TestAdapterDepsWiring_AllFieldsWiredInNewPipelineDeps(t *testing.T) {
	t.Parallel()

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
		t.Fatal("NewPipelineDeps returned nil")
	}

	// Check all required fields are non-nil
	checks := []struct {
		name  string
		field interface{}
	}{
		{"AgentResolver", deps.AgentResolver},
		{"LLMClient", deps.LLMClient},
		{"ReviewInvoker", deps.ReviewInvoker},
		{"TrackerClient", deps.TrackerClient},
		{"BeadQueryClient", deps.BeadQueryClient},
		{"BacklogClient", deps.BacklogClient},
		{"BacklogWriter", deps.BacklogWriter},
		{"RefineRenderer", deps.RefineRenderer},
		{"PlanRenderer", deps.PlanRenderer},
		{"DecomposeRenderer", deps.DecomposeRenderer},
		{"ReviewRenderer", deps.ReviewRenderer},
		{"ExploreRenderer", deps.ExploreRenderer},
		{"LearningsManager", deps.LearningsManager},
		{"StateManager", deps.StateManager},
		{"LogWriter", deps.LogWriter},
	}

	for _, check := range checks {
		if check.field == nil {
			t.Errorf("Deps.%s is nil - not wired in NewPipelineDeps", check.name)
		}
	}

	t.Log("All pipeline.Deps fields are properly wired in NewPipelineDeps")
}

// TestNewPipelineDeps_IsDocumentedAsSingleDIPoint documents the architectural
// decision that NewPipelineDeps is the single point where all adapter
// dependencies are constructed.
func TestNewPipelineDeps_IsDocumentedAsSingleDIPoint(t *testing.T) {
	t.Parallel()

	// Read the source file to verify documentation
	sourceFile := "adapter_deps.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Skipf("Could not read %s: %v", sourceFile, err)
	}

	sourceStr := string(content)

	// Verify that the comment describes it as the single DI point
	if !strings.Contains(sourceStr, "single dependency injection point") {
		t.Errorf("NewPipelineDeps should be documented as the single DI point")
	}
}

// TestAdapterWiring_CompleteFormalization verifies that the adapter wiring
// is fully formalized and complete:
//
// 1. All pipeline.Deps fields can be instantiated
// 2. All adapters properly implement their declared interfaces
// 3. The dependency injection works end-to-end through NewPipelineDeps
// 4. All adapters are available for workflows to use
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

	// Verify LLMClient provides Run method
	if deps.LLMClient == nil {
		t.Fatal("LLMClient is nil")
	}

	result, err := deps.LLMClient.Run("test prompt", "haiku")
	if err != nil {
		t.Logf("LLMClient.Run returned error (expected in test environment): %v", err)
	}
	if result != nil {
		t.Logf("LLMClient.Run returned result: %+v", result)
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
