package provider

import (
	"encoding/json"
	"testing"

	"github.com/danabrams/gromit/internal/claude"
)

// TestConvertResultPropagatesCostFields verifies that convertResult maps
// CostUSD, InputTokens, and OutputTokens from claude.Result to provider.Result.
// Previously these fields were silently dropped, causing 0 cost for all Claude models.
func TestConvertResultPropagatesCostFields(t *testing.T) {
	claudeResult := &claude.Result{
		Success:      true,
		Output:       "test output",
		ExitCode:     0,
		Model:        "sonnet",
		CostUSD:      1.23,
		InputTokens:  5000,
		OutputTokens: 2000,
	}

	result := convertResult(claudeResult)

	if result.CostUSD != 1.23 {
		t.Errorf("CostUSD = %f, want 1.23", result.CostUSD)
	}
	if result.InputTokens != 5000 {
		t.Errorf("InputTokens = %d, want 5000", result.InputTokens)
	}
	if result.OutputTokens != 2000 {
		t.Errorf("OutputTokens = %d, want 2000", result.OutputTokens)
	}
}

// TestConvertResultPreservesExistingFields verifies that cost field propagation
// doesn't break existing field mapping.
func TestConvertResultPreservesExistingFields(t *testing.T) {
	claudeResult := &claude.Result{
		Success:      true,
		Output:       "hello",
		ExitCode:     0,
		Model:        "opus",
		CostUSD:      0.50,
		InputTokens:  100,
		OutputTokens: 50,
	}

	result := convertResult(claudeResult)

	if result.Success != true {
		t.Error("Success should be true")
	}
	if result.Output != "hello" {
		t.Errorf("Output = %q, want %q", result.Output, "hello")
	}
	if result.Model != "opus" {
		t.Errorf("Model = %q, want %q", result.Model, "opus")
	}
}

// TestConvertResultPropagatesCachedInputTokens verifies that convertResult maps
// CachedInputTokens from claude.Result to provider.Result.
// This is used to track cache hit efficiency on prompt caching invocations.
func TestConvertResultPropagatesCachedInputTokens(t *testing.T) {
	claudeResult := &claude.Result{
		Success:           true,
		Output:            "cached response",
		ExitCode:          0,
		Model:             "sonnet",
		CostUSD:           0.75,
		InputTokens:       3000,
		OutputTokens:      1500,
		CachedInputTokens: 1500,
	}

	result := convertResult(claudeResult)

	if result.CachedInputTokens != 1500 {
		t.Errorf("CachedInputTokens = %d, want 1500", result.CachedInputTokens)
	}
}

func TestStreamJSONCachedTokensPropagateToProvider(t *testing.T) {
	payload := `{"type":"result","result":"cached output","model":"sonnet","total_cost_usd":0.35,"input_tokens":800,"output_tokens":400,"cache_read_input_tokens":200}` + "\n"

	claudeResult, err := parseClaudeStreamResultFromJSON(payload)
	if err != nil {
		t.Fatalf("parse stream JSON: %v", err)
	}

	result := convertResult(claudeResult)
	if result.CachedInputTokens == 0 {
		t.Fatalf("CachedInputTokens = %d, want non-zero", result.CachedInputTokens)
	}
}

func parseClaudeStreamResultFromJSON(payload string) (*claude.Result, error) {
	var event struct {
		Type                 string  `json:"type"`
		Result               string  `json:"result"`
		TotalCostUSD         float64 `json:"total_cost_usd"`
		InputTokens          int     `json:"input_tokens"`
		OutputTokens         int     `json:"output_tokens"`
		CacheReadInputTokens int     `json:"cache_read_input_tokens"`
		Model                string  `json:"model"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return nil, err
	}

	return &claude.Result{
		Success:           true,
		Output:            event.Result,
		Model:             event.Model,
		CostUSD:           event.TotalCostUSD,
		InputTokens:       event.InputTokens,
		OutputTokens:      event.OutputTokens,
		CachedInputTokens: event.CacheReadInputTokens,
	}, nil
}
