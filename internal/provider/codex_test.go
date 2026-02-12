package provider

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodexProviderStructExists verifies that CodexProvider struct exists
// and can be instantiated.
func TestCodexProviderStructExists(t *testing.T) {
	var cp *CodexProvider
	if cp != nil {
		t.Error("nil CodexProvider should be nil")
	}
}

// TestNewCodexProviderConstructor verifies that NewCodexProvider constructor
// creates a CodexProvider with all required fields set correctly.
func TestNewCodexProviderConstructor(t *testing.T) {
	binaryPath := "/usr/local/bin/codex"
	flags := []string{"--no-color"}
	promptDelivery := "prompt_file_arg"
	promptFlag := "--prompt"
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	cp := NewCodexProvider(binaryPath, flags, promptDelivery, promptFlag, tierMap)

	if cp == nil {
		t.Fatal("NewCodexProvider() returned nil")
	}

	if cp.binaryPath != binaryPath {
		t.Errorf("binaryPath = %q, want %q", cp.binaryPath, binaryPath)
	}

	if len(cp.flags) != len(flags) || cp.flags[0] != flags[0] {
		t.Errorf("flags = %v, want %v", cp.flags, flags)
	}

	if cp.promptDelivery != promptDelivery {
		t.Errorf("promptDelivery = %q, want %q", cp.promptDelivery, promptDelivery)
	}

	if cp.promptFlag != promptFlag {
		t.Errorf("promptFlag = %q, want %q", cp.promptFlag, promptFlag)
	}

	if cp.tierToModel[TierHigh] != "o3" {
		t.Errorf("tierToModel[TierHigh] = %q, want %q", cp.tierToModel[TierHigh], "o3")
	}
}

// TestCodexProviderNameMethod verifies that CodexProvider implements
// Name() method returning "codex".
func TestCodexProviderNameMethod(t *testing.T) {
	cp := &CodexProvider{}

	name := cp.Name()

	if name != "codex" {
		t.Errorf("Name() = %q, want %q", name, "codex")
	}
}

// TestCodexProviderModelForTierReturnsCorrectModel verifies that ModelForTier()
// maps tier constants to the configured Codex model names.
func TestCodexProviderModelForTierReturnsCorrectModel(t *testing.T) {
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	cp := &CodexProvider{
		tierToModel: tierMap,
	}

	tests := []struct {
		tier      string
		wantModel string
	}{
		{TierHigh, "o3"},
		{TierMedium, "gpt-4o"},
		{TierLow, "gpt-4o-mini"},
	}

	for _, tt := range tests {
		t.Run("tier_"+tt.tier, func(t *testing.T) {
			got := cp.ModelForTier(tt.tier)
			if got != tt.wantModel {
				t.Errorf("ModelForTier(%q) = %q, want %q", tt.tier, got, tt.wantModel)
			}
		})
	}
}

// TestCodexProviderRunWritesPromptToTempFile verifies that Run() writes the
// prompt to a temporary file when promptDelivery is "prompt_file_arg".
func TestCodexProviderRunWritesPromptToTempFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock codex binary that reads and echoes the prompt file
	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
# The prompt file should be passed via --prompt flag
PROMPT_FILE=""
for i in "$@"; do
    if [ "$prev" = "--prompt" ]; then
        PROMPT_FILE="$i"
        break
    fi
    prev="$i"
done

if [ -f "$PROMPT_FILE" ]; then
    echo "PROMPT_CONTENT:"
    cat "$PROMPT_FILE"
else
    echo "ERROR: Prompt file not found or not passed"
    exit 1
fi
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	testPrompt := "This is a test prompt for Codex"
	result, err := cp.Run(ctx, testPrompt, TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !strings.Contains(result.Output, "PROMPT_CONTENT:") {
		t.Errorf("Run() did not pass prompt file correctly, output: %s", result.Output)
	}

	if !strings.Contains(result.Output, testPrompt) {
		t.Errorf("Run() prompt file missing expected content, output: %s", result.Output)
	}
}

// TestCodexProviderRunCapturesStdoutAndStderr verifies that Run() captures both
// stdout and stderr from the codex CLI invocation.
func TestCodexProviderRunCapturesStdoutAndStderr(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "stdout message"
echo "stderr message" >&2
exit 1
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !strings.Contains(result.Output, "stdout message") {
		t.Errorf("Run() output missing stdout content, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "stderr message") {
		t.Errorf("Run() output missing stderr content, got: %s", result.Output)
	}

	if result.ExitCode != 1 {
		t.Errorf("Run() ExitCode = %d, want 1", result.ExitCode)
	}

	if result.Success {
		t.Error("Run() Success should be false for non-zero exit code")
	}
}

// TestCodexProviderStreamRunStreamsOutput verifies that StreamRun() writes
// output to the provided io.Writer as it's produced by the codex CLI.
func TestCodexProviderStreamRunStreamsOutput(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "Line 1"
echo "Line 2"
echo "Line 3"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	result, err := cp.StreamRun(ctx, "test prompt", TierMedium, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Line 1") {
		t.Errorf("StreamRun() output missing 'Line 1', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Line 2") {
		t.Errorf("StreamRun() output missing 'Line 2', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Line 3") {
		t.Errorf("StreamRun() output missing 'Line 3', got: %s", outputStr)
	}
}
