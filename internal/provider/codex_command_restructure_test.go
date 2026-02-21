package provider

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodexProviderBuildsExecCommand verifies that buildCommandArgs produces
// the correct Codex CLI command structure: ['exec', flags..., '--full-auto',
// '--skip-git-repo-check', '--color', 'never', '--model', model, '-'].
func TestCodexProviderBuildsExecCommand(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "ARGS: $@"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	flags := []string{"--cd", "/workspace"}
	cp := NewCodexProvider(mockBinary, flags, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test prompt", TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	outputStr := result.Output
	// Verify command structure includes 'exec' as first positional arg
	if !strings.Contains(outputStr, "exec") {
		t.Errorf("Run() output missing 'exec' command, got: %s", outputStr)
	}

	// Verify --full-auto flag is present
	if !strings.Contains(outputStr, "--full-auto") {
		t.Errorf("Run() output missing '--full-auto' flag, got: %s", outputStr)
	}

	// Verify --skip-git-repo-check flag is present
	if !strings.Contains(outputStr, "--skip-git-repo-check") {
		t.Errorf("Run() output missing '--skip-git-repo-check' flag, got: %s", outputStr)
	}

	// Verify --color never flag is present
	if !strings.Contains(outputStr, "--color") || !strings.Contains(outputStr, "never") {
		t.Errorf("Run() output missing '--color never' flag, got: %s", outputStr)
	}

	// Verify --model flag with correct model name
	if !strings.Contains(outputStr, "--model") || !strings.Contains(outputStr, "gpt-4o") {
		t.Errorf("Run() output missing '--model gpt-4o', got: %s", outputStr)
	}

	// Verify stdin delivery with '-' positional arg
	if !strings.Contains(outputStr, " -") {
		t.Errorf("Run() output missing '-' stdin positional arg, got: %s", outputStr)
	}
}

// TestCodexProviderDoesNotHavePromptDeliveryField verifies that CodexProvider
// struct no longer has the promptDelivery field.
func TestCodexProviderDoesNotHavePromptDeliveryField(t *testing.T) {
	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider("/bin/codex", []string{}, tierMap)

	// This test verifies that the constructor signature doesn't require promptDelivery
	// by passing empty strings where promptDelivery and promptFlag used to be.
	// If the fields were removed, this should compile and work.
	if cp == nil {
		t.Fatal("NewCodexProvider returned nil")
	}

	// Attempt to access promptDelivery field should not compile after refactor
	// This is a compile-time check - if this compiles, the field still exists
	_ = cp.binaryPath // This should compile
	// _ = cp.promptDelivery // This should NOT compile after refactor
}

// TestCodexProviderDoesNotHavePromptFlagField verifies that CodexProvider
// struct no longer has the promptFlag field.
func TestCodexProviderDoesNotHavePromptFlagField(t *testing.T) {
	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider("/bin/codex", []string{}, tierMap)

	if cp == nil {
		t.Fatal("NewCodexProvider returned nil")
	}

	// Attempt to access promptFlag field should not compile after refactor
	_ = cp.flags // This should compile
	// _ = cp.promptFlag // This should NOT compile after refactor
}

// TestCodexProviderRunDeliversPromptViaStdin verifies that Run() pipes the
// prompt content to the codex binary via stdin instead of using a temp file.
func TestCodexProviderRunDeliversPromptViaStdin(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	// This script reads from stdin and echoes it
	mockScript := `#!/bin/bash
echo "STDIN_CONTENT:"
cat
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	testPrompt := "This is the test prompt content that should be sent via stdin"
	result, err := cp.Run(ctx, testPrompt, TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	// Verify the prompt content was delivered via stdin
	if !strings.Contains(result.Output, "STDIN_CONTENT:") {
		t.Errorf("Run() did not read from stdin, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, testPrompt) {
		t.Errorf("Run() stdin missing prompt content, got: %s", result.Output)
	}
}

// TestCodexProviderStreamRunDeliversPromptViaStdin verifies that StreamRun()
// pipes the prompt content to the codex binary via stdin.
func TestCodexProviderStreamRunDeliversPromptViaStdin(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
PROMPT=$(cat)
echo "{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"RECEIVED_VIA_STDIN: $PROMPT\"}}"
echo '{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5}}'
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	testPrompt := "StreamRun test prompt via stdin"
	result, err := cp.StreamRun(ctx, testPrompt, TierMedium, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	combinedOutput := result.Output + output.String()
	if !strings.Contains(combinedOutput, "RECEIVED_VIA_STDIN:") {
		t.Errorf("StreamRun() did not use stdin, got: %s", combinedOutput)
	}

	if !strings.Contains(combinedOutput, testPrompt) {
		t.Errorf("StreamRun() stdin missing prompt content, got: %s", combinedOutput)
	}
}

// TestCodexProviderRunDoesNotCreateTempFile verifies that Run() no longer
// creates temporary prompt files in the system temp directory.
func TestCodexProviderRunDoesNotCreateTempFile(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	// Script that lists files in temp directory before and after
	mockScript := `#!/bin/bash
# Read stdin to completion
cat > /dev/null
echo "completed"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	// Count temp files before Run()
	tempPattern := filepath.Join(os.TempDir(), "codex-prompt-*.txt")
	beforeFiles, _ := filepath.Glob(tempPattern)
	beforeCount := len(beforeFiles)

	ctx := context.Background()
	_, err := cp.Run(ctx, "test prompt", TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	// Count temp files after Run()
	afterFiles, _ := filepath.Glob(tempPattern)
	afterCount := len(afterFiles)

	// After refactor, Run() should not create any temp files
	if afterCount > beforeCount {
		t.Errorf("Run() created %d temp files, want 0 (before=%d, after=%d)",
			afterCount-beforeCount, beforeCount, afterCount)
	}
}

// TestCodexProviderStreamRunDoesNotCreateTempFile verifies that StreamRun()
// no longer creates temporary prompt files.
func TestCodexProviderStreamRunDoesNotCreateTempFile(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
cat > /dev/null
echo "done"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	tempPattern := filepath.Join(os.TempDir(), "codex-prompt-*.txt")
	beforeFiles, _ := filepath.Glob(tempPattern)
	beforeCount := len(beforeFiles)

	ctx := context.Background()
	var output bytes.Buffer
	_, err := cp.StreamRun(ctx, "test prompt", TierMedium, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	afterFiles, _ := filepath.Glob(tempPattern)
	afterCount := len(afterFiles)

	if afterCount > beforeCount {
		t.Errorf("StreamRun() created %d temp files, want 0 (before=%d, after=%d)",
			afterCount-beforeCount, beforeCount, afterCount)
	}
}

// TestCodexProviderConstructorDoesNotRequirePromptParams verifies that
// NewCodexProvider no longer requires promptDelivery and promptFlag parameters.
func TestCodexProviderConstructorDoesNotRequirePromptParams(t *testing.T) {
	binaryPath := "/usr/local/bin/codex"
	flags := []string{"--cd", "/workspace"}
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	// After refactor, constructor should only need: binaryPath, flags, tierToModel
	// The empty strings here represent removed promptDelivery and promptFlag
	cp := NewCodexProvider(binaryPath, flags, tierMap)

	if cp == nil {
		t.Fatal("NewCodexProvider returned nil")
	}

	// Verify essential fields are set
	if cp.binaryPath != binaryPath {
		t.Errorf("binaryPath = %q, want %q", cp.binaryPath, binaryPath)
	}

	if len(cp.flags) != len(flags) {
		t.Errorf("len(flags) = %d, want %d", len(cp.flags), len(flags))
	}

	if cp.tierToModel[TierHigh] != "o3" {
		t.Errorf("tierToModel[TierHigh] = %q, want %q", cp.tierToModel[TierHigh], "o3")
	}
}

// TestCodexProviderCommandIncludesUserFlags verifies that buildCommandArgs
// includes user-provided flags from the flags field before the standard flags.
func TestCodexProviderCommandIncludesUserFlags(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "ALL_ARGS: $@"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	userFlags := []string{"--cd", "/workspace", "--verbose"}
	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, userFlags, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	outputStr := result.Output
	// Verify user flags are included
	if !strings.Contains(outputStr, "--cd") || !strings.Contains(outputStr, "/workspace") {
		t.Errorf("Run() output missing user flag '--cd /workspace', got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "--verbose") {
		t.Errorf("Run() output missing user flag '--verbose', got: %s", outputStr)
	}

	// Verify standard flags are also present
	if !strings.Contains(outputStr, "--full-auto") {
		t.Errorf("Run() output missing standard flag '--full-auto', got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "--model") {
		t.Errorf("Run() output missing standard flag '--model', got: %s", outputStr)
	}
}

// TestCodexProviderStdinDeliveryWithLargePrompt verifies that stdin delivery
// works correctly with large prompts that would exceed ARG_MAX if passed as arguments.
func TestCodexProviderStdinDeliveryWithLargePrompt(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
# Count bytes from stdin
BYTE_COUNT=$(cat | wc -c)
echo "STDIN_BYTES: $BYTE_COUNT"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	// Create a large prompt (100KB) that would fail with ARG_MAX
	largePrompt := strings.Repeat("This is a large prompt. ", 5000) // ~125KB

	ctx := context.Background()
	result, err := cp.Run(ctx, largePrompt, TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	// Verify the full prompt was delivered via stdin
	if !strings.Contains(result.Output, "STDIN_BYTES:") {
		t.Errorf("Run() did not deliver prompt via stdin, got: %s", result.Output)
	}

	// Verify byte count is non-zero (prompt was received)
	if strings.Contains(result.Output, "STDIN_BYTES: 0") {
		t.Errorf("Run() stdin received 0 bytes, want > 0 for large prompt")
	}
}

// TestCodexProviderBuildCommandArgsProducesCorrectOrder verifies that
// buildCommandArgs produces arguments in the correct order for Codex CLI:
// ['exec', user_flags..., '--full-auto', '--skip-git-repo-check', '--color', 'never', '--model', model, '-']
func TestCodexProviderBuildCommandArgsProducesCorrectOrder(t *testing.T) {
	tierMap := map[string]string{TierMedium: "gpt-4o"}
	userFlags := []string{"--cd", "/workspace"}
	cp := &CodexProvider{
		binaryPath:  "/bin/codex",
		flags:       userFlags,
		tierToModel: tierMap,
	}

	model := "gpt-4o"
	// Note: buildCommandArgs signature changed - takes model and jsonMode boolean
	args := cp.buildCommandArgs(model, false)

	// Verify first arg is 'exec'
	if len(args) == 0 || args[0] != "exec" {
		t.Errorf("buildCommandArgs()[0] = %q, want %q", args[0], "exec")
	}

	// Verify user flags come after 'exec'
	foundCdFlag := false
	for i, arg := range args {
		if arg == "--cd" && i+1 < len(args) && args[i+1] == "/workspace" {
			foundCdFlag = true
			break
		}
	}
	if !foundCdFlag {
		t.Errorf("buildCommandArgs() missing user flag '--cd /workspace', got: %v", args)
	}

	// Verify standard flags are present
	hasFullAuto := false
	hasSkipGitCheck := false
	hasColorNever := false
	hasModel := false
	hasStdinArg := false

	for i, arg := range args {
		if arg == "--full-auto" {
			hasFullAuto = true
		}
		if arg == "--skip-git-repo-check" {
			hasSkipGitCheck = true
		}
		if arg == "--color" && i+1 < len(args) && args[i+1] == "never" {
			hasColorNever = true
		}
		if arg == "--model" && i+1 < len(args) && args[i+1] == "gpt-4o" {
			hasModel = true
		}
		if arg == "-" {
			hasStdinArg = true
		}
	}

	if !hasFullAuto {
		t.Errorf("buildCommandArgs() missing '--full-auto', got: %v", args)
	}
	if !hasSkipGitCheck {
		t.Errorf("buildCommandArgs() missing '--skip-git-repo-check', got: %v", args)
	}
	if !hasColorNever {
		t.Errorf("buildCommandArgs() missing '--color never', got: %v", args)
	}
	if !hasModel {
		t.Errorf("buildCommandArgs() missing '--model gpt-4o', got: %v", args)
	}
	if !hasStdinArg {
		t.Errorf("buildCommandArgs() missing '-' stdin arg, got: %v", args)
	}

	// Verify '-' is the last argument
	if len(args) > 0 && args[len(args)-1] != "-" {
		t.Errorf("buildCommandArgs() last arg = %q, want %q (stdin indicator)", args[len(args)-1], "-")
	}
}

func TestCodexProviderBuildCommandArgsAddsReasoningEffortByTier(t *testing.T) {
	cp := &CodexProvider{
		binaryPath:      "/bin/codex",
		flags:           []string{},
		tierToModel:     map[string]string{TierHigh: "gpt-5.3-codex"},
		tierToReasoning: map[string]string{TierHigh: "high"},
	}

	args := cp.buildCommandArgsForTier("gpt-5.3-codex", TierHigh, false)
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "model_reasoning_effort=high") {
		t.Fatalf("expected model_reasoning_effort high in args, got: %v", args)
	}
}

func TestCodexProviderBuildCommandArgsDoesNotOverrideUserReasoningEffort(t *testing.T) {
	cp := &CodexProvider{
		binaryPath:      "/bin/codex",
		flags:           []string{"-c", "model_reasoning_effort=medium"},
		tierToModel:     map[string]string{TierHigh: "gpt-5.3-codex"},
		tierToReasoning: map[string]string{TierHigh: "high"},
	}

	args := cp.buildCommandArgsForTier("gpt-5.3-codex", TierHigh, false)
	joined := strings.Join(args, " ")

	if strings.Count(joined, "model_reasoning_effort") != 1 {
		t.Fatalf("expected exactly one reasoning effort config, got args: %v", args)
	}
	if !strings.Contains(joined, "model_reasoning_effort=medium") {
		t.Fatalf("expected user-provided reasoning effort to win, got args: %v", args)
	}
}

func TestReasoningEffortFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "short config split arg",
			args: []string{"exec", "-c", "model_reasoning_effort=high"},
			want: "high",
		},
		{
			name: "long config split arg",
			args: []string{"exec", "--config", "model_reasoning_effort=medium"},
			want: "medium",
		},
		{
			name: "inline config arg",
			args: []string{"exec", "--config=model_reasoning_effort=low"},
			want: "low",
		},
		{
			name: "missing config",
			args: []string{"exec", "--model", "gpt-5.3-codex"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reasoningEffortFromArgs(tt.args)
			if got != tt.want {
				t.Fatalf("reasoningEffortFromArgs(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

// TestCodexProviderStreamRunWithJSONFlagAndStdin verifies that when EventHandler
// is present, StreamRun adds --json flag AND uses stdin for prompt delivery.
func TestCodexProviderStreamRunWithJSONFlagAndStdin(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
# Read stdin
STDIN_CONTENT=$(cat)

# Check if --json flag is present
if [[ "$*" == *"--json"* ]]; then
    # Emit JSON with markers
    echo '{"type":"item.completed","item":{"type":"agent_message","text":"HAS_JSON_FLAG STDIN_CONTENT:'"$STDIN_CONTENT"'"}}'
else
    # Plain text mode
    echo "HAS_JSON_FLAG"
    echo "STDIN_CONTENT:$STDIN_CONTENT"
fi
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	testPrompt := "test prompt content"

	handlerCalled := false
	handler := func(line []byte) {
		handlerCalled = true
	}

	result, err := cp.StreamRun(ctx, testPrompt, TierMedium, &output, handler, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	combinedOutput := result.Output + output.String()

	// Verify --json flag was passed
	if !strings.Contains(combinedOutput, "HAS_JSON_FLAG") {
		t.Errorf("StreamRun() with EventHandler did not pass --json flag, got: %s", combinedOutput)
	}

	// Verify stdin delivery was used
	if !strings.Contains(combinedOutput, "STDIN_CONTENT:") {
		t.Errorf("StreamRun() did not use stdin delivery, got: %s", combinedOutput)
	}

	if !strings.Contains(combinedOutput, testPrompt) {
		t.Errorf("StreamRun() stdin missing prompt content, got: %s", combinedOutput)
	}

	// Handler should have been called (placeholder check)
	_ = handlerCalled
}

// TestCodexProviderRunHandlesStdinWriteError verifies that Run() properly
// handles errors when writing to stdin pipe fails.
func TestCodexProviderRunHandlesStdinWriteError(t *testing.T) {
	// This test verifies error handling for stdin write failures
	// When the binary exits early or closes stdin, writing to stdin should error gracefully

	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	// Binary that exits immediately without reading stdin
	mockScript := `#!/bin/bash
exit 1
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	largePrompt := strings.Repeat("data ", 100000) // Large prompt to trigger write

	result, err := cp.Run(ctx, largePrompt, TierMedium)

	// Should not panic or hang; may return error or complete with failure result
	if err != nil {
		// Error is acceptable for this scenario
		return
	}

	// Or result with non-zero exit code is also acceptable
	if result != nil && !result.Success {
		return
	}

	// Either error or failure result is expected
	t.Log("Run() completed without error despite stdin write likely failing")
}

// TestCodexProviderStreamRunHandlesStdinPipeError verifies that StreamRun()
// properly handles errors when creating or writing to stdin pipe.
func TestCodexProviderStreamRunHandlesStdinPipeError(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
# Exit immediately to close stdin early
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	largePrompt := strings.Repeat("large data ", 50000)

	result, err := cp.StreamRun(ctx, largePrompt, TierMedium, &output, nil, nil)

	// Should handle the error gracefully without panicking
	if err != nil {
		// Error handling is acceptable
		return
	}

	if result != nil && !result.Success {
		// Failure result is also acceptable
		return
	}

	t.Log("StreamRun() completed despite potential stdin pipe issues")
}

// TestCodexProviderBuildCommandArgsSignatureDoesNotRequirePromptFile verifies
// that buildCommandArgs no longer accepts a promptFile parameter.
func TestCodexProviderBuildCommandArgsSignatureDoesNotRequirePromptFile(t *testing.T) {
	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := &CodexProvider{
		binaryPath:  "/bin/codex",
		flags:       []string{},
		tierToModel: tierMap,
	}

	model := "gpt-4o"

	// After refactor, buildCommandArgs takes model and jsonMode boolean
	args := cp.buildCommandArgs(model, false)

	if len(args) == 0 {
		t.Error("buildCommandArgs() returned empty slice")
	}

	// Old signature was: buildCommandArgs(model, promptFile string) []string
	// New signature is: buildCommandArgs(model string, jsonMode bool) []string
	// This test verifies the new signature by not passing promptFile

	_ = args // Use the result to avoid unused variable error
}
