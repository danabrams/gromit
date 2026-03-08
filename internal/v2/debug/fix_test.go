package debug

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestApplyFix_CreatesCodeFixFromContext applies a code fix and validates it passes.
func TestApplyFix_CreatesCodeFixFromContext(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a test file with a simple error
	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc broken() int {\n  return // missing value\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a fix context describing what's wrong
	fixCtx := &FixContext{
		FailedStage: "build",
		ErrorMsg:    "syntax error: missing return value",
		FilesInvolved: []string{"main.go"},
		WorktreeRoot: tmpDir,
	}

	// Apply the fix - should succeed with a validated result
	result, err := ApplyFix(ctx, fixCtx)
	if err != nil {
		t.Fatalf("ApplyFix failed: %v", err)
	}
	if !result.Applied {
		t.Error("result.Applied = false, want true")
	}
}

// TestApplyFix_ReturnsErrorForMissingContext returns error when FixContext is nil.
func TestApplyFix_ReturnsErrorForMissingContext(t *testing.T) {
	ctx := context.Background()
	_, err := ApplyFix(ctx, nil)
	if err == nil {
		t.Error("expected error for nil FixContext, got nil")
	}
}
