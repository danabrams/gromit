package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexProviderRunValidationRunsPrompt(t *testing.T) {
	tempDir := t.TempDir()
	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
cat
echo "VALIDATION_PASSED"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierLow: "gpt-4o-mini"})
	result, err := cp.RunValidation(context.Background(), []string{"go test", "go vet"}, TierLow, t.TempDir())
	if err != nil {
		t.Fatalf("RunValidation() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("RunValidation() returned nil result")
	}

	if !strings.Contains(result.Output, "1. go test") {
		t.Errorf("RunValidation() output missing numbered command, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "2. go vet") {
		t.Errorf("RunValidation() output missing numbered command, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, validationPassedMarker) {
		t.Errorf("RunValidation() output missing marker, got: %s", result.Output)
	}
	if !cp.IsValidationPassed(result) {
		t.Fatalf("IsValidationPassed() = false, want true (output=%q)", result.Output)
	}
}

func TestCodexProviderValidationMarkerDetection(t *testing.T) {
	cp := &CodexProvider{}

	tests := []struct {
		name     string
		result   *Result
		expected bool
	}{
		{
			name: "validation passed marker",
			result: &Result{
				Success: true,
				Output:  "log\nVALIDATION_PASSED\n",
			},
			expected: true,
		},
		{
			name: "validation failed marker",
			result: &Result{
				Success: true,
				Output:  "log\nVALIDATION_FAILED\nexit 1",
			},
			expected: false,
		},
		{
			name: "failed run even with passed marker",
			result: &Result{
				Success: false,
				Output:  "VALIDATION_PASSED",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cp.IsValidationPassed(tt.result)
			if got != tt.expected {
				t.Fatalf("IsValidationPassed() = %v, want %v (output=%q)", got, tt.expected, tt.result.Output)
			}
		})
	}
}

func TestCodexProviderRunValidationRejectsInvalidCommands(t *testing.T) {
	cp := NewCodexProvider("codex", nil, map[string]string{TierLow: "gpt-4o-mini"})

	tests := []struct {
		name     string
		commands []string
		wantErr  string
	}{
		{
			name:     "empty command list",
			commands: nil,
			wantErr:  "at least one command is required",
		},
		{
			name:     "empty command entry",
			commands: []string{""},
			wantErr:  "command 1 is empty",
		},
		{
			name:     "multiline command",
			commands: []string{"go test\ngo vet"},
			wantErr:  "command 1 must be a single line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cp.RunValidation(context.Background(), tt.commands, TierLow, t.TempDir())
			if err == nil {
				t.Fatalf("RunValidation() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("RunValidation() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
			if result != nil {
				t.Fatalf("RunValidation() result = %#v, want nil when command validation fails", result)
			}
		})
	}
}
