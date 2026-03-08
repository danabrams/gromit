package debug

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestValidateFix_RunsStageAfterFix validates that a fix passes validation.
func TestValidateFix_RunsStageAfterFix(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a simple file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("valid content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create validation context - use a simple command that will pass
	validCtx := &ValidateContext{
		WorktreeRoot: tmpDir,
		FailedStage:  "build",
		ValidateCmd:  "test -f test.txt",
	}

	// Validate the fix
	result, err := ValidateFix(ctx, validCtx)
	if err != nil {
		t.Fatalf("ValidateFix failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("result.Passed = false, want true; output: %s", result.Output)
	}
}

// TestValidateFix_CaptsuresFailureOutput captures output when validation fails.
func TestValidateFix_CapturesFailureOutput(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a file with a syntax error
	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc broken() {\n  return // missing value\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	validCtx := &ValidateContext{
		WorktreeRoot: tmpDir,
		FailedStage:  "build",
		ValidateCmd:  "go build -v",
	}

	result, err := ValidateFix(ctx, validCtx)
	if err != nil {
		// Non-nil error is OK - validation failed as expected
	}

	if result.Passed {
		t.Error("result.Passed = true, want false for broken code")
	}
	if result.Output == "" && result.Error == "" {
		t.Error("expected output or error from failed validation")
	}
}

// TestValidateFix_ReturnsErrorForNilContext returns error for nil context.
func TestValidateFix_ReturnsErrorForNilContext(t *testing.T) {
	ctx := context.Background()
	_, err := ValidateFix(ctx, nil)
	if err == nil {
		t.Error("expected error for nil ValidateContext, got nil")
	}
}
