package provider

import (
	"bytes"
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
