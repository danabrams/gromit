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
