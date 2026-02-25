package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConsolidateCliAdapters_FileExists verifies cli_adapters.go exists
// and contains the consolidated prompt renderer adapters.
func TestConsolidateCliAdapters_FileExists(t *testing.T) {
	path := filepath.Join(".", "cli_adapters.go")
	_, err := os.Stat(path)
	if err != nil {
		t.Fatalf("cli_adapters.go should exist: %v", err)
	}
}

// TestConsolidateCliAdapters_ContainsPromptRenderers verifies cli_adapters.go
// defines cliPromptRenderer and explorePromptRenderer.
func TestConsolidateCliAdapters_ContainsPromptRenderers(t *testing.T) {
	path := filepath.Join(".", "cli_adapters.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading cli_adapters.go: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "type cliPromptRenderer struct") {
		t.Error("cli_adapters.go should define cliPromptRenderer")
	}

	if !strings.Contains(contentStr, "type explorePromptRenderer struct") {
		t.Error("cli_adapters.go should define explorePromptRenderer")
	}
}

// TestConsolidateCliAdapters_HasCorrectMethods verifies the adapters
// have their required interface methods.
func TestConsolidateCliAdapters_HasCorrectMethods(t *testing.T) {
	path := filepath.Join(".", "cli_adapters.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading cli_adapters.go: %v", err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, "func (r *cliPromptRenderer) RenderThoroughReview") {
		t.Error("cliPromptRenderer should have RenderThoroughReview method")
	}

	if !strings.Contains(contentStr, "func (r *explorePromptRenderer) RenderExplore") {
		t.Error("explorePromptRenderer should have RenderExplore method")
	}
}
