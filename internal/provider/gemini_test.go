package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiProviderRunValidationRunsPrompt(t *testing.T) {
	tempDir := t.TempDir()
	mockBinary := filepath.Join(tempDir, "gemini")
	mockScript := `#!/bin/bash
echo '{"output":"1. go test\n2. go vet\n\nVALIDATION_PASSED","usage":{"input_tokens":100,"output_tokens":50,"cached_input_tokens":0},"cost":{"total":0},"model":"gemini-2.0-flash","session_id":"test"}'
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	gp := NewGeminiProvider(mockBinary, nil, map[string]string{TierLow: "gemini-2.0-flash"})
	result, err := gp.RunValidation(context.Background(), []string{"go test", "go vet"}, TierLow, t.TempDir())
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
	if !gp.IsValidationPassed(result) {
		t.Fatalf("IsValidationPassed() = false, want true (output=%q)", result.Output)
	}
}

func TestGeminiProviderIsUsageLimitErrorDetection(t *testing.T) {
	gp := &GeminiProvider{}

	tests := []struct {
		name     string
		result   *Result
		err      error
		expected bool
	}{
		{
			name: "exit code 2 with usage limit message",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: usage limit exceeded. Please try again later.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with rate limit message",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: rate limit exceeded. Please wait before retrying.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with quota exceeded message",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: quota exceeded for this billing period.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with case-insensitive USAGE LIMIT",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: USAGE LIMIT has been reached.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 1 with usage limit message - not a limit error",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Error: usage limit exceeded.",
			},
			err:      nil,
			expected: false,
		},
		{
			name: "exit code 2 with generic error - not a limit error",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: invalid API key.",
			},
			err:      nil,
			expected: false,
		},
		{
			name:     "nil result returns false",
			result:   nil,
			err:      nil,
			expected: false,
		},
		{
			name: "successful result returns false",
			result: &Result{
				Success:  true,
				ExitCode: 0,
				Output:   "All good",
			},
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gp.IsUsageLimitError(tt.result, tt.err)
			if got != tt.expected {
				t.Errorf("IsUsageLimitError() = %v, want %v (result=%+v)", got, tt.expected, tt.result)
			}
		})
	}
}

func TestGeminiProviderRunValidationRejectsInvalidCommands(t *testing.T) {
	gp := NewGeminiProvider("gemini", nil, map[string]string{TierLow: "gemini-2.0-flash"})

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
			result, err := gp.RunValidation(context.Background(), tt.commands, TierLow, t.TempDir())
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

func TestGeminiProviderRunInvokesJSONMode(t *testing.T) {
	ctx := context.Background()
	clockwork := make([]string, 0)
	gp := &GeminiProvider{
		binary: "gemini",
		flags:  []string{"--approval-mode", "yolo"},
		tierToModel: map[string]string{
			TierHigh: "auto-gemini-3",
		},
		runFn: func(ctx context.Context, binary string, args []string) (*geminiRunResult, error) {
			if binary != "gemini" {
				t.Fatalf("binary=%q, want gemini", binary)
			}
			clockwork = append(clockwork, args...)
			payload := []byte(`{
  "output": "READY",
  "usage": {
    "input_tokens": 1,
    "output_tokens": 2,
    "cached_input_tokens": 0
  },
  "cost": { "total": 0 },
  "model": "auto-gemini-3",
  "session_id": "abc",
  "response": "READY"
}`)
			return &geminiRunResult{stdout: payload, stderr: nil, exitCode: 0, duration: 0}, nil
		},
	}

	if gp.Name() != "gemini" {
		t.Fatalf("name=%q, want gemini", gp.Name())
	}

	result, err := gp.Run(ctx, "hello world", TierHigh)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Output != "READY" {
		t.Fatalf("output=%q, want READY", result.Output)
	}

	wants := []string{"--approval-mode", "yolo", "--output-format", "json", "--model", "auto-gemini-3", "-p", "hello world"}
	if len(clockwork) != len(wants) {
		t.Fatalf("args=%v, want %v", clockwork, wants)
	}
	for i, want := range wants {
		if clockwork[i] != want {
			t.Fatalf("arg[%d]=%q, want %q", i, clockwork[i], want)
		}
	}
}
