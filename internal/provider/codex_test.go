package provider

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCodexProviderStructExists verifies that CodexProvider struct exists
// and can be instantiated.
// Expected failure: CodexProvider struct does not exist yet
func TestCodexProviderStructExists(t *testing.T) {
	var cp *CodexProvider
	if cp != nil {
		t.Error("nil CodexProvider should be nil")
	}
}

// TestCodexProviderImplementsProviderInterface verifies that CodexProvider
// satisfies the Provider interface via compile-time check.
// Expected failure: CodexProvider struct does not exist yet
func TestCodexProviderImplementsProviderInterface(t *testing.T) {
	var _ Provider = (*CodexProvider)(nil)
}

// TestCodexProviderHasBinaryPathField verifies that CodexProvider has a
// binaryPath field for storing the path to the codex CLI binary.
// Expected failure: CodexProvider struct and binaryPath field do not exist yet
func TestCodexProviderHasBinaryPathField(t *testing.T) {
	cp := &CodexProvider{
		binaryPath: "/usr/local/bin/codex",
	}

	if cp.binaryPath != "/usr/local/bin/codex" {
		t.Errorf("binaryPath = %q, want %q", cp.binaryPath, "/usr/local/bin/codex")
	}
}

// TestCodexProviderHasFlagsField verifies that CodexProvider has a flags field
// for storing CLI flags to pass to the codex binary.
// Expected failure: CodexProvider struct and flags field do not exist yet
func TestCodexProviderHasFlagsField(t *testing.T) {
	flags := []string{"--verbose", "--no-color"}
	cp := &CodexProvider{
		flags: flags,
	}

	if len(cp.flags) != 2 {
		t.Errorf("len(flags) = %d, want 2", len(cp.flags))
	}
	if cp.flags[0] != "--verbose" {
		t.Errorf("flags[0] = %q, want %q", cp.flags[0], "--verbose")
	}
}

// TestCodexProviderHasPromptDeliveryField verifies that CodexProvider has a
// promptDelivery field specifying how to pass prompts (stdin, file_ref, prompt_file_arg).
// Expected failure: CodexProvider struct and promptDelivery field do not exist yet
func TestCodexProviderHasPromptDeliveryField(t *testing.T) {
	cp := &CodexProvider{
		promptDelivery: "prompt_file_arg",
	}

	if cp.promptDelivery != "prompt_file_arg" {
		t.Errorf("promptDelivery = %q, want %q", cp.promptDelivery, "prompt_file_arg")
	}
}

// TestCodexProviderHasPromptFlagField verifies that CodexProvider has a
// promptFlag field for the CLI flag name to use when passing prompt file paths.
// Expected failure: CodexProvider struct and promptFlag field do not exist yet
func TestCodexProviderHasPromptFlagField(t *testing.T) {
	cp := &CodexProvider{
		promptFlag: "--prompt",
	}

	if cp.promptFlag != "--prompt" {
		t.Errorf("promptFlag = %q, want %q", cp.promptFlag, "--prompt")
	}
}

// TestCodexProviderHasTierToModelMap verifies that CodexProvider has a
// tierToModel map field for mapping abstract tiers to Codex-specific model names.
// Expected failure: CodexProvider struct and tierToModel field do not exist yet
func TestCodexProviderHasTierToModelMap(t *testing.T) {
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	cp := &CodexProvider{
		tierToModel: tierMap,
	}

	if cp.tierToModel == nil {
		t.Error("CodexProvider.tierToModel should not be nil after assignment")
	}

	if cp.tierToModel[TierHigh] != "o3" {
		t.Errorf("tierToModel[TierHigh] = %q, want %q", cp.tierToModel[TierHigh], "o3")
	}
	if cp.tierToModel[TierMedium] != "gpt-4o" {
		t.Errorf("tierToModel[TierMedium] = %q, want %q", cp.tierToModel[TierMedium], "gpt-4o")
	}
	if cp.tierToModel[TierLow] != "gpt-4o-mini" {
		t.Errorf("tierToModel[TierLow] = %q, want %q", cp.tierToModel[TierLow], "gpt-4o-mini")
	}
}

// TestNewCodexProviderConstructor verifies that NewCodexProvider constructor
// creates a CodexProvider with all required fields set correctly.
// Expected failure: NewCodexProvider() function does not exist yet
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
// Expected failure: CodexProvider struct and Name() method do not exist yet
func TestCodexProviderNameMethod(t *testing.T) {
	cp := &CodexProvider{}

	name := cp.Name()

	if name != "codex" {
		t.Errorf("Name() = %q, want %q", name, "codex")
	}
}

// TestCodexProviderRunBuildsCommandWithModelFlag verifies that Run() constructs
// the command with the --model flag using the tier-to-model mapping.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunBuildsCommandWithModelFlag(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock codex binary that echoes its arguments
	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "MODEL_FLAG: $1"
echo "MODEL_VALUE: $2"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test prompt", TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Verify that the command was built with --model gpt-4o
	if !strings.Contains(result.Output, "MODEL_FLAG: --model") {
		t.Errorf("Run() output missing --model flag, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "MODEL_VALUE: gpt-4o") {
		t.Errorf("Run() output missing model value gpt-4o, got: %s", result.Output)
	}
}

// TestCodexProviderRunWritesPromptToTempFile verifies that Run() writes the
// prompt to a temporary file when promptDelivery is "prompt_file_arg".
// Expected failure: CodexProvider Run() method does not exist yet
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

// TestCodexProviderRunCapturesStdout verifies that Run() captures the
// standard output from the codex CLI invocation.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunCapturesStdout(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "Test output line 1"
echo "Test output line 2"
exit 0
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

	if !strings.Contains(result.Output, "Test output line 1") {
		t.Errorf("Run() output missing expected stdout, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Test output line 2") {
		t.Errorf("Run() output missing expected stdout line 2, got: %s", result.Output)
	}
}

// TestCodexProviderRunCapturesStderr verifies that Run() captures the
// standard error output from the codex CLI invocation.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunCapturesStderr(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "Error message" >&2
exit 1
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	// Run should not return an error for non-zero exit codes
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !strings.Contains(result.Output, "Error message") {
		t.Errorf("Run() output missing stderr content, got: %s", result.Output)
	}

	if result.ExitCode != 1 {
		t.Errorf("Run() ExitCode = %d, want 1", result.ExitCode)
	}

	if result.Success {
		t.Error("Run() Success should be false for non-zero exit code")
	}
}

// TestCodexProviderRunReturnsResultWithDuration verifies that Run() populates
// the Result.Duration field with the actual execution time.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunReturnsResultWithDuration(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
sleep 0.1
echo "done"
exit 0
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

	if result.Duration < 50*time.Millisecond {
		t.Errorf("Run() Duration = %v, expected at least 50ms", result.Duration)
	}

	if result.Duration > 5*time.Second {
		t.Errorf("Run() Duration = %v, unexpectedly long", result.Duration)
	}
}

// TestCodexProviderRunSetsSuccessBasedOnExitCode verifies that Run() sets
// Result.Success to true when exit code is 0, false otherwise.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunSetsSuccessBasedOnExitCode(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		exitCode    int
		wantSuccess bool
	}{
		{
			name:        "exit code 0 means success",
			exitCode:    0,
			wantSuccess: true,
		},
		{
			name:        "exit code 1 means failure",
			exitCode:    1,
			wantSuccess: false,
		},
		{
			name:        "exit code 2 means failure",
			exitCode:    2,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBinary := filepath.Join(tempDir, "codex-"+tt.name)
			mockScript := "#!/bin/bash\nexit " + string(rune('0'+tt.exitCode)) + "\n"
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

			if result.Success != tt.wantSuccess {
				t.Errorf("Run() Success = %v, want %v for exit code %d",
					result.Success, tt.wantSuccess, tt.exitCode)
			}

			if result.ExitCode != tt.exitCode {
				t.Errorf("Run() ExitCode = %d, want %d", result.ExitCode, tt.exitCode)
			}
		})
	}
}

// TestCodexProviderRunPopulatesModelInResult verifies that Run() sets the
// Result.Model field to the resolved model name from the tier mapping.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunPopulatesModelInResult(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := "#!/bin/bash\necho 'done'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

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
			ctx := context.Background()
			result, err := cp.Run(ctx, "test", tt.tier)

			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}

			if result.Model != tt.wantModel {
				t.Errorf("Run() Model = %q, want %q for tier %s",
					result.Model, tt.wantModel, tt.tier)
			}
		})
	}
}

// TestCodexProviderStreamRunStreamsOutput verifies that StreamRun() writes
// output to the provided io.Writer as it's produced by the codex CLI.
// Expected failure: CodexProvider StreamRun() method does not exist yet
func TestCodexProviderStreamRunStreamsOutput(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "Line 1"
sleep 0.05
echo "Line 2"
sleep 0.05
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

// TestCodexProviderStreamRunEventHandlerCalledWithJSON verifies that StreamRun()
// invokes EventHandler when a non-nil handler is provided and the binary emits JSONL.
// Expected failure: CodexProvider StreamRun() does not add --json flag or call handler yet
func TestCodexProviderStreamRunEventHandlerCalledWithJSON(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	// Mock binary that emits a JSONL event when --json flag is present
	mockScript := `#!/bin/bash
if [[ "$*" == *"--json"* ]]; then
    echo '{"type":"thread.started","data":{"thread_id":"t-123"}}'
else
    echo 'plain text output'
fi
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	var output bytes.Buffer

	// EventHandler SHOULD be called when non-nil and --json is active
	handlerCalled := false
	handler := func(line []byte) {
		handlerCalled = true
	}

	result, err := cp.StreamRun(ctx, "test", TierLow, &output, handler, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	// EventHandler SHOULD be called when non-nil (after implementation)
	if !handlerCalled {
		t.Error("StreamRun() with non-nil EventHandler should invoke handler for JSONL events")
	}
}

// TestCodexProviderStreamRunToolCallHandlerCalledForToolEvents verifies that StreamRun()
// invokes ToolCallHandler when tool-related events are emitted in JSONL format.
// Expected failure: CodexProvider StreamRun() does not parse tool events or call handler yet
func TestCodexProviderStreamRunToolCallHandlerCalledForToolEvents(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	// Mock binary that emits a command_execution event when --json is present
	mockScript := `#!/bin/bash
if [[ "$*" == *"--json"* ]]; then
    echo '{"type":"item.started","item":{"type":"command_execution","command":"go test"}}'
else
    echo 'plain text output'
fi
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	var output bytes.Buffer

	// ToolCallHandler SHOULD be called for tool-related item.started events
	handlerCalled := false
	var receivedEvent ToolEvent
	toolHandler := func(event ToolEvent) {
		handlerCalled = true
		receivedEvent = event
	}

	result, err := cp.StreamRun(ctx, "test", TierLow, &output, func(line []byte) {}, toolHandler)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	// ToolCallHandler SHOULD be called when non-nil and tool events are present
	if !handlerCalled {
		t.Error("StreamRun() with non-nil ToolCallHandler should invoke handler for tool events")
	}

	// Verify the tool event has correct fields
	if handlerCalled && receivedEvent.ToolName != "Bash" {
		t.Errorf("ToolEvent.ToolName = %q, want %q for command_execution", receivedEvent.ToolName, "Bash")
	}
}

// TestCodexProviderStreamRunReturnsResultWithMetadata verifies that StreamRun()
// returns a Result with all metadata fields populated correctly.
// Expected failure: CodexProvider StreamRun() method does not exist yet
func TestCodexProviderStreamRunReturnsResultWithMetadata(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := "#!/bin/bash\necho 'success'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	result, err := cp.StreamRun(ctx, "test", TierMedium, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	if !result.Success {
		t.Error("StreamRun() Success = false, want true for exit code 0")
	}

	if result.ExitCode != 0 {
		t.Errorf("StreamRun() ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Model != "gpt-4o" {
		t.Errorf("StreamRun() Model = %q, want %q", result.Model, "gpt-4o")
	}

	if result.Duration <= 0 {
		t.Errorf("StreamRun() Duration = %v, want > 0", result.Duration)
	}

	if !strings.Contains(result.Output, "success") {
		t.Errorf("StreamRun() Output missing expected content, got: %s", result.Output)
	}
}

// TestCodexProviderIsUsageLimitErrorDetectsOpenAIErrors verifies that
// IsUsageLimitError() detects OpenAI/Codex-specific usage limit patterns.
// Expected failure: CodexProvider IsUsageLimitError() method does not exist yet
func TestCodexProviderIsUsageLimitErrorDetectsOpenAIErrors(t *testing.T) {
	cp := &CodexProvider{}

	tests := []struct {
		name     string
		result   *Result
		err      error
		expected bool
	}{
		{
			name: "detects quota exceeded",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Error: quota exceeded for this model",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "detects rate limit",
			result: &Result{
				Success:  false,
				ExitCode: 429,
				Output:   "Rate limit exceeded. Please try again later.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "detects usage limit",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "usage limit reached",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "does not detect generic errors",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Error: invalid prompt format",
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
				Output:   "completed successfully",
			},
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cp.IsUsageLimitError(tt.result, tt.err)
			if got != tt.expected {
				t.Errorf("IsUsageLimitError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestCodexProviderModelForTierReturnsCorrectModel verifies that ModelForTier()
// maps tier constants to the configured Codex model names.
// Expected failure: CodexProvider ModelForTier() method does not exist yet
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

// TestCodexProviderRunWithContextCancellation verifies that Run() respects
// context cancellation and stops execution when the context is cancelled.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunWithContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping context cancellation test in short mode")
	}

	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	// Script that runs for a long time
	mockScript := "#!/bin/bash\nsleep 10\necho 'done'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := cp.Run(ctx, "test", TierLow)

	// Should return an error due to context timeout
	if err == nil {
		t.Error("Run() error = nil, want context deadline exceeded error")
	}

	if err != nil && !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Run() error = %q, want context cancellation error", err.Error())
	}

	// Result may be nil or partial
	_ = result
}

// TestCodexProviderRunWithAdditionalFlags verifies that Run() includes
// any additional flags configured in the CodexProvider.flags field.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunWithAdditionalFlags(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	// Echo all arguments to verify flags were passed
	mockScript := `#!/bin/bash
echo "ARGS: $@"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	flags := []string{"--verbose", "--no-color"}
	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, flags, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	outputStr := result.Output
	if !strings.Contains(outputStr, "--verbose") {
		t.Errorf("Run() output missing --verbose flag, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "--no-color") {
		t.Errorf("Run() output missing --no-color flag, got: %s", outputStr)
	}
}

// TestCodexProviderNilReceiverSafety verifies that CodexProvider methods
// handle nil receiver safely by returning appropriate errors.
// Expected failure: CodexProvider methods do not exist yet
func TestCodexProviderNilReceiverSafety(t *testing.T) {
	var cp *CodexProvider

	t.Run("Name with nil receiver", func(t *testing.T) {
		// Name() should handle nil receiver safely (return empty string or panic)
		defer func() {
			if r := recover(); r == nil {
				name := cp.Name()
				if name != "" && name != "codex" {
					t.Errorf("Name() on nil receiver = %q, want empty or 'codex'", name)
				}
			}
		}()
		_ = cp.Name()
	})

	t.Run("Run with nil receiver", func(t *testing.T) {
		ctx := context.Background()
		result, err := cp.Run(ctx, "test", TierLow)

		if err == nil {
			t.Error("Run() on nil receiver error = nil, want non-nil error")
		}
		if result != nil {
			t.Errorf("Run() on nil receiver result = %v, want nil", result)
		}
	})
}

// TestCodexProviderBinaryNotFound verifies that Run() returns an appropriate
// error when the codex binary path does not exist.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderBinaryNotFound(t *testing.T) {
	nonexistentPath := "/nonexistent/path/to/codex"
	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(nonexistentPath, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	if err == nil {
		t.Error("Run() with nonexistent binary error = nil, want non-nil error")
	}

	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "executable") && !strings.Contains(errMsg, "not found") &&
			!strings.Contains(errMsg, "no such file") {
			t.Errorf("Run() error = %q, want error indicating binary not found", errMsg)
		}
	}

	// Result should be nil when command fails to start
	if result != nil {
		t.Errorf("Run() result = %v, want nil when binary not found", result)
	}
}

// TestCodexProviderEmptyTierToModelMap verifies that CodexProvider handles
// an empty tierToModel map by falling back to using the tier name as the model.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderEmptyTierToModelMap(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "MODEL: $2"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	// Empty tier map
	emptyTierMap := map[string]string{}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", emptyTierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	// When tier is not in map, should use tier name directly
	if !strings.Contains(result.Output, "MODEL: "+TierMedium) {
		t.Errorf("Run() with empty tier map should use tier name as model, got output: %s", result.Output)
	}
}

// TestCodexProviderIntegrationWithRealBinary is a comprehensive integration test
// that verifies the full CodexProvider flow with a real-like binary interaction.
// Expected failure: CodexProvider struct and methods do not exist yet
func TestCodexProviderIntegrationWithRealBinary(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available for integration test")
	}

	tempDir := t.TempDir()

	// Create a realistic mock codex binary
	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash

# Parse arguments
MODEL=""
PROMPT_FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --model)
            MODEL="$2"
            shift 2
            ;;
        --prompt)
            PROMPT_FILE="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

# Verify prompt file exists
if [ ! -f "$PROMPT_FILE" ]; then
    echo "Error: Prompt file not found: $PROMPT_FILE" >&2
    exit 1
fi

# Simulate codex response
echo "Processing with model: $MODEL"
echo "Prompt content:"
cat "$PROMPT_FILE"
echo ""
echo "Response: This is a simulated Codex response."

exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	// Create CodexProvider with full configuration
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	// Verify Name()
	if name := cp.Name(); name != "codex" {
		t.Errorf("Name() = %q, want %q", name, "codex")
	}

	// Verify ModelForTier()
	if model := cp.ModelForTier(TierHigh); model != "o3" {
		t.Errorf("ModelForTier(TierHigh) = %q, want %q", model, "o3")
	}

	// Test Run()
	ctx := context.Background()
	testPrompt := "Write a function to calculate fibonacci numbers"
	result, err := cp.Run(ctx, testPrompt, TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if !result.Success {
		t.Errorf("Run() Success = false, want true. Output: %s", result.Output)
	}

	if result.ExitCode != 0 {
		t.Errorf("Run() ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Model != "gpt-4o" {
		t.Errorf("Run() Model = %q, want %q", result.Model, "gpt-4o")
	}

	if !strings.Contains(result.Output, testPrompt) {
		t.Errorf("Run() output missing prompt content, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "gpt-4o") {
		t.Errorf("Run() output missing model name, got: %s", result.Output)
	}

	if result.Duration <= 0 {
		t.Errorf("Run() Duration = %v, want > 0", result.Duration)
	}

	// Test StreamRun()
	var streamOutput bytes.Buffer
	streamResult, err := cp.StreamRun(ctx, testPrompt, TierHigh, &streamOutput, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if streamResult == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	if streamResult.Model != "o3" {
		t.Errorf("StreamRun() Model = %q, want %q", streamResult.Model, "o3")
	}

	streamOutputStr := streamOutput.String()
	if !strings.Contains(streamOutputStr, testPrompt) {
		t.Errorf("StreamRun() streamed output missing prompt, got: %s", streamOutputStr)
	}
}
