package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestReviewCommand_UsesNewPipelineDeps verifies that review.go uses
// NewPipelineDeps() for dependency injection instead of manual construction.
func TestReviewCommand_UsesNewPipelineDeps(t *testing.T) {
	t.Parallel()

	// Use runtime.Caller to get the absolute path of this test file,
	// then derive the review.go path from there. This avoids issues
	// with relative paths when t.Parallel() races with os.Chdir in other tests.
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	testDir := filepath.Dir(testFile)
	reviewPath := filepath.Join(testDir, "review.go")
	content, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("reading review.go: %v", err)
	}

	contentStr := string(content)

	// Verify that NewPipelineDeps is called in both interactive and non-interactive paths
	if !strings.Contains(contentStr, "NewPipelineDeps(") {
		t.Fatal("review.go must call NewPipelineDeps() for dependency injection")
	}

	// Verify that manual &pipeline.Deps{} construction is removed
	// (should be replaced by NewPipelineDeps calls)
	// Count how many times &pipeline.Deps{ appears
	depsCalls := strings.Count(contentStr, "&pipeline.Deps{")
	if depsCalls > 0 {
		t.Errorf("review.go should use NewPipelineDeps() instead of manual &pipeline.Deps{} construction (found %d manual constructions)", depsCalls)
	}
}
