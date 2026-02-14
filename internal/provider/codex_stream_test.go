package provider

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProcessCodexStreamEmptyInput verifies that processCodexStream handles empty input.
// Red: processCodexStream function does not exist yet
func TestProcessCodexStreamEmptyInput(t *testing.T) {
	reader := strings.NewReader("")
	var output bytes.Buffer

	resultText, usage, err := processCodexStream(reader, &output, nil, nil)

	if err != nil {
		t.Fatalf("processCodexStream() error = %v, want nil", err)
	}

	if resultText != "" {
		t.Errorf("resultText = %q, want empty string", resultText)
	}

	if usage != nil {
		t.Errorf("usage = %v, want nil for empty input", usage)
	}
}

// TestProcessCodexStreamThreadStartedEvent verifies that thread.started events invoke handler.
// Red: processCodexStream does not emit events for thread.started yet
func TestProcessCodexStreamThreadStartedEvent(t *testing.T) {
	input := `{"type":"thread.started"}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer
	var handlerCalled bool

	handler := func(line []byte) {
		handlerCalled = true
	}

	_, _, err := processCodexStream(reader, &output, handler, nil)

	if err != nil {
		t.Fatalf("processCodexStream() error = %v, want nil", err)
	}

	if !handlerCalled {
		t.Error("EventHandler should be called for thread.started event")
	}
}

// TestProcessCodexStreamAgentMessageEvent verifies that item.completed agent_message
// emits assistant event and extracts result text.
// Red: processCodexStream does not handle agent_message events yet
func TestProcessCodexStreamAgentMessageEvent(t *testing.T) {
	input := `{"type":"item.completed","item":{"type":"agent_message","text":"Hello world"}}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer
	var handlerCalled bool

	handler := func(line []byte) {
		handlerCalled = true
	}

	resultText, _, err := processCodexStream(reader, &output, handler, nil)

	if err != nil {
		t.Fatalf("processCodexStream() error = %v, want nil", err)
	}

	if !handlerCalled {
		t.Error("EventHandler should be called for agent_message event")
	}

	if resultText != "Hello world" {
		t.Errorf("resultText = %q, want %q", resultText, "Hello world")
	}

	if !strings.Contains(output.String(), "Hello world") {
		t.Errorf("output = %q, want it to contain agent message text", output.String())
	}
}

// TestProcessCodexStreamToolCallCommand verifies that command_execution invokes ToolCallHandler.
// Red: processCodexStream does not handle command_execution events yet
func TestProcessCodexStreamToolCallCommand(t *testing.T) {
	input := `{"type":"item.started","item":{"type":"command_execution","command":"go test"}}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer
	var receivedToolCalls []ToolEvent

	toolHandler := func(event ToolEvent) {
		receivedToolCalls = append(receivedToolCalls, event)
	}

	_, _, err := processCodexStream(reader, &output, nil, toolHandler)

	if err != nil {
		t.Fatalf("processCodexStream() error = %v, want nil", err)
	}

	if len(receivedToolCalls) != 1 {
		t.Fatalf("len(receivedToolCalls) = %d, want 1", len(receivedToolCalls))
	}

	if receivedToolCalls[0].ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", receivedToolCalls[0].ToolName, "Bash")
	}

	if !strings.Contains(receivedToolCalls[0].FilePath, "go test") {
		t.Errorf("FilePath = %q, want it to contain command", receivedToolCalls[0].FilePath)
	}

	if receivedToolCalls[0].Timestamp.IsZero() {
		t.Error("Timestamp should be non-zero")
	}
}

// TestProcessCodexStreamUsageExtraction verifies that turn.completed extracts usage.
// Red: processCodexStream does not extract usage from turn.completed yet
func TestProcessCodexStreamUsageExtraction(t *testing.T) {
	input := `{"type":"turn.completed","usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":500}}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	_, usage, err := processCodexStream(reader, &output, nil, nil)

	if err != nil {
		t.Fatalf("processCodexStream() error = %v, want nil", err)
	}

	if usage == nil {
		t.Fatal("usage is nil, want non-nil")
	}

	if usage.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", usage.InputTokens)
	}

	if usage.CachedInputTokens != 200 {
		t.Errorf("CachedInputTokens = %d, want 200", usage.CachedInputTokens)
	}

	if usage.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", usage.OutputTokens)
	}
}

// TestStreamRunWithHandlerAddsJSONFlag verifies that StreamRun adds --json flag when handler is non-nil.
// Red: StreamRun does not add --json flag conditionally yet
func TestStreamRunWithHandlerAddsJSONFlag(t *testing.T) {
	tempDir := t.TempDir()

	// Mock binary that emits JSON only if --json is present
	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
if [[ "$*" == *"--json"* ]]; then
    echo '{"type":"thread.started"}'
else
    echo "plain text"
fi
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	var handlerCalled bool

	// With handler, should add --json and call handler
	result, err := cp.StreamRun(ctx, "test", TierMedium, &output, func([]byte) {
		handlerCalled = true
	}, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	if !handlerCalled {
		t.Error("StreamRun() with handler should call EventHandler when --json produces events")
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}
}

// TestStreamRunWithoutHandlerOmitsJSONFlag verifies that StreamRun does not add --json when handler is nil.
// Red: StreamRun does not conditionally omit --json yet
func TestStreamRunWithoutHandlerOmitsJSONFlag(t *testing.T) {
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
	cp := NewCodexProvider(mockBinary, []string{}, "prompt_file_arg", "--prompt", tierMap)

	ctx := context.Background()
	var output bytes.Buffer

	// Without handler, should NOT add --json
	result, err := cp.StreamRun(ctx, "test", TierMedium, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	if strings.Contains(result.Output, "--json") {
		t.Errorf("StreamRun() without handler should not add --json flag, output: %s", result.Output)
	}
}
