//go:build acceptance

package provider

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestProcessCodexStreamMapsThreadStartedToSystem verifies that processCodexStream
// maps thread.started events to StreamEvent type "system" and calls EventHandler.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamMapsThreadStartedToSystem(t *testing.T) {
	input := `{"type":"thread.started","data":{"thread_id":"t-abc"}}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer
	var receivedEvents [][]byte

	handler := func(line []byte) {
		cp := make([]byte, len(line))
		copy(cp, line)
		receivedEvents = append(receivedEvents, cp)
	}

	_, _, err := processCodexStream(reader, &output, handler, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if len(receivedEvents) == 0 {
		t.Fatal("EventHandler was not called for thread.started event")
	}

	var parsed struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(receivedEvents[0], &parsed); err != nil {
		t.Fatalf("failed to parse emitted event: %v", err)
	}

	if parsed.Type != "system" {
		t.Errorf("thread.started mapped to type %q, want %q", parsed.Type, "system")
	}
}

// TestProcessCodexStreamMapsAgentMessageToAssistant verifies that item.completed
// events with type "agent_message" are mapped to StreamEvent type "assistant"
// with a text content block.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamMapsAgentMessageToAssistant(t *testing.T) {
	input := `{"type":"item.completed","item":{"type":"agent_message","text":"Hello from agent"}}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer
	var receivedEvents [][]byte

	handler := func(line []byte) {
		cp := make([]byte, len(line))
		copy(cp, line)
		receivedEvents = append(receivedEvents, cp)
	}

	resultText, _, err := processCodexStream(reader, &output, handler, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if len(receivedEvents) == 0 {
		t.Fatal("EventHandler was not called for agent_message event")
	}

	var parsed struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(receivedEvents[0], &parsed); err != nil {
		t.Fatalf("failed to parse emitted event: %v", err)
	}

	if parsed.Type != "assistant" {
		t.Errorf("agent_message mapped to type %q, want %q", parsed.Type, "assistant")
	}
	if len(parsed.Message.Content) == 0 {
		t.Fatal("agent_message event missing content blocks")
	}
	if parsed.Message.Content[0].Type != "text" {
		t.Errorf("content block type = %q, want %q", parsed.Message.Content[0].Type, "text")
	}
	if parsed.Message.Content[0].Text != "Hello from agent" {
		t.Errorf("content text = %q, want %q", parsed.Message.Content[0].Text, "Hello from agent")
	}

	// resultText should contain the agent message text
	if resultText != "Hello from agent" {
		t.Errorf("resultText = %q, want %q", resultText, "Hello from agent")
	}
}

// TestProcessCodexStreamMapsTurnCompletedToResult verifies that turn.completed
// events with usage data are mapped to StreamEvent type "result" with token counts.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamMapsTurnCompletedToResult(t *testing.T) {
	input := `{"type":"turn.completed","usage":{"input_tokens":2500,"output_tokens":1200,"cached_input_tokens":400}}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer
	var receivedEvents [][]byte

	handler := func(line []byte) {
		cp := make([]byte, len(line))
		copy(cp, line)
		receivedEvents = append(receivedEvents, cp)
	}

	_, usage, err := processCodexStream(reader, &output, handler, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	// Verify usage is extracted
	if usage == nil {
		t.Fatal("processCodexStream() returned nil usage for turn.completed with usage data")
	}
	if usage.InputTokens != 2500 {
		t.Errorf("usage.InputTokens = %d, want 2500", usage.InputTokens)
	}
	if usage.OutputTokens != 1200 {
		t.Errorf("usage.OutputTokens = %d, want 1200", usage.OutputTokens)
	}
	if usage.CachedInputTokens != 400 {
		t.Errorf("usage.CachedInputTokens = %d, want 400", usage.CachedInputTokens)
	}

	// Verify result event was emitted to handler
	if len(receivedEvents) == 0 {
		t.Fatal("EventHandler was not called for turn.completed event")
	}

	var parsed struct {
		Type         string `json:"type"`
		InputTokens  int    `json:"input_tokens"`
		OutputTokens int    `json:"output_tokens"`
	}
	if err := json.Unmarshal(receivedEvents[0], &parsed); err != nil {
		t.Fatalf("failed to parse emitted event: %v", err)
	}

	if parsed.Type != "result" {
		t.Errorf("turn.completed mapped to type %q, want %q", parsed.Type, "result")
	}
	if parsed.InputTokens != 2500 {
		t.Errorf("emitted event InputTokens = %d, want 2500", parsed.InputTokens)
	}
	if parsed.OutputTokens != 1200 {
		t.Errorf("emitted event OutputTokens = %d, want 1200", parsed.OutputTokens)
	}
}

// TestProcessCodexStreamToolCallHandlers verifies that processCodexStream invokes
// ToolCallHandler with correct ToolEvent fields for each tool-related item type.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamToolCallHandlers(t *testing.T) {
	tests := []struct {
		name            string
		jsonLine        string
		wantToolName    string
		wantFilePathHas string
	}{
		{
			name:            "command_execution maps to Bash",
			jsonLine:        `{"type":"item.started","item":{"type":"command_execution","command":"go test ./..."}}`,
			wantToolName:    "Bash",
			wantFilePathHas: "go test",
		},
		{
			name:            "file_change maps to Write",
			jsonLine:        `{"type":"item.started","item":{"type":"file_change","path":"/src/main.go"}}`,
			wantToolName:    "Write",
			wantFilePathHas: "/src/main.go",
		},
		{
			name:            "mcp_tool_call uses tool_name",
			jsonLine:        `{"type":"item.started","item":{"type":"mcp_tool_call","tool_name":"github_search"}}`,
			wantToolName:    "github_search",
			wantFilePathHas: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.jsonLine + "\n"
			reader := strings.NewReader(input)
			var output bytes.Buffer
			var receivedToolCalls []ToolEvent

			toolHandler := func(event ToolEvent) {
				receivedToolCalls = append(receivedToolCalls, event)
			}

			_, _, err := processCodexStream(reader, &output, func([]byte) {}, toolHandler)
			if err != nil {
				t.Fatalf("processCodexStream() error = %v", err)
			}

			if len(receivedToolCalls) == 0 {
				t.Fatal("ToolCallHandler was not invoked")
			}

			tc := receivedToolCalls[0]
			if tc.ToolName != tt.wantToolName {
				t.Errorf("ToolEvent.ToolName = %q, want %q", tc.ToolName, tt.wantToolName)
			}
			if tt.wantFilePathHas != "" && !strings.Contains(tc.FilePath, tt.wantFilePathHas) {
				t.Errorf("ToolEvent.FilePath = %q, want it to contain %q", tc.FilePath, tt.wantFilePathHas)
			}
			if tc.Timestamp.IsZero() {
				t.Error("ToolEvent.Timestamp is zero, want non-zero")
			}
		})
	}
}

// TestProcessCodexStreamExtractsLastAgentMessageAsResult verifies that when
// multiple agent_message events are present, the last one's text becomes the result.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamExtractsLastAgentMessageAsResult(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"First thought"}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"Second thought"}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"Final answer"}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	resultText, _, err := processCodexStream(reader, &output, func([]byte) {}, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if resultText != "Final answer" {
		t.Errorf("resultText = %q, want %q (last agent_message)", resultText, "Final answer")
	}
}

// TestProcessCodexStreamWritesAgentTextToOutput verifies that agent message text
// is written to the output writer as it arrives.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamWritesAgentTextToOutput(t *testing.T) {
	input := `{"type":"item.completed","item":{"type":"agent_message","text":"Response text here"}}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	_, _, err := processCodexStream(reader, &output, func([]byte) {}, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if !strings.Contains(output.String(), "Response text here") {
		t.Errorf("output = %q, want it to contain agent message text", output.String())
	}
}

// TestProcessCodexStreamSkipsMalformedLines verifies that non-JSON lines are
// silently skipped without returning an error.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamSkipsMalformedLines(t *testing.T) {
	input := strings.Join([]string{
		"this is not json",
		`{"type":"item.completed","item":{"type":"agent_message","text":"Valid"}}`,
		"another bad line",
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	resultText, _, err := processCodexStream(reader, &output, func([]byte) {}, nil)
	if err != nil {
		t.Fatalf("processCodexStream() should skip malformed lines, got error: %v", err)
	}

	if resultText != "Valid" {
		t.Errorf("resultText = %q, want %q", resultText, "Valid")
	}
}

// TestProcessCodexStreamEmptyInput verifies that processCodexStream handles
// empty input gracefully.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamEmptyInput(t *testing.T) {
	reader := strings.NewReader("")
	var output bytes.Buffer

	resultText, usage, err := processCodexStream(reader, &output, func([]byte) {}, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error on empty input = %v", err)
	}

	if resultText != "" {
		t.Errorf("resultText = %q, want empty string for empty input", resultText)
	}

	// Usage may be nil for empty input
	_ = usage
}

// TestProcessCodexStreamDetectsUsageLimitExceeded verifies that turn.completed
// events with status "failed" and UsageLimitExceeded error type are detected.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamDetectsUsageLimitExceeded(t *testing.T) {
	input := `{"type":"turn.completed","status":"failed","error":{"type":"UsageLimitExceeded","message":"Rate limit exceeded"}}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer
	var receivedEvents [][]byte

	handler := func(line []byte) {
		cp := make([]byte, len(line))
		copy(cp, line)
		receivedEvents = append(receivedEvents, cp)
	}

	_, _, err := processCodexStream(reader, &output, handler, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	// Should emit a rate limit event
	if len(receivedEvents) == 0 {
		t.Fatal("EventHandler was not called for UsageLimitExceeded event")
	}

	// Verify the emitted event indicates a rate limit/error
	var parsed struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype,omitempty"`
	}
	if err := json.Unmarshal(receivedEvents[0], &parsed); err != nil {
		t.Fatalf("failed to parse emitted event: %v", err)
	}

	// The event should indicate this is an error/rate-limit situation
	if parsed.Type != "error" && parsed.Type != "result" {
		t.Errorf("UsageLimitExceeded event type = %q, want 'error' or 'result'", parsed.Type)
	}
}

// TestProcessCodexStreamNilHandlers verifies that processCodexStream handles
// nil EventHandler and ToolCallHandler gracefully without panicking.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamNilHandlers(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"thread.started"}`,
		`{"type":"item.started","item":{"type":"command_execution","command":"ls"}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	// Both handlers nil - should not panic
	resultText, usage, err := processCodexStream(reader, &output, nil, nil)
	if err != nil {
		t.Fatalf("processCodexStream() with nil handlers error = %v", err)
	}

	// Should still extract result text and usage even without handlers
	if resultText != "Done" {
		t.Errorf("resultText = %q, want %q", resultText, "Done")
	}
	if usage == nil {
		t.Fatal("usage is nil, want non-nil even with nil handlers")
	}
	if usage.InputTokens != 100 {
		t.Errorf("usage.InputTokens = %d, want 100", usage.InputTokens)
	}
}

// TestProcessCodexStreamFullConversation verifies end-to-end processing of a
// realistic Codex JSONL stream with multiple event types in sequence.
// Expected failure: processCodexStream function does not exist yet
func TestProcessCodexStreamFullConversation(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"thread.started","data":{"thread_id":"t-123"}}`,
		`{"type":"turn.started"}`,
		`{"type":"item.started","item":{"type":"command_execution","command":"go test ./..."}}`,
		`{"type":"item.completed","item":{"type":"command_execution","status":"completed","exit_code":0}}`,
		`{"type":"item.started","item":{"type":"file_change","path":"/src/handler.go"}}`,
		`{"type":"item.completed","item":{"type":"file_change","status":"completed"}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"I've fixed the bug and all tests pass."}}`,
		`{"type":"turn.completed","usage":{"input_tokens":3000,"output_tokens":1500,"cached_input_tokens":500}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer
	var eventCount int
	var toolCalls []ToolEvent

	handler := func(line []byte) {
		eventCount++
	}
	toolHandler := func(event ToolEvent) {
		toolCalls = append(toolCalls, event)
	}

	resultText, usage, err := processCodexStream(reader, &output, handler, toolHandler)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	// Should have processed multiple events
	if eventCount < 3 {
		t.Errorf("eventCount = %d, want at least 3", eventCount)
	}

	// Should have 2 tool calls: command_execution and file_change
	if len(toolCalls) < 2 {
		t.Errorf("len(toolCalls) = %d, want at least 2", len(toolCalls))
	}

	// Verify tool call types
	if len(toolCalls) >= 1 && toolCalls[0].ToolName != "Bash" {
		t.Errorf("toolCalls[0].ToolName = %q, want %q", toolCalls[0].ToolName, "Bash")
	}
	if len(toolCalls) >= 2 && toolCalls[1].ToolName != "Write" {
		t.Errorf("toolCalls[1].ToolName = %q, want %q", toolCalls[1].ToolName, "Write")
	}

	// Result text should be the agent message
	if resultText != "I've fixed the bug and all tests pass." {
		t.Errorf("resultText = %q, want agent message text", resultText)
	}

	// Usage should be populated
	if usage == nil {
		t.Fatal("usage is nil")
	}
	if usage.InputTokens != 3000 {
		t.Errorf("usage.InputTokens = %d, want 3000", usage.InputTokens)
	}
	if usage.OutputTokens != 1500 {
		t.Errorf("usage.OutputTokens = %d, want 1500", usage.OutputTokens)
	}

	// Output writer should contain the agent message text
	if !strings.Contains(output.String(), "I've fixed the bug") {
		t.Errorf("output = %q, want it to contain agent message", output.String())
	}
}

// TestCodexEventStructParsesAllFields verifies that codexEvent struct correctly
// parses all expected fields from Codex JSONL events including nested item,
// usage, status, and error info.
// Expected failure: codexEvent struct does not exist yet
func TestCodexEventStructParsesAllFields(t *testing.T) {
	tests := []struct {
		name       string
		jsonInput  string
		checkEvent func(t *testing.T, event codexEvent)
	}{
		{
			name:      "thread.started with data",
			jsonInput: `{"type":"thread.started","data":{"thread_id":"t-1"}}`,
			checkEvent: func(t *testing.T, event codexEvent) {
				if event.Type != "thread.started" {
					t.Errorf("Type = %q, want %q", event.Type, "thread.started")
				}
			},
		},
		{
			name:      "item.started with command_execution item",
			jsonInput: `{"type":"item.started","item":{"type":"command_execution","command":"make build"}}`,
			checkEvent: func(t *testing.T, event codexEvent) {
				if event.Type != "item.started" {
					t.Errorf("Type = %q, want %q", event.Type, "item.started")
				}
				if event.Item == nil {
					t.Fatal("Item is nil")
				}
				if event.Item.Type != "command_execution" {
					t.Errorf("Item.Type = %q, want %q", event.Item.Type, "command_execution")
				}
				if event.Item.Command != "make build" {
					t.Errorf("Item.Command = %q, want %q", event.Item.Command, "make build")
				}
			},
		},
		{
			name:      "item.completed with agent_message",
			jsonInput: `{"type":"item.completed","item":{"type":"agent_message","text":"The fix is applied."}}`,
			checkEvent: func(t *testing.T, event codexEvent) {
				if event.Item == nil {
					t.Fatal("Item is nil")
				}
				if event.Item.Text != "The fix is applied." {
					t.Errorf("Item.Text = %q, want %q", event.Item.Text, "The fix is applied.")
				}
			},
		},
		{
			name:      "item.started with file_change",
			jsonInput: `{"type":"item.started","item":{"type":"file_change","path":"/src/app.go"}}`,
			checkEvent: func(t *testing.T, event codexEvent) {
				if event.Item == nil {
					t.Fatal("Item is nil")
				}
				if event.Item.Path != "/src/app.go" {
					t.Errorf("Item.Path = %q, want %q", event.Item.Path, "/src/app.go")
				}
			},
		},
		{
			name:      "item.started with mcp_tool_call",
			jsonInput: `{"type":"item.started","item":{"type":"mcp_tool_call","tool_name":"read_file"}}`,
			checkEvent: func(t *testing.T, event codexEvent) {
				if event.Item == nil {
					t.Fatal("Item is nil")
				}
				if event.Item.ToolName != "read_file" {
					t.Errorf("Item.ToolName = %q, want %q", event.Item.ToolName, "read_file")
				}
			},
		},
		{
			name:      "turn.completed with usage and status",
			jsonInput: `{"type":"turn.completed","status":"completed","usage":{"input_tokens":500,"output_tokens":200,"cached_input_tokens":100}}`,
			checkEvent: func(t *testing.T, event codexEvent) {
				if event.Status != "completed" {
					t.Errorf("Status = %q, want %q", event.Status, "completed")
				}
				if event.Usage == nil {
					t.Fatal("Usage is nil")
				}
				if event.Usage.InputTokens != 500 {
					t.Errorf("Usage.InputTokens = %d, want 500", event.Usage.InputTokens)
				}
			},
		},
		{
			name:      "turn.completed with failed status and error",
			jsonInput: `{"type":"turn.completed","status":"failed","error":{"type":"UsageLimitExceeded","message":"You have exceeded your usage limit"}}`,
			checkEvent: func(t *testing.T, event codexEvent) {
				if event.Status != "failed" {
					t.Errorf("Status = %q, want %q", event.Status, "failed")
				}
				if event.ErrorInfo == nil {
					t.Fatal("ErrorInfo is nil")
				}
				if event.ErrorInfo.Type != "UsageLimitExceeded" {
					t.Errorf("ErrorInfo.Type = %q, want %q", event.ErrorInfo.Type, "UsageLimitExceeded")
				}
				if event.ErrorInfo.Message != "You have exceeded your usage limit" {
					t.Errorf("ErrorInfo.Message = %q, want full message", event.ErrorInfo.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event codexEvent
			if err := json.Unmarshal([]byte(tt.jsonInput), &event); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			tt.checkEvent(t, event)
		})
	}
}

// TestCodexErrorInfoTypes verifies that codexErrorInfo correctly parses different
// error types from Codex events.
// Expected failure: codexErrorInfo struct does not exist yet
func TestCodexErrorInfoTypes(t *testing.T) {
	tests := []struct {
		name        string
		jsonInput   string
		wantType    string
		wantMessage string
	}{
		{
			name:        "UsageLimitExceeded",
			jsonInput:   `{"type":"UsageLimitExceeded","message":"Rate limit exceeded"}`,
			wantType:    "UsageLimitExceeded",
			wantMessage: "Rate limit exceeded",
		},
		{
			name:        "HttpConnectionFailed",
			jsonInput:   `{"type":"HttpConnectionFailed","message":"Connection refused"}`,
			wantType:    "HttpConnectionFailed",
			wantMessage: "Connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errInfo codexErrorInfo
			if err := json.Unmarshal([]byte(tt.jsonInput), &errInfo); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if errInfo.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", errInfo.Type, tt.wantType)
			}
			if errInfo.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", errInfo.Message, tt.wantMessage)
			}
		})
	}
}
