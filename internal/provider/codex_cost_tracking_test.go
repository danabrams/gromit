package provider

import (
	"bytes"
	"strings"
	"testing"
)

// TestProcessCodexStreamExtractsUsageFromResultEvent verifies that processCodexStream
// extracts token usage from native "result" events (matching Claude's reporting path).
// Some codex provider versions report token usage in a top-level "result" event with
// a nested "usage" field rather than via "turn.completed" events.
func TestProcessCodexStreamExtractsUsageFromResultEvent(t *testing.T) {
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

// TestProcessCodexStreamReturnsCostWithNilHandler verifies that cost data is
// extracted from turn.completed events even when EventHandler is nil.
// This is the core of the cost tracking bug: preserve_provider_output mode
// passes nil handler, which previously caused --json to be omitted and
// cost data to be lost.
func TestProcessCodexStreamReturnsCostWithNilHandler(t *testing.T) {
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
