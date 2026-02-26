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

func TestParseGeminiStreamEvent(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantType  string
		wantRole  string
		wantModel string
	}{
		{
			name:     "init event",
			line:     `{"type":"init","timestamp":"2026-02-23T23:22:33.421Z","session_id":"0fc744b2-d900-4dce-880f-c78c1d34fc80","model":"auto-gemini-3"}`,
			wantType: "init",
			wantRole: "",
			wantModel: "auto-gemini-3",
		},
		{
			name:     "user message event",
			line:     `{"type":"message","timestamp":"2026-02-23T23:22:33.424Z","role":"user","content":"Return exactly STREAM_OK"}`,
			wantType: "message",
			wantRole: "user",
		},
		{
			name:     "assistant message event",
			line:     `{"type":"message","timestamp":"2026-02-23T23:22:36.416Z","role":"assistant","content":"STREAM_OK","delta":true}`,
			wantType: "message",
			wantRole: "assistant",
		},
		{
			name:     "result event",
			line:     `{"type":"result","timestamp":"2026-02-23T23:22:36.430Z","status":"success","stats":{"total_tokens":13458,"input_tokens":13284,"output_tokens":33,"cached":0,"input":13284,"duration_ms":3009,"tool_calls":0}}`,
			wantType: "result",
			wantRole: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := parseGeminiStreamEvent([]byte(tt.line))
			if err != nil {
				t.Fatalf("parseGeminiStreamEvent failed: %v", err)
			}

			if event == nil {
				t.Fatal("expected non-nil event")
			}

			if eventType, ok := event["type"].(string); !ok || eventType != tt.wantType {
				t.Errorf("expected type %q, got %q", tt.wantType, eventType)
			}

			if tt.wantRole != "" {
				if role, ok := event["role"].(string); !ok || role != tt.wantRole {
					t.Errorf("expected role %q, got %q", tt.wantRole, role)
				}
			}

			if tt.wantModel != "" {
				if model, ok := event["model"].(string); !ok || model != tt.wantModel {
					t.Errorf("expected model %q, got %q", tt.wantModel, model)
				}
			}
		})
	}
}

func TestExtractGeminiAssistantText(t *testing.T) {
	events := []map[string]interface{}{
		{
			"type":    "message",
			"role":    "user",
			"content": "Return exactly STREAM_OK",
		},
		{
			"type":    "message",
			"role":    "assistant",
			"content": "STREAM_OK",
			"delta":   true,
		},
	}

	text := extractGeminiAssistantText(events)
	if text != "STREAM_OK" {
		t.Errorf("expected 'STREAM_OK', got %q", text)
	}
}

func TestExtractGeminiAssistantText_MultipleMessages(t *testing.T) {
	events := []map[string]interface{}{
		{
			"type":    "message",
			"role":    "assistant",
			"content": "Hello",
		},
		{
			"type":    "message",
			"role":    "assistant",
			"content": " ",
		},
		{
			"type":    "message",
			"role":    "assistant",
			"content": "world",
		},
	}

	text := extractGeminiAssistantText(events)
	if text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", text)
	}
}

func TestExtractGeminiAssistantText_EmptyEvents(t *testing.T) {
	text := extractGeminiAssistantText([]map[string]interface{}{})
	if text != "" {
		t.Errorf("expected empty string, got %q", text)
	}
}

func TestExtractGeminiTokens_FromStreamEvent(t *testing.T) {
	event := map[string]interface{}{
		"type":   "result",
		"status": "success",
		"stats": map[string]interface{}{
			"total_tokens":  13458.0,
			"input_tokens":  13284.0,
			"output_tokens": 33.0,
			"cached":        0.0,
		},
	}

	inputTokens, outputTokens, cached := extractGeminiTokens(event)
	if inputTokens != 13284 {
		t.Errorf("expected input_tokens 13284, got %d", inputTokens)
	}
	if outputTokens != 33 {
		t.Errorf("expected output_tokens 33, got %d", outputTokens)
	}
	if cached != 0 {
		t.Errorf("expected cached 0, got %d", cached)
	}
}

func TestExtractGeminiTokens_FromJSONResponse(t *testing.T) {
	jsonData := map[string]interface{}{
		"usage": map[string]interface{}{
			"input_tokens":        13284.0,
			"output_tokens":       33.0,
			"cached_input_tokens": 0.0,
		},
	}

	inputTokens, outputTokens, cached := extractGeminiTokens(jsonData)
	if inputTokens != 13284 {
		t.Errorf("expected input_tokens 13284, got %d", inputTokens)
	}
	if outputTokens != 33 {
		t.Errorf("expected output_tokens 33, got %d", outputTokens)
	}
	if cached != 0 {
		t.Errorf("expected cached 0, got %d", cached)
	}
}

func TestExtractGeminiTokens_EmptyData(t *testing.T) {
	inputTokens, outputTokens, cached := extractGeminiTokens(map[string]interface{}{})
	if inputTokens != 0 || outputTokens != 0 || cached != 0 {
		t.Errorf("expected all zeros, got input=%d, output=%d, cached=%d", inputTokens, outputTokens, cached)
	}
}

func TestExtractGeminiCost_FromJSONResponse(t *testing.T) {
	jsonData := map[string]interface{}{
		"cost": map[string]interface{}{
			"total": 0.0,
		},
	}

	cost := extractGeminiCost(jsonData)
	if cost != 0.0 {
		t.Errorf("expected cost 0.0, got %f", cost)
	}
}

func TestExtractGeminiCost_WithValue(t *testing.T) {
	jsonData := map[string]interface{}{
		"cost": map[string]interface{}{
			"total": 0.123456,
		},
	}

	cost := extractGeminiCost(jsonData)
	if cost != 0.123456 {
		t.Errorf("expected cost 0.123456, got %f", cost)
	}
}

func TestExtractGeminiCost_EmptyData(t *testing.T) {
	cost := extractGeminiCost(map[string]interface{}{})
	if cost != 0.0 {
		t.Errorf("expected cost 0.0, got %f", cost)
	}
}

func TestExtractGeminiCost_MissingCostField(t *testing.T) {
	jsonData := map[string]interface{}{
		"output": "test",
	}

	cost := extractGeminiCost(jsonData)
	if cost != 0.0 {
		t.Errorf("expected cost 0.0, got %f", cost)
	}
}

func TestClassifyGeminiFailure_Success(t *testing.T) {
	category := classifyGeminiFailure(0, "")
	if category != FailureCategoryNone {
		t.Errorf("expected FailureCategoryNone for exit code 0, got %q", category)
	}
}

func TestClassifyGeminiFailure_AuthError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
	}{
		{"invalid api key", "error: invalid api key"},
		{"unauthorized", "unauthorized request"},
		{"forbidden", "forbidden: access denied"},
		{"authentication", "authentication failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := classifyGeminiFailure(1, tt.stderr)
			if category != FailureCategoryAuth {
				t.Errorf("expected FailureCategoryAuth, got %q", category)
			}
		})
	}
}

func TestClassifyGeminiFailure_StartupError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
	}{
		{"failed to start", "failed to start gemini"},
		{"initialization error", "initialization error"},
		{"startup failed", "startup failed: no api key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := classifyGeminiFailure(1, tt.stderr)
			if category != FailureCategoryStartupError {
				t.Errorf("expected FailureCategoryStartupError, got %q", category)
			}
		})
	}
}

func TestClassifyGeminiFailure_TransportError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
	}{
		{"connection reset", "connection reset by peer"},
		{"timeout", "request timeout"},
		{"service unavailable", "service unavailable"},
		{"broken pipe", "broken pipe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := classifyGeminiFailure(1, tt.stderr)
			if category != FailureCategoryTransportDisconnect {
				t.Errorf("expected FailureCategoryTransportDisconnect, got %q", category)
			}
		})
	}
}

func TestClassifyGeminiFailure_RateLimit(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
	}{
		{"rate limit", "rate limit exceeded"},
		{"quota exceeded", "quota exceeded"},
		{"too many requests", "too many requests"},
		{"429 status", "HTTP 429"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := classifyGeminiFailure(1, tt.stderr)
			if category != FailureCategoryRateLimited {
				t.Errorf("expected FailureCategoryRateLimited, got %q", category)
			}
		})
	}
}

func TestClassifyGeminiFailure_Other(t *testing.T) {
	category := classifyGeminiFailure(1, "unknown error")
	if category != FailureCategoryOther {
		t.Errorf("expected FailureCategoryOther, got %q", category)
	}
}
