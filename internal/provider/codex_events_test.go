package provider

import (
	"encoding/json"
	"testing"
)

// TestCodexUsageStruct verifies that codexUsage struct can parse token usage data.
// Red: codexUsage struct does not exist yet
func TestCodexUsageStruct(t *testing.T) {
	jsonData := `{"input_tokens":100,"cached_input_tokens":50,"output_tokens":75}`

	var usage codexUsage
	err := json.Unmarshal([]byte(jsonData), &usage)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", usage.InputTokens)
	}
	if usage.CachedInputTokens != 50 {
		t.Errorf("CachedInputTokens = %d, want 50", usage.CachedInputTokens)
	}
	if usage.OutputTokens != 75 {
		t.Errorf("OutputTokens = %d, want 75", usage.OutputTokens)
	}
}

// TestCodexErrorInfoStruct verifies that codexErrorInfo struct can parse error data.
// Red: codexErrorInfo struct does not exist yet
func TestCodexErrorInfoStruct(t *testing.T) {
	jsonData := `{"type":"UsageLimitExceeded","message":"Rate limit exceeded"}`

	var errInfo codexErrorInfo
	err := json.Unmarshal([]byte(jsonData), &errInfo)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if errInfo.Type != "UsageLimitExceeded" {
		t.Errorf("Type = %q, want %q", errInfo.Type, "UsageLimitExceeded")
	}
	if errInfo.Message != "Rate limit exceeded" {
		t.Errorf("Message = %q, want %q", errInfo.Message, "Rate limit exceeded")
	}
}

// TestCodexItemStruct verifies that codexItem struct can parse different item types.
// Red: codexItem struct does not exist yet
func TestCodexItemStruct(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		checkFn  func(t *testing.T, item codexItem)
	}{
		{
			name:     "agent_message",
			jsonData: `{"type":"agent_message","text":"Hello world"}`,
			checkFn: func(t *testing.T, item codexItem) {
				if item.Type != "agent_message" {
					t.Errorf("Type = %q, want %q", item.Type, "agent_message")
				}
				if item.Text != "Hello world" {
					t.Errorf("Text = %q, want %q", item.Text, "Hello world")
				}
			},
		},
		{
			name:     "command_execution",
			jsonData: `{"type":"command_execution","command":"go test"}`,
			checkFn: func(t *testing.T, item codexItem) {
				if item.Type != "command_execution" {
					t.Errorf("Type = %q, want %q", item.Type, "command_execution")
				}
				if item.Command != "go test" {
					t.Errorf("Command = %q, want %q", item.Command, "go test")
				}
			},
		},
		{
			name:     "file_change",
			jsonData: `{"type":"file_change","path":"/src/main.go"}`,
			checkFn: func(t *testing.T, item codexItem) {
				if item.Type != "file_change" {
					t.Errorf("Type = %q, want %q", item.Type, "file_change")
				}
				if item.Path != "/src/main.go" {
					t.Errorf("Path = %q, want %q", item.Path, "/src/main.go")
				}
			},
		},
		{
			name:     "mcp_tool_call",
			jsonData: `{"type":"mcp_tool_call","tool_name":"github_search"}`,
			checkFn: func(t *testing.T, item codexItem) {
				if item.Type != "mcp_tool_call" {
					t.Errorf("Type = %q, want %q", item.Type, "mcp_tool_call")
				}
				if item.ToolName != "github_search" {
					t.Errorf("ToolName = %q, want %q", item.ToolName, "github_search")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item codexItem
			err := json.Unmarshal([]byte(tt.jsonData), &item)
			if err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			tt.checkFn(t, item)
		})
	}
}
