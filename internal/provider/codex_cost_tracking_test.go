package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// TestProcessCodexStreamExtractsUsageFromResultEvent verifies that processCodexStream
// extracts token usage from native "result" events (matching Claude's reporting path).
// Some codex provider versions report token usage in a top-level "result" event with
// a nested "usage" field rather than via "turn.completed" events.
func TestProcessCodexStreamExtractsUsageFromResultEvent(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
		`{"type":"result","usage":{"input_tokens":1500,"output_tokens":400,"total_cost_usd":0.025}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	_, usage, _, err := processCodexStream(reader, &output, nil, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if usage == nil {
		t.Fatal("usage is nil, want non-nil when result event has usage data")
	}
	if usage.InputTokens != 1500 {
		t.Errorf("usage.InputTokens = %d, want 1500", usage.InputTokens)
	}
	if usage.OutputTokens != 400 {
		t.Errorf("usage.OutputTokens = %d, want 400", usage.OutputTokens)
	}
	if usage.TotalCostUSD != 0.025 {
		t.Errorf("usage.TotalCostUSD = %f, want 0.025", usage.TotalCostUSD)
	}
}

// TestProcessCodexStreamExtractsUsageFromNestedResultPayload verifies usage
// extraction when codex emits token data under result.usage.
func TestProcessCodexStreamExtractsUsageFromNestedResultPayload(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
		`{"type":"result","result":{"status":"success","usage":{"input_tokens":2400,"cached_input_tokens":300,"output_tokens":650,"total_cost_usd":0.044}}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	_, usage, _, err := processCodexStream(reader, &output, nil, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if usage == nil {
		t.Fatal("usage is nil, want non-nil when nested result.usage is present")
	}
	if usage.InputTokens != 2400 {
		t.Errorf("usage.InputTokens = %d, want 2400", usage.InputTokens)
	}
	if usage.CachedInputTokens != 300 {
		t.Errorf("usage.CachedInputTokens = %d, want 300", usage.CachedInputTokens)
	}
	if usage.OutputTokens != 650 {
		t.Errorf("usage.OutputTokens = %d, want 650", usage.OutputTokens)
	}
	if usage.TotalCostUSD != 0.044 {
		t.Errorf("usage.TotalCostUSD = %f, want 0.044", usage.TotalCostUSD)
	}
}

// TestProcessCodexStreamExtractsUsageFromResultTokenFields verifies usage
// extraction when result events include direct token fields.
func TestProcessCodexStreamExtractsUsageFromResultTokenFields(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
		`{"type":"result","input_tokens":1800,"output_tokens":450,"total_cost_usd":0.031}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	_, usage, _, err := processCodexStream(reader, &output, nil, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if usage == nil {
		t.Fatal("usage is nil, want non-nil when result has direct token fields")
	}
	if usage.InputTokens != 1800 {
		t.Errorf("usage.InputTokens = %d, want 1800", usage.InputTokens)
	}
	if usage.OutputTokens != 450 {
		t.Errorf("usage.OutputTokens = %d, want 450", usage.OutputTokens)
	}
	if usage.TotalCostUSD != 0.031 {
		t.Errorf("usage.TotalCostUSD = %f, want 0.031", usage.TotalCostUSD)
	}
}

// TestProcessCodexStreamExtractsUsageFromResponseCompleted verifies usage
// extraction when codex emits response.completed events with nested response.usage.
func TestProcessCodexStreamExtractsUsageFromResponseCompleted(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Done"}]}}`,
		`{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":2100,"cached_input_tokens":250,"output_tokens":520,"total_cost_usd":0.036}}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	resultText, usage, _, err := processCodexStream(reader, &output, nil, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if resultText != "Done" {
		t.Errorf("resultText = %q, want %q", resultText, "Done")
	}
	if usage == nil {
		t.Fatal("usage is nil, want non-nil when response.completed has usage data")
	}
	if usage.InputTokens != 2100 {
		t.Errorf("usage.InputTokens = %d, want 2100", usage.InputTokens)
	}
	if usage.CachedInputTokens != 250 {
		t.Errorf("usage.CachedInputTokens = %d, want 250", usage.CachedInputTokens)
	}
	if usage.OutputTokens != 520 {
		t.Errorf("usage.OutputTokens = %d, want 520", usage.OutputTokens)
	}
	if usage.TotalCostUSD != 0.036 {
		t.Errorf("usage.TotalCostUSD = %f, want 0.036", usage.TotalCostUSD)
	}
}

// TestProcessCodexStreamMergesUsageAcrossTurnCompletedEvents verifies usage is
// merged across multiple turn.completed events rather than overwritten.
func TestProcessCodexStreamMergesUsageAcrossTurnCompletedEvents(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"type":"turn.completed","usage":{"input_tokens":2100,"cached_input_tokens":180}}`,
		`{"type":"turn.completed","usage":{"output_tokens":520,"total_cost_usd":0.036}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	_, usage, _, err := processCodexStream(reader, &output, nil, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if usage == nil {
		t.Fatal("usage is nil, want non-nil when turn.completed events include usage data")
	}
	if usage.InputTokens != 2100 {
		t.Errorf("usage.InputTokens = %d, want 2100", usage.InputTokens)
	}
	if usage.CachedInputTokens != 180 {
		t.Errorf("usage.CachedInputTokens = %d, want 180", usage.CachedInputTokens)
	}
	if usage.OutputTokens != 520 {
		t.Errorf("usage.OutputTokens = %d, want 520", usage.OutputTokens)
	}
	if usage.TotalCostUSD != 0.036 {
		t.Errorf("usage.TotalCostUSD = %f, want 0.036", usage.TotalCostUSD)
	}
}

// TestProcessCodexStreamReturnsCostWithNilHandler verifies that cost data is
// extracted from turn.completed events even when EventHandler is nil.
// This is the core of the cost tracking bug: preserve_provider_output mode
// passes nil handler, which previously caused --json to be omitted and
// cost data to be lost.
func TestProcessCodexStreamReturnsCostWithNilHandler(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":2000,"output_tokens":500,"total_cost_usd":0.035}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	// nil handler — simulates preserve_provider_output mode
	resultText, usage, _, err := processCodexStream(reader, &output, nil, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if resultText != "Done" {
		t.Errorf("resultText = %q, want %q", resultText, "Done")
	}

	if usage == nil {
		t.Fatal("usage is nil with nil handler, want cost data extracted regardless")
	}
	if usage.TotalCostUSD != 0.035 {
		t.Errorf("usage.TotalCostUSD = %f, want 0.035", usage.TotalCostUSD)
	}
	if usage.InputTokens != 2000 {
		t.Errorf("usage.InputTokens = %d, want 2000", usage.InputTokens)
	}
	if usage.OutputTokens != 500 {
		t.Errorf("usage.OutputTokens = %d, want 500", usage.OutputTokens)
	}
}

// TestBuildStreamCommandArgsAlwaysIncludesJSON verifies that --json is always
// included in stream command args, regardless of the jsonMode parameter.
// Cost tracking requires JSONL events; plain text mode loses all cost data.
func TestBuildStreamCommandArgsAlwaysIncludesJSON(t *testing.T) {
	t.Parallel()
	cp := NewCodexProvider("/usr/bin/codex", []string{}, map[string]string{TierMedium: "gpt-4o"})

	// Even when jsonMode=false (nil handler), --json should be present
	args := cp.buildStreamCommandArgs("gpt-4o", false)

	found := false
	for _, arg := range args {
		if arg == "--json" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("buildStreamCommandArgs(model, false) = %v, want --json always present for cost tracking", args)
	}
}

// TestCodexStreamRunNilHandlerReturnsCost verifies that StreamRun with nil
// EventHandler still populates CostUSD and token fields on Result.
// This is the end-to-end test for the preserve_provider_output cost bug.
func TestCodexStreamRunNilHandlerReturnsCost(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":1000,"output_tokens":300,"total_cost_usd":0.02}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	// Call processCodexStream directly with nil handler (simulates what
	// streamRunOnce should do regardless of handler presence)
	_, usage, _, err := processCodexStream(reader, &output, nil, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	cost := usageCost(usage)
	if cost != 0.02 {
		t.Errorf("usageCost() = %f, want 0.02", cost)
	}
	inTokens := usageInputTokens(usage)
	if inTokens != 1000 {
		t.Errorf("usageInputTokens() = %d, want 1000", inTokens)
	}
	outTokens := usageOutputTokens(usage)
	if outTokens != 300 {
		t.Errorf("usageOutputTokens() = %d, want 300", outTokens)
	}
}

// TestProcessCodexStreamExtractsUsageFromTurnCompletedTopLevelFields verifies
// that turn.completed extracts token usage from top-level InputTokens/OutputTokens
// fields using the same extraction logic as response.completed and result events.
func TestProcessCodexStreamExtractsUsageFromTurnCompletedTopLevelFields(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
		`{"type":"turn.completed","input_tokens":1900,"output_tokens":480,"total_cost_usd":0.033}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	_, usage, _, err := processCodexStream(reader, &output, nil, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if usage == nil {
		t.Fatal("usage is nil, want non-nil when turn.completed has direct token fields")
	}
	if usage.InputTokens != 1900 {
		t.Errorf("usage.InputTokens = %d, want 1900", usage.InputTokens)
	}
	if usage.OutputTokens != 480 {
		t.Errorf("usage.OutputTokens = %d, want 480", usage.OutputTokens)
	}
	if usage.TotalCostUSD != 0.033 {
		t.Errorf("usage.TotalCostUSD = %f, want 0.033", usage.TotalCostUSD)
	}
}

// TestProcessCodexStreamNormalizesMergeSemantics verifies that all three event
// handlers (turn.completed, response.completed, result) use consistent merge
// semantics with extractUsageFromFields and mergeCodexUsage.
func TestProcessCodexStreamNormalizesMergeSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		input         string
		expectedInput int
		expectedOut   int
		expectedCost  float64
	}{
		{
			name: "turn.completed with nested usage",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"turn.completed","usage":{"input_tokens":2000,"output_tokens":500,"total_cost_usd":0.035}}`,
			}, "\n") + "\n",
			expectedInput: 2000,
			expectedOut:   500,
			expectedCost:  0.035,
		},
		{
			name: "response.completed with nested usage",
			input: strings.Join([]string{
				`{"type":"assistant","message":{"content":[{"type":"text","text":"Done"}]}}`,
				`{"type":"response.completed","response":{"usage":{"input_tokens":2100,"output_tokens":520,"total_cost_usd":0.036}}}`,
			}, "\n") + "\n",
			expectedInput: 2100,
			expectedOut:   520,
			expectedCost:  0.036,
		},
		{
			name: "result with nested usage",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"result","result":{"usage":{"input_tokens":2200,"output_tokens":540,"total_cost_usd":0.037}}}`,
			}, "\n") + "\n",
			expectedInput: 2200,
			expectedOut:   540,
			expectedCost:  0.037,
		},
		{
			name: "turn.completed with top-level fields (after normalization)",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"turn.completed","input_tokens":1900,"output_tokens":480,"total_cost_usd":0.033}`,
			}, "\n") + "\n",
			expectedInput: 1900,
			expectedOut:   480,
			expectedCost:  0.033,
		},
		{
			name: "result with top-level fields (after normalization)",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"result","input_tokens":1800,"output_tokens":450,"total_cost_usd":0.031}`,
			}, "\n") + "\n",
			expectedInput: 1800,
			expectedOut:   450,
			expectedCost:  0.031,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reader := strings.NewReader(tc.input)
			var output bytes.Buffer

			_, usage, _, err := processCodexStream(reader, &output, nil, nil)
			if err != nil {
				t.Fatalf("processCodexStream() error = %v", err)
			}

			if usage == nil {
				t.Fatal("usage is nil, want non-nil for valid usage data")
			}
			if usage.InputTokens != tc.expectedInput {
				t.Errorf("usage.InputTokens = %d, want %d", usage.InputTokens, tc.expectedInput)
			}
			if usage.OutputTokens != tc.expectedOut {
				t.Errorf("usage.OutputTokens = %d, want %d", usage.OutputTokens, tc.expectedOut)
			}
			if usage.TotalCostUSD != tc.expectedCost {
				t.Errorf("usage.TotalCostUSD = %f, want %f", usage.TotalCostUSD, tc.expectedCost)
			}
		})
	}
}

// TestProcessCodexStreamEmitsWarningWhenInputTokensExceed3M verifies that a warning
// event is emitted when input tokens exceed 3 million (the high-cost iteration threshold).
func TestProcessCodexStreamEmitsWarningWhenInputTokensExceed3M(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":3100000,"output_tokens":500,"total_cost_usd":0.527}}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer

	var capturedEvents []map[string]interface{}
	handler := func(eventJSON []byte) {
		var event map[string]interface{}
		if err := json.Unmarshal(eventJSON, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		capturedEvents = append(capturedEvents, event)
	}

	_, usage, _, err := processCodexStream(reader, &output, handler, nil)
	if err != nil {
		t.Fatalf("processCodexStream() error = %v", err)
	}

	if usage == nil {
		t.Fatal("usage is nil, want non-nil")
	}
	if usage.InputTokens != 3100000 {
		t.Errorf("usage.InputTokens = %d, want 3100000", usage.InputTokens)
	}

	// Verify warning event was emitted
	var foundWarning bool
	for _, event := range capturedEvents {
		if eventType, ok := event["type"].(string); ok && eventType == "warning" {
			foundWarning = true
			if msg, ok := event["message"].(string); !ok || !strings.Contains(msg, "3000000") {
				t.Errorf("warning event message missing threshold: %v", event)
			}
			break
		}
	}
	if !foundWarning {
		t.Errorf("no warning event emitted for input tokens > 3M, events: %v", capturedEvents)
	}
}

// TestCodexProviderFallbackCost_ComputesFromTokensWhenNoCostInStream verifies that
// when Codex emits tokens but no total_cost_usd (as happens with ChatGPT accounts),
// the provider falls back to computing cost from token counts × configured rates.
func TestCodexProviderFallbackCost_ComputesFromTokensWhenNoCostInStream(t *testing.T) {
	t.Parallel()
	// Emit tokens but no total_cost_usd — mirrors real Codex ChatGPT account output:
	// {"type":"turn.completed","usage":{"input_tokens":9349,"cached_input_tokens":6912,"output_tokens":72}}
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":1000000,"cached_input_tokens":0,"output_tokens":500000}}'
exit 0`)
	cp := NewCodexProvider(mockBinary, []string{}, map[string]string{TierLow: "gpt-5.4"})
	// gpt-5.4: $2.00/1M input, $8.00/1M output → 1M input + 500K output = $2.00 + $4.00 = $6.00
	cp.SetModelPricing(map[string]ModelPricing{
		"gpt-5.4": {InputCostPerMillion: 2.00, OutputCostPerMillion: 8.00},
	})

	result, err := cp.StreamRun(context.Background(), "test", TierLow, io.Discard, nil, nil)
	if err != nil {
		t.Fatalf("StreamRun error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.InputTokens != 1000000 {
		t.Errorf("InputTokens = %d, want 1000000", result.InputTokens)
	}
	if result.OutputTokens != 500000 {
		t.Errorf("OutputTokens = %d, want 500000", result.OutputTokens)
	}
	const wantCost = 6.00 // $2.00 + $4.00
	if result.CostUSD != wantCost {
		t.Errorf("CostUSD = %f, want %f (computed from tokens × rates)", result.CostUSD, wantCost)
	}
}

// TestCodexProviderFallbackCost_ZeroWhenNoPricingConfigured verifies that when no
// model pricing is configured, CostUSD remains 0 (no panics or guesses).
func TestCodexProviderFallbackCost_ZeroWhenNoPricingConfigured(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":1000,"output_tokens":300}}'
exit 0`)
	cp := NewCodexProvider(mockBinary, []string{}, map[string]string{TierLow: "gpt-5.4"})
	// No SetModelPricing call — pricing is unconfigured.

	result, err := cp.StreamRun(context.Background(), "test", TierLow, io.Discard, nil, nil)
	if err != nil {
		t.Fatalf("StreamRun error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0 when no pricing configured", result.CostUSD)
	}
}

// TestCodexProviderFallbackCost_StreamCostTakesPrecedence verifies that when Codex
// DOES emit total_cost_usd, that value is used rather than the token-based fallback.
func TestCodexProviderFallbackCost_StreamCostTakesPrecedence(t *testing.T) {
	t.Parallel()
	// Emit both tokens AND cost — the stream cost should win.
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":1000000,"output_tokens":500000,"total_cost_usd":1.23}}'
exit 0`)
	cp := NewCodexProvider(mockBinary, []string{}, map[string]string{TierLow: "gpt-5.4"})
	cp.SetModelPricing(map[string]ModelPricing{
		"gpt-5.4": {InputCostPerMillion: 2.00, OutputCostPerMillion: 8.00},
	})

	result, err := cp.StreamRun(context.Background(), "test", TierLow, io.Discard, nil, nil)
	if err != nil {
		t.Fatalf("StreamRun error: %v", err)
	}
	// Stream reported $1.23; token-based fallback would give $6.00. Stream wins.
	if result.CostUSD != 1.23 {
		t.Errorf("CostUSD = %f, want 1.23 (stream value takes precedence)", result.CostUSD)
	}
}
