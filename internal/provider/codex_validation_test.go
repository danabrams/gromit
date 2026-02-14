package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodexProviderRunValidation verifies that RunValidation constructs a prompt and runs it.
// Red: RunValidation is not implemented for CodexProvider yet
func TestCodexProviderRunValidation(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
# Echo the prompt from stdin
cat
echo "VALIDATION_PASSED"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	commands := []string{"go test", "go vet"}
	workDir := "/tmp/test"

	result, err := cp.RunValidation(ctx, commands, TierLow, workDir)

	if err != nil {
		t.Fatalf("RunValidation() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("RunValidation() returned nil result")
	}

	// Verify prompt contains numbered commands
	if !strings.Contains(result.Output, "1. go test") {
		t.Errorf("RunValidation() output missing numbered command, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "2. go vet") {
		t.Errorf("RunValidation() output missing numbered command, got: %s", result.Output)
	}

	// Verify markers are present
	if !strings.Contains(result.Output, "VALIDATION_PASSED") {
		t.Errorf("RunValidation() output missing marker, got: %s", result.Output)
	}
}
