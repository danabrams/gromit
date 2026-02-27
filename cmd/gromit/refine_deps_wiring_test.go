package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRefineCommand_UsesNewPipelineDeps verifies that refine.go uses
// NewPipelineDeps() for dependency injection instead of manual construction.
func TestRefineCommand_UsesNewPipelineDeps(t *testing.T) {
	t.Parallel()

	refinePath := filepath.Join(".", "refine.go")
	content, err := os.ReadFile(refinePath)
	if err != nil {
		t.Fatalf("reading refine.go: %v", err)
	}

	contentStr := string(content)

	// Verify that NewPipelineDeps is called
	if !strings.Contains(contentStr, "NewPipelineDeps(") {
		t.Fatal("refine.go must call NewPipelineDeps() for dependency injection")
	}

	// Verify that manual &pipeline.Deps{} construction is removed
	depsCalls := strings.Count(contentStr, "&pipeline.Deps{")
	if depsCalls > 0 {
		t.Errorf("refine.go should use NewPipelineDeps() instead of manual &pipeline.Deps{} construction (found %d manual constructions)", depsCalls)
	}
}
