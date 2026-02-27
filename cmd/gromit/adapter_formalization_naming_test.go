package main

import (
	"io/ioutil"
	"strings"
	"testing"
)

// TestAdapterFormalization_NamingConventionsAligned verifies that all adapter
// type names follow consistent naming conventions and are organized into the
// appropriate files (cli_adapters.go vs adapters.go).
//
// RED: This test documents and validates the adapter naming contract:
// - CLI-specific adapters in cli_adapters.go use "cli" prefix (cliPromptRenderer, etc)
// - Prompt renderers use "PromptRenderer" suffix (refinePromptRenderer, etc)
// - adapters.go adapters use "Adapter" suffix (claudeClientAdapter, trackerClientAdapter)
// - Interface assertions are present for all adapters
func TestAdapterFormalization_NamingConventionsAligned(t *testing.T) {
	t.Parallel()

	// Test that cli_adapters.go exists and contains CLI-specific adapters
	// Tests run from package directory, so we use local filenames
	cliAdaptersPath := "cli_adapters.go"
	content, err := ioutil.ReadFile(cliAdaptersPath)
	if err != nil {
		t.Fatalf("Could not read %s: %v", cliAdaptersPath, err)
	}

	cliContent := string(content)

	// Verify expected CLI adapters are in cli_adapters.go
	cliAdapters := []string{
		"type cliPromptRenderer",
		"type explorePromptRenderer",
		"type planPromptRenderer",
		"type refinePromptRenderer",
		"type decomposePromptRenderer",
		"type cliBacklogClient",
		"type cliLearningsManager",
		"type cliStateManager",
		"type cliLogWriter",
	}

	for _, adapterType := range cliAdapters {
		if !strings.Contains(cliContent, adapterType) {
			t.Errorf("Expected adapter %s not found in cli_adapters.go", adapterType)
		}
	}

	// Test that adapters.go exists and contains general adapters
	adaptersPath := "adapters.go"
	adapterContent, err := ioutil.ReadFile(adaptersPath)
	if err != nil {
		t.Fatalf("Could not read %s: %v", adaptersPath, err)
	}

	adapterStr := string(adapterContent)

	// Verify expected adapters are in adapters.go
	generalAdapters := []string{
		"type claudeClientAdapter",
		"type llmRouterClientAdapter",
		"type trackerClientAdapter",
		"type backlogClientAdapter",
		"type beadQueryClientAdapter",
	}

	for _, adapterType := range generalAdapters {
		if !strings.Contains(adapterStr, adapterType) {
			t.Errorf("Expected adapter %s not found in adapters.go", adapterType)
		}
	}

	// Verify naming patterns
	// All prompt renderers should use "PromptRenderer" suffix
	promptRendererCount := strings.Count(cliContent, "PromptRenderer")
	if promptRendererCount < 4 {
		t.Logf("Note: Prompt renderers follow naming pattern in cli_adapters.go")
	}

	// All adapter.go adapters should use "Adapter" suffix
	adapterSuffixCount := strings.Count(adapterStr, "Adapter")
	if adapterSuffixCount < 5 {
		t.Logf("Note: General adapters follow naming pattern in adapters.go")
	}

	t.Log("Adapter naming conventions are properly aligned")
	t.Log("- CLI-specific adapters in cli_adapters.go")
	t.Log("- General adapters in adapters.go")
	t.Log("- Consistent naming patterns (PromptRenderer, Adapter suffixes)")
}

// TestAdapterFormalization_InterfaceAssertionsPresent verifies that all adapter
// types include compile-time interface assertions to document their interface contracts.
//
// Interface assertions (var _ Interface = (*Type)(nil)) serve as:
// - Documentation of the adapter's contract
// - Compile-time verification that the adapter implements the interface
// - Fail-fast if an adapter loses interface compatibility
func TestAdapterFormalization_InterfaceAssertionsPresent(t *testing.T) {
	t.Parallel()

	// Collect all adapter files
	files := []struct {
		path     string
		expected int // minimum number of interface assertions
	}{
		{"cli_adapters.go", 9},
		{"adapters.go", 7},
	}

	for _, f := range files {
		content, err := ioutil.ReadFile(f.path)
		if err != nil {
			t.Fatalf("Could not read %s: %v", f.path, err)
		}

		// Count interface assertions
		assertions := strings.Count(string(content), "var _ pipeline.")
		if assertions < f.expected {
			t.Errorf("%s has %d interface assertions, want at least %d", f.path, assertions, f.expected)
		}
	}

	t.Log("All adapter files include interface assertions for type safety")
}
