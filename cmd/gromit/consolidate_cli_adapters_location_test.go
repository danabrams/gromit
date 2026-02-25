package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCliAdapters_PromptRenderersNotInReviewOrExplore verifies that
// cliPromptRenderer and explorePromptRenderer have been moved to cli_adapters.go
// and are no longer defined in review.go and explore.go.
func TestCliAdapters_PromptRenderersNotInReviewOrExplore(t *testing.T) {
	reviewPath := filepath.Join(".", "review.go")
	reviewContent, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("reading review.go: %v", err)
	}
	reviewStr := string(reviewContent)

	if strings.Contains(reviewStr, "type cliPromptRenderer struct") {
		t.Error("cliPromptRenderer should not be defined in review.go - it should be in cli_adapters.go")
	}

	explorePath := filepath.Join(".", "explore.go")
	exploreContent, err := os.ReadFile(explorePath)
	if err != nil {
		t.Fatalf("reading explore.go: %v", err)
	}
	exploreStr := string(exploreContent)

	if strings.Contains(exploreStr, "type explorePromptRenderer struct") {
		t.Error("explorePromptRenderer should not be defined in explore.go - it should be in cli_adapters.go")
	}
}

// TestCliAdapters_NoMapConstructionForPrompts verifies adapters in cli_adapters.go
// don't construct intermediate maps for prompt data.
func TestCliAdapters_NoMapConstructionForPrompts(t *testing.T) {
	cliAdaptersPath := filepath.Join(".", "cli_adapters.go")
	content, err := os.ReadFile(cliAdaptersPath)
	if err != nil {
		t.Fatalf("reading cli_adapters.go: %v", err)
	}

	contentStr := string(content)

	// Check for map construction in adapter implementations
	if strings.Contains(contentStr, "map[string]interface{}{") {
		t.Error("cliPromptRenderer and explorePromptRenderer should construct typed pipeline structs, not map[string]interface{}")
	}
}
