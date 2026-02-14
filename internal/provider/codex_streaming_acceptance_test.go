//go:build acceptance

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCodexProviderStreamRunWithJSONFlag verifies that StreamRun() invokes
// codex exec with --json flag when EventHandler is non-nil.
// Expected failure: CodexProvider.StreamRun does not add --json flag based on EventHandler presence yet
func TestCodexProviderStreamRunWithJSONFlag(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock codex binary that echoes its arguments
	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "ARGS: $@"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer

	// EventHandler is non-nil, so --json flag should be added
	handler := func(line []byte) {
		// Handler called for events
	}

	result, err := cp.StreamRun(ctx, "test prompt", TierMedium, &output, handler, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	// Verify that --json flag was passed to the command
	if !strings.Contains(result.Output, "--json") && !strings.Contains(output.String(), "--json") {
		t.Errorf("StreamRun() with non-nil EventHandler should pass --json flag, output: %s", result.Output)
	}
}

// TestCodexProviderStreamRunWithoutJSONFlag verifies that StreamRun() does NOT
// add --json flag when EventHandler is nil.
// Expected failure: CodexProvider.StreamRun does not conditionally add --json flag yet
func TestCodexProviderStreamRunWithoutJSONFlag(t *testing.T) {
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
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer

	// EventHandler is nil, so --json flag should NOT be added
	result, err := cp.StreamRun(ctx, "test prompt", TierMedium, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	// Verify that --json flag was NOT passed when handler is nil
	outputStr := result.Output + output.String()
	if strings.Contains(outputStr, "--json") {
		t.Errorf("StreamRun() with nil EventHandler should not pass --json flag, but output contains it: %s", outputStr)
	}
}

// TestCodexProviderParsesThreadStartedEvent verifies that processCodexStream
// converts thread.started events to StreamEvent with type "system".
// Expected failure: processCodexStream function does not exist yet
func TestCodexProviderParsesThreadStartedEvent(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	codexEvent := map[string]interface{}{
		"type": "thread.started",
		"data": map[string]interface{}{
			"thread_id": "thread-123",
		},
	}
	eventJSON, _ := json.Marshal(codexEvent)
	mockScript := "#!/bin/bash\necho '" + string(eventJSON) + "'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	var receivedEvents [][]byte

	handler := func(line []byte) {
		receivedEvents = append(receivedEvents, line)
	}

	_, err := cp.StreamRun(ctx, "test", TierMedium, &output, handler, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	// Verify that handler was called and event was normalized to StreamEvent format
	if len(receivedEvents) == 0 {
		t.Fatal("EventHandler was not called for thread.started event")
	}

	// Parse the normalized event and verify it's type "system"
	var streamEvent struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(receivedEvents[0], &streamEvent); err != nil {
		t.Fatalf("failed to parse normalized event: %v", err)
	}

	if streamEvent.Type != "system" {
		t.Errorf("thread.started event normalized to type %q, want %q", streamEvent.Type, "system")
	}
}

// TestCodexProviderParsesAgentMessageEvent verifies that item.completed events
// with type "agent_message" are converted to StreamEvent type "assistant".
// Expected failure: processCodexStream does not convert agent_message events to assistant type yet
func TestCodexProviderParsesAgentMessageEvent(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	codexEvent := map[string]interface{}{
		"type": "item.completed",
		"item": map[string]interface{}{
			"type": "agent_message",
			"text": "This is the agent's response.",
		},
	}
	eventJSON, _ := json.Marshal(codexEvent)
	mockScript := "#!/bin/bash\necho '" + string(eventJSON) + "'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	var receivedEvents [][]byte

	handler := func(line []byte) {
		receivedEvents = append(receivedEvents, line)
	}

	_, err := cp.StreamRun(ctx, "test", TierMedium, &output, handler, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	if len(receivedEvents) == 0 {
		t.Fatal("EventHandler was not called for item.completed agent_message event")
	}

	// Parse the normalized event
	var streamEvent struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message,omitempty"`
	}
	if err := json.Unmarshal(receivedEvents[0], &streamEvent); err != nil {
		t.Fatalf("failed to parse normalized event: %v", err)
	}

	if streamEvent.Type != "assistant" {
		t.Errorf("agent_message event normalized to type %q, want %q", streamEvent.Type, "assistant")
	}

	// Verify text content is present
	if len(streamEvent.Message.Content) == 0 {
		t.Fatal("agent_message event missing content blocks")
	}

	if streamEvent.Message.Content[0].Type != "text" {
		t.Errorf("content block type = %q, want %q", streamEvent.Message.Content[0].Type, "text")
	}

	if !strings.Contains(streamEvent.Message.Content[0].Text, "agent's response") {
		t.Errorf("content text = %q, want it to contain %q", streamEvent.Message.Content[0].Text, "agent's response")
	}
}

// TestCodexProviderInvokesToolCallHandlerForCommandExecution verifies that
// item.started events with type "command_execution" trigger ToolCallHandler.
// Expected failure: processCodexStream does not invoke ToolCallHandler for command_execution events yet
func TestCodexProviderInvokesToolCallHandlerForCommandExecution(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	codexEvent := map[string]interface{}{
		"type": "item.started",
		"item": map[string]interface{}{
			"type":    "command_execution",
			"command": "go test ./...",
		},
	}
	eventJSON, _ := json.Marshal(codexEvent)
	mockScript := "#!/bin/bash\necho '" + string(eventJSON) + "'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	var receivedToolCalls []ToolEvent

	toolHandler := func(event ToolEvent) {
		receivedToolCalls = append(receivedToolCalls, event)
	}

	_, err := cp.StreamRun(ctx, "test", TierMedium, &output, func(line []byte) {}, toolHandler)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	if len(receivedToolCalls) == 0 {
		t.Fatal("ToolCallHandler was not invoked for command_execution event")
	}

	toolEvent := receivedToolCalls[0]
	if toolEvent.ToolName != "Bash" {
		t.Errorf("ToolEvent.ToolName = %q, want %q", toolEvent.ToolName, "Bash")
	}

	if !strings.Contains(toolEvent.FilePath, "go test") {
		t.Errorf("ToolEvent.FilePath = %q, want it to contain command string", toolEvent.FilePath)
	}
}

// TestCodexProviderInvokesToolCallHandlerForFileChange verifies that
// item.started events with type "file_change" trigger ToolCallHandler with "Write".
// Expected failure: processCodexStream does not invoke ToolCallHandler for file_change events yet
func TestCodexProviderInvokesToolCallHandlerForFileChange(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	codexEvent := map[string]interface{}{
		"type": "item.started",
		"item": map[string]interface{}{
			"type": "file_change",
			"path": "/home/user/project/main.go",
		},
	}
	eventJSON, _ := json.Marshal(codexEvent)
	mockScript := "#!/bin/bash\necho '" + string(eventJSON) + "'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	var receivedToolCalls []ToolEvent

	toolHandler := func(event ToolEvent) {
		receivedToolCalls = append(receivedToolCalls, event)
	}

	_, err := cp.StreamRun(ctx, "test", TierMedium, &output, func(line []byte) {}, toolHandler)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	if len(receivedToolCalls) == 0 {
		t.Fatal("ToolCallHandler was not invoked for file_change event")
	}

	toolEvent := receivedToolCalls[0]
	if toolEvent.ToolName != "Write" {
		t.Errorf("ToolEvent.ToolName = %q, want %q for file_change", toolEvent.ToolName, "Write")
	}

	if toolEvent.FilePath != "/home/user/project/main.go" {
		t.Errorf("ToolEvent.FilePath = %q, want %q", toolEvent.FilePath, "/home/user/project/main.go")
	}
}

// TestCodexProviderInvokesToolCallHandlerForMCPTool verifies that
// item.started events with type "mcp_tool_call" trigger ToolCallHandler.
// Expected failure: processCodexStream does not invoke ToolCallHandler for mcp_tool_call events yet
func TestCodexProviderInvokesToolCallHandlerForMCPTool(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	codexEvent := map[string]interface{}{
		"type": "item.started",
		"item": map[string]interface{}{
			"type":      "mcp_tool_call",
			"tool_name": "github_create_issue",
		},
	}
	eventJSON, _ := json.Marshal(codexEvent)
	mockScript := "#!/bin/bash\necho '" + string(eventJSON) + "'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	var receivedToolCalls []ToolEvent

	toolHandler := func(event ToolEvent) {
		receivedToolCalls = append(receivedToolCalls, event)
	}

	_, err := cp.StreamRun(ctx, "test", TierMedium, &output, func(line []byte) {}, toolHandler)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	if len(receivedToolCalls) == 0 {
		t.Fatal("ToolCallHandler was not invoked for mcp_tool_call event")
	}

	toolEvent := receivedToolCalls[0]
	if toolEvent.ToolName != "github_create_issue" {
		t.Errorf("ToolEvent.ToolName = %q, want %q", toolEvent.ToolName, "github_create_issue")
	}
}

// TestCodexProviderExtractsTokenUsageFromTurnCompleted verifies that
// turn.completed events with usage data populate Result's token fields.
// Expected failure: processCodexStream does not extract token usage from turn.completed events yet
func TestCodexProviderExtractsTokenUsageFromTurnCompleted(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	codexEvent := map[string]interface{}{
		"type": "turn.completed",
		"usage": map[string]interface{}{
			"input_tokens":  1500,
			"output_tokens": 800,
		},
	}
	eventJSON, _ := json.Marshal(codexEvent)
	mockScript := "#!/bin/bash\necho '" + string(eventJSON) + "'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	var receivedEvents [][]byte

	handler := func(line []byte) {
		receivedEvents = append(receivedEvents, line)
	}

	result, err := cp.StreamRun(ctx, "test", TierMedium, &output, handler, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	// Verify that a result event was emitted with token usage
	var foundResultEvent bool
	for _, eventData := range receivedEvents {
		var streamEvent struct {
			Type         string `json:"type"`
			InputTokens  int    `json:"input_tokens,omitempty"`
			OutputTokens int    `json:"output_tokens,omitempty"`
		}
		if err := json.Unmarshal(eventData, &streamEvent); err != nil {
			continue
		}
		if streamEvent.Type == "result" {
			foundResultEvent = true
			if streamEvent.InputTokens != 1500 {
				t.Errorf("result event InputTokens = %d, want 1500", streamEvent.InputTokens)
			}
			if streamEvent.OutputTokens != 800 {
				t.Errorf("result event OutputTokens = %d, want 800", streamEvent.OutputTokens)
			}
			break
		}
	}

	if !foundResultEvent {
		t.Error("turn.completed event did not produce a result StreamEvent with token usage")
	}

	// Also verify Result object has the token data (if Result includes these fields)
	_ = result
}

// TestCodexProviderDetectsUsageLimitErrorFromJSON verifies that
// IsUsageLimitError detects UsageLimitExceeded from turn.completed error events.
// Expected failure: CodexProvider.IsUsageLimitError does not check for structured UsageLimitExceeded yet
func TestCodexProviderDetectsUsageLimitErrorFromJSON(t *testing.T) {
	// This test verifies that when --json streaming is active and we detect
	// a UsageLimitExceeded error, IsUsageLimitError returns true.
	// The Result object should contain information about the error.

	cp := &CodexProvider{}

	result := &Result{
		Success:  false,
		ExitCode: 1,
		Output:   `{"type":"turn.completed","status":"failed","error":{"type":"UsageLimitExceeded","message":"Rate limit exceeded"}}`,
	}

	// When JSON streaming is active, IsUsageLimitError should detect UsageLimitExceeded
	// from the structured error info, not just string matching
	if !cp.IsUsageLimitError(result, nil) {
		t.Error("IsUsageLimitError() should detect UsageLimitExceeded from JSON error event")
	}
}

// TestCodexProviderRunValidationConstructsPrompt verifies that
// CodexProvider.RunValidation builds a validation prompt with numbered commands.
// Expected failure: CodexProvider.RunValidation is not implemented yet
func TestCodexProviderRunValidationConstructsPrompt(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	// Mock binary that echoes the prompt file content to verify prompt structure
	mockScript := `#!/bin/bash
PROMPT_FILE=""
for i in "$@"; do
    if [ "$prev" = "--prompt" ]; then
        PROMPT_FILE="$i"
        break
    fi
    prev="$i"
done

if [ -f "$PROMPT_FILE" ]; then
    cat "$PROMPT_FILE"
fi
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	commands := []string{"go test ./...", "go vet ./...", "golangci-lint run"}
	workDir := "/home/user/project"

	result, err := cp.RunValidation(ctx, commands, TierLow, workDir)

	if err != nil {
		t.Fatalf("RunValidation() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("RunValidation() returned nil result")
	}

	// Verify the prompt contains numbered commands
	if !strings.Contains(result.Output, "1. go test ./...") {
		t.Errorf("RunValidation() prompt missing numbered command 1, output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "2. go vet ./...") {
		t.Errorf("RunValidation() prompt missing numbered command 2, output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "3. golangci-lint run") {
		t.Errorf("RunValidation() prompt missing numbered command 3, output: %s", result.Output)
	}

	// Verify the prompt contains VALIDATION_PASSED/FAILED markers
	if !strings.Contains(result.Output, "VALIDATION_PASSED") {
		t.Errorf("RunValidation() prompt missing VALIDATION_PASSED marker, output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "VALIDATION_FAILED") {
		t.Errorf("RunValidation() prompt missing VALIDATION_FAILED marker, output: %s", result.Output)
	}

	// Verify work directory is mentioned
	if !strings.Contains(result.Output, workDir) {
		t.Errorf("RunValidation() prompt missing work directory, output: %s", result.Output)
	}
}

// TestCodexProviderRunValidationDetectsValidationPassed verifies that
// RunValidation correctly detects VALIDATION_PASSED marker in output.
// Expected failure: CodexProvider.RunValidation does not check for VALIDATION_PASSED marker yet
func TestCodexProviderRunValidationDetectsValidationPassed(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "Running tests..."
echo "All tests passed"
echo "VALIDATION_PASSED"
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	commands := []string{"go test ./..."}

	result, err := cp.RunValidation(ctx, commands, TierLow, "/tmp")
	if err != nil {
		t.Fatalf("RunValidation() error = %v", err)
	}

	// Use the shared IsValidationPassed helper to check the result
	if !IsValidationPassed(result) {
		t.Errorf("RunValidation() with VALIDATION_PASSED marker should be detected as passed, output: %s", result.Output)
	}
}

// TestCodexProviderRunValidationDetectsValidationFailed verifies that
// RunValidation correctly detects VALIDATION_FAILED marker in output.
// Expected failure: CodexProvider.RunValidation does not check for VALIDATION_FAILED marker yet
func TestCodexProviderRunValidationDetectsValidationFailed(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo "Running tests..."
echo "Test failed: some error"
echo "VALIDATION_FAILED"
exit 1
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	commands := []string{"go test ./..."}

	result, err := cp.RunValidation(ctx, commands, TierLow, "/tmp")
	if err != nil {
		t.Fatalf("RunValidation() error = %v", err)
	}

	// Use the shared IsValidationPassed helper - should return false
	if IsValidationPassed(result) {
		t.Errorf("RunValidation() with VALIDATION_FAILED marker should not be detected as passed, output: %s", result.Output)
	}

	// Verify output contains the VALIDATION_FAILED marker
	if !strings.Contains(result.Output, "VALIDATION_FAILED") {
		t.Errorf("RunValidation() output missing VALIDATION_FAILED marker: %s", result.Output)
	}
}

// TestCodexProviderExtractsAgentTextFromItemCompleted verifies that
// Result.Output contains the text from item.completed agent_message events.
// Expected failure: processCodexStream does not extract text from agent_message events to Result.Output yet
func TestCodexProviderExtractsAgentTextFromItemCompleted(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	// Emit multiple agent message events - last one should be in Result.Output
	mockScript := `#!/bin/bash
echo '{"type":"item.completed","item":{"type":"agent_message","text":"First response"}}'
echo '{"type":"item.completed","item":{"type":"agent_message","text":"Second response"}}'
echo '{"type":"item.completed","item":{"type":"agent_message","text":"Final answer"}}'
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer

	result, err := cp.StreamRun(ctx, "test", TierMedium, &output, func(line []byte) {}, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	// Result.Output should contain the text from agent_message events
	// (spec says "last such event's text field becomes Result.Output")
	if !strings.Contains(result.Output, "Final answer") {
		t.Errorf("Result.Output = %q, want it to contain text from agent_message events", result.Output)
	}
}

// TestCodexProviderStreamsAgentTextToWriter verifies that agent message text
// is written to the output writer in real-time as events arrive.
// Expected failure: processCodexStream does not write agent text to output writer yet
func TestCodexProviderStreamsAgentTextToWriter(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo '{"type":"item.agentMessage.delta","delta":{"text":"Hello "}}'
sleep 0.05
echo '{"type":"item.agentMessage.delta","delta":{"text":"world"}}'
sleep 0.05
echo '{"type":"item.completed","item":{"type":"agent_message","text":"Hello world"}}'
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer

	_, err := cp.StreamRun(ctx, "test", TierMedium, &output, func(line []byte) {}, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Hello") {
		t.Errorf("output writer missing agent message text, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "world") {
		t.Errorf("output writer missing full agent message text, got: %s", outputStr)
	}
}

// TestCodexProviderHandlesMultipleEventTypes verifies that processCodexStream
// correctly handles a mix of different Codex event types in a single stream.
// Expected failure: processCodexStream does not exist or does not handle all event types correctly yet
func TestCodexProviderHandlesMultipleEventTypes(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
echo '{"type":"thread.started","data":{"thread_id":"t1"}}'
echo '{"type":"item.started","item":{"type":"command_execution","command":"ls"}}'
echo '{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}'
exit 0
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	var receivedEvents [][]byte
	var receivedToolCalls []ToolEvent

	handler := func(line []byte) {
		receivedEvents = append(receivedEvents, line)
	}

	toolHandler := func(event ToolEvent) {
		receivedToolCalls = append(receivedToolCalls, event)
	}

	result, err := cp.StreamRun(ctx, "test", TierMedium, &output, handler, toolHandler)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	// Verify multiple event types were processed
	if len(receivedEvents) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(receivedEvents))
	}

	if len(receivedToolCalls) < 1 {
		t.Error("expected at least 1 tool call for command_execution")
	}

	// Verify result contains agent text
	if !strings.Contains(result.Output, "Done") {
		t.Errorf("Result.Output missing agent message text: %s", result.Output)
	}
}

// TestProviderInterfaceIncludesValidationMethods verifies that the Provider
// interface includes IsValidationPassed and IsScopeTooLarge methods.
// Expected failure: Provider interface does not include these methods yet
func TestProviderInterfaceIncludesValidationMethods(t *testing.T) {
	// This test verifies the interface signature by attempting to call
	// the methods through the Provider interface
	var p Provider

	// Assign a CodexProvider to the interface variable
	p = &CodexProvider{}

	// These calls should compile when the interface includes the methods
	_ = p.IsValidationPassed(&Result{})
	_, _ = p.IsScopeTooLarge(&Result{})
}

// TestCodexEventStructsExist verifies that codexEvent, codexItem, codexUsage,
// and codexErrorInfo structs exist for parsing Codex JSON events.
// Expected failure: codexEvent struct and related types do not exist yet
func TestCodexEventStructsExist(t *testing.T) {
	// Test that we can unmarshal a Codex event into the structs
	jsonEvent := `{
		"type": "turn.completed",
		"status": "failed",
		"usage": {
			"input_tokens": 1000,
			"output_tokens": 500
		},
		"error": {
			"type": "UsageLimitExceeded",
			"message": "Rate limit exceeded"
		}
	}`

	var event codexEvent
	if err := json.Unmarshal([]byte(jsonEvent), &event); err != nil {
		t.Fatalf("failed to unmarshal into codexEvent: %v", err)
	}

	if event.Type != "turn.completed" {
		t.Errorf("event.Type = %q, want %q", event.Type, "turn.completed")
	}

	if event.Usage == nil {
		t.Fatal("event.Usage is nil, want non-nil")
	}

	if event.Usage.InputTokens != 1000 {
		t.Errorf("event.Usage.InputTokens = %d, want 1000", event.Usage.InputTokens)
	}

	if event.Usage.OutputTokens != 500 {
		t.Errorf("event.Usage.OutputTokens = %d, want 500", event.Usage.OutputTokens)
	}

	if event.ErrorInfo == nil {
		t.Fatal("event.ErrorInfo is nil, want non-nil")
	}

	if event.ErrorInfo.Type != "UsageLimitExceeded" {
		t.Errorf("event.ErrorInfo.Type = %q, want %q", event.ErrorInfo.Type, "UsageLimitExceeded")
	}
}

// TestProcessCodexStreamFunctionSignature verifies that processCodexStream
// exists with the expected signature.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamFunctionSignature(t *testing.T) {
	// Create a mock stream to process
	mockStream := strings.NewReader(`{"type":"thread.started"}`)
	var outputBuf bytes.Buffer

	handlerCalled := false
	handler := func(line []byte) {
		handlerCalled = true
	}

	toolHandler := func(event ToolEvent) {
		// Tool handler for events
	}

	// Call processCodexStream
	resultText, usage, err := processCodexStream(mockStream, &outputBuf, handler, toolHandler)

	if err != nil {
		t.Errorf("processCodexStream() error = %v, want nil", err)
	}

	// Verify return values exist
	_ = resultText
	_ = usage

	// Handler should have been called for the event
	if !handlerCalled {
		t.Error("EventHandler was not called by processCodexStream")
	}
}

// TestCodexUsageStructFields verifies that codexUsage struct has the
// correct fields for token tracking.
// Expected failure: codexUsage struct does not exist yet
func TestCodexUsageStructFields(t *testing.T) {
	jsonUsage := `{
		"input_tokens": 1234,
		"cached_input_tokens": 567,
		"output_tokens": 890
	}`

	var usage codexUsage
	if err := json.Unmarshal([]byte(jsonUsage), &usage); err != nil {
		t.Fatalf("failed to unmarshal into codexUsage: %v", err)
	}

	if usage.InputTokens != 1234 {
		t.Errorf("usage.InputTokens = %d, want 1234", usage.InputTokens)
	}

	if usage.CachedInputTokens != 567 {
		t.Errorf("usage.CachedInputTokens = %d, want 567", usage.CachedInputTokens)
	}

	if usage.OutputTokens != 890 {
		t.Errorf("usage.OutputTokens = %d, want 890", usage.OutputTokens)
	}
}

// TestCodexItemStructSupportsMultipleTypes verifies that codexItem struct
// can parse different item types (agent_message, command_execution, file_change, mcp_tool_call).
// Expected failure: codexItem struct does not exist or does not support all item types yet
func TestCodexItemStructSupportsMultipleTypes(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantType string
	}{
		{
			name:     "agent_message",
			json:     `{"type":"agent_message","text":"Hello"}`,
			wantType: "agent_message",
		},
		{
			name:     "command_execution",
			json:     `{"type":"command_execution","command":"ls -la"}`,
			wantType: "command_execution",
		},
		{
			name:     "file_change",
			json:     `{"type":"file_change","path":"/tmp/test.go"}`,
			wantType: "file_change",
		},
		{
			name:     "mcp_tool_call",
			json:     `{"type":"mcp_tool_call","tool_name":"web_search"}`,
			wantType: "mcp_tool_call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item codexItem
			if err := json.Unmarshal([]byte(tt.json), &item); err != nil {
				t.Fatalf("failed to unmarshal %s item: %v", tt.name, err)
			}

			if item.Type != tt.wantType {
				t.Errorf("item.Type = %q, want %q", item.Type, tt.wantType)
			}
		})
	}
}

// TestCodexProviderStreamRunCreatesTimestampedToolEvents verifies that
// ToolEvent structs created from Codex events have a Timestamp field populated.
// Expected failure: processCodexStream does not populate ToolEvent.Timestamp yet
func TestCodexProviderStreamRunCreatesTimestampedToolEvents(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	codexEvent := map[string]interface{}{
		"type": "item.started",
		"item": map[string]interface{}{
			"type":    "command_execution",
			"command": "go test",
		},
	}
	eventJSON, _ := json.Marshal(codexEvent)
	mockScript := "#!/bin/bash\necho '" + string(eventJSON) + "'\nexit 0\n"
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	var receivedToolCalls []ToolEvent

	toolHandler := func(event ToolEvent) {
		receivedToolCalls = append(receivedToolCalls, event)
	}

	startTime := time.Now()
	_, err := cp.StreamRun(ctx, "test", TierMedium, &output, func(line []byte) {}, toolHandler)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}

	if len(receivedToolCalls) == 0 {
		t.Fatal("no tool calls received")
	}

	toolEvent := receivedToolCalls[0]
	if toolEvent.Timestamp.IsZero() {
		t.Error("ToolEvent.Timestamp is zero, want non-zero timestamp")
	}

	// Timestamp should be after start time and not too far in the future
	if toolEvent.Timestamp.Before(startTime) {
		t.Errorf("ToolEvent.Timestamp = %v is before test start %v", toolEvent.Timestamp, startTime)
	}

	if toolEvent.Timestamp.After(time.Now().Add(1 * time.Second)) {
		t.Errorf("ToolEvent.Timestamp = %v is too far in the future", toolEvent.Timestamp)
	}
}
