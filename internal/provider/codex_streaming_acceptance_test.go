//go:build acceptance

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCodexProviderStreamRunWithJSONFlag verifies that StreamRun() invokes
// codex exec with --json flag when EventHandler is non-nil.
func TestCodexProviderStreamRunWithJSONFlag(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock codex binary that echoes arguments as JSON (for handler mode)
	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := `#!/bin/bash
cat > /dev/null  # Consume stdin
# Emit JSON with args embedded
echo '{"type":"item.completed","item":{"type":"agent_message","text":"ARGS: '"$*"'"}}'
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

// TestCodexProviderStreamRunNilHandlerStillUsesJSONFlag verifies that StreamRun()
// still adds --json when EventHandler is nil, so usage/cost events are available.
func TestCodexProviderStreamRunNilHandlerStillUsesJSONFlag(t *testing.T) {
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

	// EventHandler is nil, but --json is still required for usage/cost extraction.
	result, err := cp.StreamRun(ctx, "test prompt", TierMedium, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	// Verify that --json flag is still passed when handler is nil.
	outputStr := result.Output + output.String()
	if !strings.Contains(outputStr, "--json") {
		t.Errorf("StreamRun() with nil EventHandler should pass --json flag for usage/cost tracking, output: %s", outputStr)
	}
}

// TestCodexProviderParsesThreadStartedEvent verifies that processCodexStream
// converts thread.started events to StreamEvent with type "system".
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
	mockScript := fmt.Sprintf("#!/bin/bash\ncat > /dev/null\nprintf '%%s\\n' '%s'\nexit 0\n", string(eventJSON))
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
func TestCodexProviderParsesAgentMessageEvent(t *testing.T) {
	tempDir := t.TempDir()

	mockBinary := filepath.Join(tempDir, "codex")
	codexEvent := map[string]interface{}{
		"type": "item.completed",
		"item": map[string]interface{}{
			"type": "agent_message",
			"text": "This is the agent response.",
		},
	}
	eventJSON, _ := json.Marshal(codexEvent)
	// Use printf with %s to avoid bash quoting issues with JSON content
	mockScript := fmt.Sprintf("#!/bin/bash\ncat > /dev/null\nprintf '%%s\\n' '%s'\nexit 0\n", string(eventJSON))
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

	if !strings.Contains(streamEvent.Message.Content[0].Text, "agent response") {
		t.Errorf("content text = %q, want it to contain %q", streamEvent.Message.Content[0].Text, "agent response")
	}
}

// TestCodexProviderInvokesToolCallHandlerForCommandExecution verifies that
// item.started events with type "command_execution" trigger ToolCallHandler.
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
	mockScript := fmt.Sprintf("#!/bin/bash\ncat > /dev/null\nprintf '%%s\\n' '%s'\nexit 0\n", string(eventJSON))
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
	mockScript := fmt.Sprintf("#!/bin/bash\ncat > /dev/null\nprintf '%%s\\n' '%s'\nexit 0\n", string(eventJSON))
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
	mockScript := fmt.Sprintf("#!/bin/bash\ncat > /dev/null\nprintf '%%s\\n' '%s'\nexit 0\n", string(eventJSON))
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
	mockScript := fmt.Sprintf("#!/bin/bash\ncat > /dev/null\nprintf '%%s\\n' '%s'\nexit 0\n", string(eventJSON))
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

// TestCodexProviderExtractsAgentTextFromItemCompleted verifies that
// Result.Output contains the text from item.completed agent_message events.
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

// TestCodexProviderStreamRunCreatesTimestampedToolEvents verifies that
// ToolEvent structs created from Codex events have a Timestamp field populated.
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
	mockScript := fmt.Sprintf("#!/bin/bash\ncat > /dev/null\nprintf '%%s\\n' '%s'\nexit 0\n", string(eventJSON))
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
