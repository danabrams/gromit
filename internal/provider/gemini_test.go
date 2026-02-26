package provider

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiShortPromptThresholdIsPractical(t *testing.T) {
	// ARG_MAX on Linux is ~2MB, so threshold should be practical for command-line argument passing
	if geminiShortPromptThreshold < 8192 {
		t.Errorf("geminiShortPromptThreshold = %d, want >= 8192", geminiShortPromptThreshold)
	}
}

func TestGeminiProviderRunFallsBackToInlinePUsesExportedConstant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	shortPrompt := "short"             // Well under threshold
	gp := &GeminiProvider{
		binary: "gemini",
		tierToModel: map[string]string{
			TierLow: "gemini-2.0-flash",
		},
		runFn: func(ctx context.Context, binary string, args []string, prompt string, workDir string) (*geminiRunResult, error) {
			// For short prompts, should use -p flag as fallback
			hasInlineFlag := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-p" {
					hasInlineFlag = true
					if args[i+1] != shortPrompt {
						t.Fatalf("expected -p flag value to match prompt, got %q", args[i+1])
					}
				}
			}
			// Verify that the comparison uses geminiShortPromptThreshold, not a redeclared constant
			if len(prompt) <= geminiShortPromptThreshold && !hasInlineFlag {
				t.Fatalf("short prompt should use -p flag, got args: %v", args)
			}
			payload := []byte(`{
  "output": "OK",
  "usage": {"input_tokens": 5, "output_tokens": 2, "cached_input_tokens": 0},
  "cost": {"total": 0},
  "model": "gemini-2.0-flash",
  "session_id": "test",
  "response": "OK"
}`)
			return &geminiRunResult{stdout: payload, stderr: nil, exitCode: 0, duration: 0}, nil
		},
	}

	result, err := gp.Run(ctx, shortPrompt, TierLow)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
}

func TestGeminiProviderRunValidationRunsPrompt(t *testing.T) {
	t.Parallel()
	mockBinary := testCreateBinaryWithETXTBSYProtection(t, `echo '{"output":"1. go test\n2. go vet\n\nVALIDATION_PASSED","usage":{"input_tokens":100,"output_tokens":50,"cached_input_tokens":0},"cost":{"total":0},"model":"gemini-2.0-flash","session_id":"test","response":"1. go test\n2. go vet\n\nVALIDATION_PASSED"}'
exit 0
`)

	gp := NewGeminiProvider(mockBinary, nil, map[string]string{TierLow: "gemini-2.0-flash"}, nil)
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
	t.Parallel()
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
			t.Parallel()
			got := gp.IsUsageLimitError(tt.result, tt.err)
			if got != tt.expected {
				t.Errorf("IsUsageLimitError() = %v, want %v (result=%+v)", got, tt.expected, tt.result)
			}
		})
	}
}

func TestGeminiProviderRunValidationRejectsInvalidCommands(t *testing.T) {
	t.Parallel()
	gp := NewGeminiProvider("gemini", nil, map[string]string{TierLow: "gemini-2.0-flash"}, nil)

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
			t.Parallel()
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

func TestGeminiProviderValidationMarkerDetection(t *testing.T) {
	t.Parallel()
	gp := &GeminiProvider{}

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
			t.Parallel()
			got := gp.IsValidationPassed(tt.result)
			if got != tt.expected {
				t.Fatalf("IsValidationPassed() = %v, want %v (output=%q)", got, tt.expected, tt.result.Output)
			}
		})
	}
}

func TestGeminiProviderIsScopeTooLargeDetection(t *testing.T) {
	t.Parallel()
	gp := &GeminiProvider{}

	tests := []struct {
		name           string
		result         *Result
		expectedFound  bool
		expectedSubstr string
	}{
		{
			name: "scope too large with explanation",
			result: &Result{
				Success: true,
				Output:  "SCOPE_TOO_LARGE: The codebase is too large to analyze\n\nSome other content",
			},
			expectedFound:  true,
			expectedSubstr: "codebase is too large",
		},
		{
			name: "no scope too large marker",
			result: &Result{
				Success: true,
				Output:  "Everything looks fine",
			},
			expectedFound:  false,
			expectedSubstr: "",
		},
		{
			name: "scope too large marker not at line start",
			result: &Result{
				Success: true,
				Output:  "Some error: SCOPE_TOO_LARGE: message",
			},
			expectedFound:  false,
			expectedSubstr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			found, explanation := gp.IsScopeTooLarge(tt.result)
			if found != tt.expectedFound {
				t.Fatalf("IsScopeTooLarge() found = %v, want %v", found, tt.expectedFound)
			}
			if tt.expectedFound && !strings.Contains(explanation, tt.expectedSubstr) {
				t.Fatalf("IsScopeTooLarge() explanation = %q, want to contain %q", explanation, tt.expectedSubstr)
			}
		})
	}
}

func TestGeminiProviderRunInvokesJSONMode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clockwork := make([]string, 0)
	gp := &GeminiProvider{
		binary: "gemini",
		flags:  []string{"--approval-mode", "yolo"},
		tierToModel: map[string]string{
			TierHigh: "auto-gemini-3",
		},
		runFn: func(ctx context.Context, binary string, args []string, prompt string, workDir string) (*geminiRunResult, error) {
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

	// "hello world" is short prompt (11 bytes < 256 threshold), so should use -p flag
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

func TestGeminiProviderRunDeliveredViaStdin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var capturedPrompt string
	// Create a large prompt (> 8192 bytes threshold) to force stdin delivery
	largePrompt := strings.Repeat("This is a long prompt to test stdin delivery. ", 200)
	gp := &GeminiProvider{
		binary: "gemini",
		tierToModel: map[string]string{
			TierLow: "gemini-2.0-flash",
		},
		runFn: func(ctx context.Context, binary string, args []string, prompt string, workDir string) (*geminiRunResult, error) {
			// Verify args end with "-" to indicate stdin
			if len(args) == 0 || args[len(args)-1] != "-" {
				t.Fatalf("expected last arg to be '-' for stdin, got args: %v", args)
			}
			capturedPrompt = prompt
			payload := []byte(`{
  "output": "READY",
  "usage": {"input_tokens": 10, "output_tokens": 5, "cached_input_tokens": 0},
  "cost": {"total": 0},
  "model": "gemini-2.0-flash",
  "session_id": "test",
  "response": "READY"
}`)
			return &geminiRunResult{stdout: payload, stderr: nil, exitCode: 0, duration: 0}, nil
		},
	}

	result, err := gp.Run(ctx, largePrompt, TierLow)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.Output != "READY" {
		t.Fatalf("output=%q, want READY", result.Output)
	}
	if capturedPrompt != largePrompt {
		t.Fatalf("prompt=%q, want %q", capturedPrompt, largePrompt)
	}
}

func TestGeminiProviderRunFallsBackToInlinePForShortPrompts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	shortPrompt := "short"             // Well under threshold
	gp := &GeminiProvider{
		binary: "gemini",
		tierToModel: map[string]string{
			TierLow: "gemini-2.0-flash",
		},
		runFn: func(ctx context.Context, binary string, args []string, prompt string, workDir string) (*geminiRunResult, error) {
			// For short prompts, should use -p flag as fallback
			hasInlineFlag := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-p" {
					hasInlineFlag = true
					if args[i+1] != shortPrompt {
						t.Fatalf("expected -p flag value to match prompt, got %q", args[i+1])
					}
				}
			}
			if len(prompt) <= geminiShortPromptThreshold && !hasInlineFlag {
				t.Fatalf("short prompt should use -p flag, got args: %v", args)
			}
			payload := []byte(`{
  "output": "OK",
  "usage": {"input_tokens": 5, "output_tokens": 2, "cached_input_tokens": 0},
  "cost": {"total": 0},
  "model": "gemini-2.0-flash",
  "session_id": "test",
  "response": "OK"
}`)
			return &geminiRunResult{stdout: payload, stderr: nil, exitCode: 0, duration: 0}, nil
		},
	}

	result, err := gp.Run(ctx, shortPrompt, TierLow)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
}

func TestGeminiProviderRunWithExplicitWorkingDirectory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	expectedWorkDir := "/expected/work/dir"
	var capturedWorkDir string

	gp := &GeminiProvider{
		binary: "gemini",
		tierToModel: map[string]string{
			TierLow: "gemini-2.0-flash",
		},
		runFn: func(ctx context.Context, binary string, args []string, prompt string, workDir string) (*geminiRunResult, error) {
			capturedWorkDir = workDir
			payload := []byte(`{
  "output": "OK",
  "usage": {"input_tokens": 5, "output_tokens": 2, "cached_input_tokens": 0},
  "cost": {"total": 0},
  "model": "gemini-2.0-flash",
  "session_id": "test",
  "response": "OK"
}`)
			return &geminiRunResult{stdout: payload, stderr: nil, exitCode: 0, duration: 0}, nil
		},
	}

	// Test that RunValidation passes workDir parameter to the runner
	result, err := gp.RunValidation(ctx, []string{"go test"}, TierLow, expectedWorkDir)
	if err != nil {
		t.Fatalf("RunValidation() error = %v", err)
	}
	if result == nil {
		t.Fatal("RunValidation() returned nil result")
	}
	if capturedWorkDir != expectedWorkDir {
		t.Fatalf("workDir=%q, want %q", capturedWorkDir, expectedWorkDir)
	}
}

func TestGeminiProviderHandlesAbsolutePaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Test with absolute path as working directory
	tempDir := t.TempDir()
	// Ensure it's an absolute path
	if !filepath.IsAbs(tempDir) {
		t.Fatalf("test directory is not absolute: %q", tempDir)
	}

	var capturedWorkDir string
	gp := &GeminiProvider{
		binary: "gemini",
		tierToModel: map[string]string{
			TierLow: "gemini-2.0-flash",
		},
		runFn: func(ctx context.Context, binary string, args []string, prompt string, workDir string) (*geminiRunResult, error) {
			capturedWorkDir = workDir
			// Verify that if a workDir is provided, it should be absolute
			if workDir != "" && !filepath.IsAbs(workDir) {
				t.Fatalf("workDir must be absolute when set, got: %q", workDir)
			}
			payload := []byte(`{
  "output": "OK",
  "usage": {"input_tokens": 5, "output_tokens": 2, "cached_input_tokens": 0},
  "cost": {"total": 0},
  "model": "gemini-2.0-flash",
  "session_id": "test",
  "response": "OK"
}`)
			return &geminiRunResult{stdout: payload, stderr: nil, exitCode: 0, duration: 0}, nil
		},
	}

	// Test RunValidation with absolute path
	result, err := gp.RunValidation(ctx, []string{"go test"}, TierLow, tempDir)
	if err != nil {
		t.Fatalf("RunValidation() error = %v", err)
	}
	if result == nil {
		t.Fatal("RunValidation() returned nil result")
	}
	if capturedWorkDir != tempDir {
		t.Fatalf("workDir=%q, want %q", capturedWorkDir, tempDir)
	}
}
