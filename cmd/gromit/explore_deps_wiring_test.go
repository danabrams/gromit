package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExploreCommand_UsesNewPipelineDeps verifies that explore.go uses
// NewPipelineDeps() for dependency injection instead of manual construction.
func TestExploreCommand_UsesNewPipelineDeps(t *testing.T) {
	t.Parallel()

	explorePath := filepath.Join(".", "explore.go")
	content, err := os.ReadFile(explorePath)
	if err != nil {
		t.Fatalf("reading explore.go: %v", err)
	}

	contentStr := string(content)

	// Verify that NewPipelineDeps is called
	if !strings.Contains(contentStr, "NewPipelineDeps(") {
		t.Fatal("explore.go must call NewPipelineDeps() for dependency injection")
	}

	// Verify that manual &pipeline.Deps{} construction is removed
	depsCalls := strings.Count(contentStr, "&pipeline.Deps{")
	if depsCalls > 0 {
		t.Errorf("explore.go should use NewPipelineDeps() instead of manual &pipeline.Deps{} construction (found %d manual constructions)", depsCalls)
	}
}
