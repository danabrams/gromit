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
