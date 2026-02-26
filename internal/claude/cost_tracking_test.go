package claude

import (
	"strings"
	"testing"
)

// TestProcessStreamJSONExtractsCost verifies that processStreamJSON parses
// cost and token data from result events. Previously the Claude CLI's
// processStreamJSON only captured the result text, not cost data.
func TestProcessStreamJSONExtractsCost(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Done"}]}}`,
		`{"type":"result","result":"Done","total_cost_usd":0.15,"input_tokens":3000,"output_tokens":1200}`,
	}, "\n") + "\n"

	c := &Client{timeout: 60}
	reader := strings.NewReader(input)
	var output strings.Builder

	handler := func(line []byte) {}

	resultText, costUSD, inputTokens, outputTokens, _ := c.processStreamJSONWithCost(reader, &output, handler, nil)

	if resultText != "Done" {
		t.Errorf("resultText = %q, want %q", resultText, "Done")
	}
	if costUSD != 0.15 {
		t.Errorf("costUSD = %f, want 0.15", costUSD)
	}
	if inputTokens != 3000 {
		t.Errorf("inputTokens = %d, want 3000", inputTokens)
	}
	if outputTokens != 1200 {
		t.Errorf("outputTokens = %d, want 1200", outputTokens)
	}
}

// TestProcessStreamJSONExtractsCostWithNilHandler verifies that cost is
// extracted even when handler is nil (preserve_provider_output mode).
func TestProcessStreamJSONExtractsCostWithNilHandler(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`,
		`{"type":"result","result":"Hello","total_cost_usd":0.08,"input_tokens":1500,"output_tokens":600}`,
	}, "\n") + "\n"

	c := &Client{timeout: 60}
	reader := strings.NewReader(input)
	var output strings.Builder

	resultText, costUSD, inputTokens, outputTokens, _ := c.processStreamJSONWithCost(reader, &output, nil, nil)

	if resultText != "Hello" {
		t.Errorf("resultText = %q, want %q", resultText, "Hello")
	}
	if costUSD != 0.08 {
		t.Errorf("costUSD = %f, want 0.08", costUSD)
	}
	if inputTokens != 1500 {
		t.Errorf("inputTokens = %d, want 1500", inputTokens)
	}
	if outputTokens != 600 {
		t.Errorf("outputTokens = %d, want 600", outputTokens)
	}
}

// TestProcessStreamJSONExtractsCostFromNestedUsage verifies that cost is
// extracted from a nested "usage" object in result events.
func TestProcessStreamJSONExtractsCostFromNestedUsage(t *testing.T) {
	input := `{"type":"result","result":"OK","usage":{"total_cost_usd":0.25,"input_tokens":4000,"output_tokens":2000}}` + "\n"

	c := &Client{timeout: 60}
	reader := strings.NewReader(input)
	var output strings.Builder

	_, costUSD, inputTokens, outputTokens, _ := c.processStreamJSONWithCost(reader, &output, nil, nil)

	if costUSD != 0.25 {
		t.Errorf("costUSD = %f, want 0.25", costUSD)
	}
	if inputTokens != 4000 {
		t.Errorf("inputTokens = %d, want 4000", inputTokens)
	}
	if outputTokens != 2000 {
		t.Errorf("outputTokens = %d, want 2000", outputTokens)
	}
}

// TestProcessStreamJSONFallsBackToAssistantText verifies that when the result
// event omits the "result" field, the function still returns meaningful text
// from streamed assistant content.
func TestProcessStreamJSONFallsBackToAssistantText(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Part 1 "}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Part 2"}]}}`,
		`{"type":"result","total_cost_usd":0.05,"input_tokens":1200,"output_tokens":400}`,
	}, "\n") + "\n"

	c := &Client{timeout: 60}
	reader := strings.NewReader(input)
	var output strings.Builder

	resultText, costUSD, inputTokens, outputTokens, _ := c.processStreamJSONWithCost(reader, &output, nil, nil)

	if resultText != "Part 1 Part 2" {
		t.Errorf("resultText = %q, want %q", resultText, "Part 1 Part 2")
	}
	if costUSD != 0.05 {
		t.Errorf("costUSD = %f, want 0.05", costUSD)
	}
	if inputTokens != 1200 {
		t.Errorf("inputTokens = %d, want 1200", inputTokens)
	}
	if outputTokens != 400 {
		t.Errorf("outputTokens = %d, want 400", outputTokens)
	}
}

// TestResultHasCostFields verifies that the Result struct has cost/token fields.
func TestResultHasCostFields(t *testing.T) {
	r := Result{
		CostUSD:      1.5,
		InputTokens:  10000,
		OutputTokens: 5000,
	}

	if r.CostUSD != 1.5 {
		t.Errorf("CostUSD = %f, want 1.5", r.CostUSD)
	}
	if r.InputTokens != 10000 {
		t.Errorf("InputTokens = %d, want 10000", r.InputTokens)
	}
	if r.OutputTokens != 5000 {
		t.Errorf("OutputTokens = %d, want 5000", r.OutputTokens)
	}
}

// TestResultHasCachedInputTokensField verifies that Result struct has CachedInputTokens field.
func TestResultHasCachedInputTokensField(t *testing.T) {
	r := Result{
		CachedInputTokens: 2500,
	}

	if r.CachedInputTokens != 2500 {
		t.Errorf("CachedInputTokens = %d, want 2500", r.CachedInputTokens)
	}
}

// TestProcessStreamJSONParsesCacheReadInputTokens verifies that cache_read_input_tokens
// is extracted from stream JSON result events.
func TestProcessStreamJSONParsesCacheReadInputTokens(t *testing.T) {
	input := `{"type":"result","result":"OK","cache_read_input_tokens":2500}` + "\n"

	c := &Client{timeout: 60}
	reader := strings.NewReader(input)
	var output strings.Builder

	resultText, _, _, _, cachedInputTokens := c.processStreamJSONWithCost(reader, &output, nil, nil)

	if resultText != "OK" {
		t.Errorf("resultText = %q, want %q", resultText, "OK")
	}
	if cachedInputTokens != 2500 {
		t.Errorf("cachedInputTokens = %d, want 2500", cachedInputTokens)
	}
}

// TestProcessStreamJSONParsesCacheReadInputTokensWithOtherTokens verifies that
// cache_read_input_tokens is correctly extracted alongside other token counts.
func TestProcessStreamJSONParsesCacheReadInputTokensWithOtherTokens(t *testing.T) {
	input := `{"type":"result","result":"Done","total_cost_usd":0.15,"input_tokens":3000,"output_tokens":1200,"cache_read_input_tokens":1800}` + "\n"

	c := &Client{timeout: 60}
	reader := strings.NewReader(input)
	var output strings.Builder

	resultText, costUSD, inputTokens, outputTokens, cachedInputTokens := c.processStreamJSONWithCost(reader, &output, nil, nil)

	if resultText != "Done" {
		t.Errorf("resultText = %q, want %q", resultText, "Done")
	}
	if costUSD != 0.15 {
		t.Errorf("costUSD = %f, want 0.15", costUSD)
	}
	if inputTokens != 3000 {
		t.Errorf("inputTokens = %d, want 3000", inputTokens)
	}
	if outputTokens != 1200 {
		t.Errorf("outputTokens = %d, want 1200", outputTokens)
	}
	if cachedInputTokens != 1800 {
		t.Errorf("cachedInputTokens = %d, want 1800", cachedInputTokens)
	}
}
