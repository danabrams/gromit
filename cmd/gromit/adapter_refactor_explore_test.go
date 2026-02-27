package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
)

// TestAdapterRefactoring_ExploreRendererCanBePureDelegate documents the
// desired state where explorePromptRenderer is a thin delegation adapter.
//
// CURRENT STATE: explorePromptRenderer contains ~130 lines of business logic
// that builds the explore prompt using strings.Builder, loads learnings,
// formats instructions, and orchestrates multiple renderer calls.
//
// DESIRED STATE: explorePromptRenderer should delegate to a method on
// prompt.Renderer that handles all the prompt building, or the logic
// should be moved to a separate service layer.
//
// RED: This test documents that the adapter should be refactored to be
// a pure delegation adapter, not a business logic container.
func TestAdapterRefactoring_ExploreRendererCanBePureDelegate(t *testing.T) {
	t.Parallel()

	// The adapter should implement the interface
	var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)

	// DESIRED: explorePromptRenderer should only:
	// 1. Extract fields from pipeline.ExplorePromptInput
	// 2. Call a single method on the wrapped prompt.Renderer
	// 3. Return the result

	// CURRENT: explorePromptRenderer does:
	// - Loads ClaudeMD from renderer
	// - Loads Rules from renderer
	// - Loads learnings file
	// - Formats learnings into sections
	// - Gets working directory
	// - Gets gromit and specs directories
	// - Builds instructions section with fmt.Sprintf
	// - Estimates section tokens
	// - Builds entire prompt with strings.Builder
	// - Combines multiple sections

	// After refactoring, prompt.Renderer should have a method like:
	//   RenderExplorePrompt(ctx context.Context, query string) (string, error)
	// That handles all the business logic above.

	// For now, document the current interface
	input := &pipeline.ExplorePromptInput{Query: "test"}
	result, err := (&explorePromptRenderer{renderer: &prompt.Renderer{}}).RenderExplore(input)

	// After refactoring, this should not error even with a nil renderer
	// if the adapter is truly delegating
	_ = result
	_ = err

	t.Log("explorePromptRenderer needs refactoring to be a pure delegation adapter")
	t.Log("Consider moving RenderExplore business logic to prompt.Renderer or a service")
}
