package provider

import (
	"strings"
	"testing"
	"time"
)

func TestClassifyGeminiCLIError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		stderr        string
		wantCategory  string
		wantRetryable bool
	}{
		{
			name:          "setup failure",
			stderr:        "zsh:2: command not found: gemini",
			wantCategory:  "setup/binary-missing",
			wantRetryable: false,
		},
		{
			name:          "model invalid",
			stderr:        "ModelNotFoundError: 404 NOT_FOUND",
			wantCategory:  "model-invalid",
			wantRetryable: false,
		},
		{
			name:          "permission denied",
			stderr:        "ls: cannot open directory: Permission denied",
			wantCategory:  "permission-denied",
			wantRetryable: false,
		},
		{
			name:          "fallback case",
			stderr:        "unexpected panic",
			wantCategory:  "fallback",
			wantRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyGeminiCLIError(tt.stderr)
			if got.Category != tt.wantCategory {
				t.Fatalf("category = %q, want %q", got.Category, tt.wantCategory)
			}
			if got.Retryable != tt.wantRetryable {
				t.Fatalf("retryable = %t, want %t", got.Retryable, tt.wantRetryable)
			}
		})
	}
}

func TestBuildGeminiResultFallbackPreservesExitCode(t *testing.T) {
	t.Parallel()
	execResult := &geminiRunResult{
		stdout:   []byte("this is not valid JSON"),
		stderr:   []byte("ModelNotFoundError: test NOT_FOUND"),
		exitCode: 42,
		duration: 123 * time.Millisecond,
	}

	result, err := buildGeminiResult(execResult, "gemini-3-flash", nil)
	if err != nil {
		t.Fatalf("buildGeminiResult failed: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result")
	}

	if result.ExitCode != execResult.exitCode {
		t.Fatalf("ExitCode = %d, want %d", result.ExitCode, execResult.exitCode)
	}
	if result.Success {
		t.Fatalf("Success = true, want false for exit code %d", execResult.exitCode)
	}
	if !strings.Contains(result.Diagnostics, "gemini_error_category=model-invalid") {
		t.Fatalf("Diagnostics missing category, got %q", result.Diagnostics)
	}
	if !strings.Contains(result.Diagnostics, "exit_code=42") {
		t.Fatalf("Diagnostics missing exit code, got %q", result.Diagnostics)
	}
	if result.Stderr != string(execResult.stderr) {
		t.Fatalf("Stderr = %q, want %q", result.Stderr, execResult.stderr)
	}
}
