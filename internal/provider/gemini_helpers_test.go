package provider

import (
	"os"
	"testing"
)

func TestParseGeminiJSONResult(t *testing.T) {
	jsonBytes, err := os.ReadFile("../../test/fixtures/gemini/json-success.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	result, err := parseGeminiJSONResult(jsonBytes)
	if err != nil {
		t.Fatalf("parseGeminiJSONResult failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Output != "READY" {
		t.Errorf("expected output 'READY', got %q", result.Output)
	}

	if result.InputTokens != 13284 {
		t.Errorf("expected input_tokens 13284, got %d", result.InputTokens)
	}

	if result.OutputTokens != 33 {
		t.Errorf("expected output_tokens 33, got %d", result.OutputTokens)
	}

	if result.CachedInputTokens != 0 {
		t.Errorf("expected cached_input_tokens 0, got %d", result.CachedInputTokens)
	}

	if result.Model != "gemini-3-flash-preview" {
		t.Errorf("expected model 'gemini-3-flash-preview', got %q", result.Model)
	}
}
