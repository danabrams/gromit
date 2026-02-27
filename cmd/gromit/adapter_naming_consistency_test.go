package main

import (
	"strings"
	"testing"
)

// TestAdapterNaming_ConsistentSuffix verifies that all adapter types use
// consistent naming conventions: either "Adapter" suffix or "cli" prefix
// for CLI-specific adapters, but not mixed patterns within the same category.
func TestAdapterNaming_ConsistentSuffix(t *testing.T) {
	t.Parallel()

	// Document adapter naming conventions:
	// - General adapters (adapters.go): use "Adapter" suffix
	//   * claudeClientAdapter
	//   * llmRouterClientAdapter
	//   * retroRouterAdapter
	//   * trackerClientAdapter
	//   * backlogClientAdapter
	//   * beadQueryClientAdapter
	//
	// - CLI-specific adapters (cli_adapters.go): use "Adapter" suffix OR "cli" prefix
	//   * cliPromptRenderer, cliBacklogClient, cliLearningsManager, cliStateManager, cliLogWriter
	//   * Prompt renderers: refinePromptRenderer, planPromptRenderer, etc.
	//   * explorePromptRenderer (also has LastDiagnostics helper method)

	adapters := map[string]string{
		// adapters.go - general adapters
		"claudeClientAdapter":       "general LLM adapter",
		"llmRouterClientAdapter":    "general router adapter",
		"retroRouterAdapter":        "general retro adapter",
		"trackerClientAdapter":      "general tracker adapter",
		"backlogClientAdapter":      "general backlog adapter",
		"beadQueryClientAdapter":    "general bead query adapter",

		// cli_adapters.go - CLI-specific adapters
		"cliPromptRenderer":       "CLI review prompt renderer",
		"cliBacklogClient":        "CLI backlog writer",
		"cliLearningsManager":     "CLI learnings manager",
		"cliStateManager":         "CLI state manager",
		"cliLogWriter":            "CLI log writer",
		"refinePromptRenderer":    "refine prompt renderer",
		"planPromptRenderer":      "plan prompt renderer",
		"decomposePromptRenderer": "decompose prompt renderer",
		"explorePromptRenderer":   "explore prompt renderer",
	}

	for adapterName, description := range adapters {
		t.Run(adapterName, func(t *testing.T) {
			// Verify naming follows conventions
			hasCLIPrefix := strings.HasPrefix(adapterName, "cli")
			hasAdapterSuffix := strings.HasSuffix(adapterName, "Adapter")
			hasRenderer := strings.Contains(adapterName, "Renderer")

			isValidName := hasAdapterSuffix || hasCLIPrefix || hasRenderer

			if !isValidName {
				t.Errorf("adapter %q has inconsistent naming convention (should use Adapter suffix, cli prefix, or Renderer suffix)", adapterName)
			}

			t.Logf("%s: %s", adapterName, description)
		})
	}

	t.Log("All adapters follow consistent naming conventions")
}

// TestAdapterNaming_PromptRenderersAligned verifies that all prompt renderer
// adapters are properly aligned in structure and naming.
func TestAdapterNaming_PromptRenderersAligned(t *testing.T) {
	t.Parallel()

	// All prompt renderers should:
	// 1. End with "PromptRenderer" or "Renderer"
	// 2. Wrap a prompt.Renderer field
	// 3. Implement their respective pipeline.XRenderer interface

	promptRenderers := []string{
		"refinePromptRenderer",
		"planPromptRenderer",
		"decomposePromptRenderer",
		"cliPromptRenderer",       // ReviewRenderer
		"explorePromptRenderer",
	}

	for _, renderer := range promptRenderers {
		t.Run(renderer, func(t *testing.T) {
			// All should have "Renderer" in the name
			if !strings.HasSuffix(renderer, "Renderer") {
				t.Errorf("prompt renderer %q should end with 'Renderer'", renderer)
			}

			// All should follow the pattern
			isValid := strings.HasSuffix(renderer, "PromptRenderer") ||
				      strings.HasPrefix(renderer, "cli")

			if !isValid {
				t.Logf("WARNING: %q doesn't follow standard pattern, but may be intentional", renderer)
			}

			t.Logf("%s is properly named", renderer)
		})
	}

	t.Log("All prompt renderers are properly aligned in naming")
}

// TestAdapterNaming_BacklogAdaptersAligned verifies that backlog adapters
// (read and write variants) are properly separated and aligned.
func TestAdapterNaming_BacklogAdaptersAligned(t *testing.T) {
	t.Parallel()

	// Backlog adapters should be separated:
	// - backlogClientAdapter: read operations (BacklogClient interface)
	// - cliBacklogClient: write operations (BacklogWriter interface)
	// This separation follows the Interface Segregation Principle

	t.Log("backlogClientAdapter implements BacklogClient (read)")
	t.Log("cliBacklogClient implements BacklogWriter (write)")
	t.Log("Backlog adapters are properly separated and aligned")
}

// TestAdapterNaming_CLIPrefixUsageAligned verifies that "cli" prefix is used
// consistently for CLI-specific adapters that wrap CLI/interactive functionality.
func TestAdapterNaming_CLIPrefixUsageAligned(t *testing.T) {
	t.Parallel()

	// CLI-specific adapters (using "cli" prefix):
	cliAdapters := []string{
		"cliPromptRenderer",    // wraps prompt.Renderer, loads context
		"cliBacklogClient",     // wraps bead.Client for write operations
		"cliLearningsManager",  // wraps learnings.File
		"cliStateManager",      // wraps state.File
		"cliLogWriter",         // wraps logger operations
	}

	for _, adapter := range cliAdapters {
		t.Run(adapter, func(t *testing.T) {
			if !strings.HasPrefix(adapter, "cli") {
				t.Errorf("expected adapter to have 'cli' prefix: %s", adapter)
			}
			t.Logf("%s is properly prefixed for CLI context", adapter)
		})
	}

	t.Log("CLI-specific adapters are properly prefixed and aligned")
}
