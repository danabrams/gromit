package provider

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestCodexStreamMergeSemanticMatrixContractAllEventTypes documents the contract that
// all Codex stream event types (turn.completed, response.completed, result) use the
// same canonical merge strategy: extracting usage via type-specific handlers and
// merging via mergeCodexEventUsage.
func TestCodexStreamMergeSemanticMatrixContractAllEventTypes(t *testing.T) {
	eventTypes := []struct {
		name     string
		input    string
		expected struct {
			input  int
			output int
			cost   float64
		}
	}{
		{
			name: "turn.completed with nested usage fields",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"turn.completed","usage":{"input_tokens":1000,"output_tokens":200,"total_cost_usd":0.011}}`,
			}, "\n") + "\n",
			expected: struct {
				input  int
				output int
				cost   float64
			}{1000, 200, 0.011},
		},
		{
			name: "turn.completed with top-level usage fields",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"turn.completed","input_tokens":1100,"output_tokens":210,"total_cost_usd":0.012}`,
			}, "\n") + "\n",
			expected: struct {
				input  int
				output int
				cost   float64
			}{1100, 210, 0.012},
		},
		{
			name: "response.completed with nested usage fields",
			input: strings.Join([]string{
				`{"type":"assistant","message":{"content":[{"type":"text","text":"Done"}]}}`,
				`{"type":"response.completed","response":{"usage":{"input_tokens":1200,"output_tokens":220,"total_cost_usd":0.013}}}`,
			}, "\n") + "\n",
			expected: struct {
				input  int
				output int
				cost   float64
			}{1200, 220, 0.013},
		},
		{
			name: "response.completed with top-level usage fields",
			input: strings.Join([]string{
				`{"type":"assistant","message":{"content":[{"type":"text","text":"Done"}]}}`,
				`{"type":"response.completed","input_tokens":1300,"output_tokens":230,"total_cost_usd":0.014}`,
			}, "\n") + "\n",
			expected: struct {
				input  int
				output int
				cost   float64
			}{1300, 230, 0.014},
		},
		{
			name: "result with nested usage fields",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"result","result":{"usage":{"input_tokens":1400,"output_tokens":240,"total_cost_usd":0.015}}}`,
			}, "\n") + "\n",
			expected: struct {
				input  int
				output int
				cost   float64
			}{1400, 240, 0.015},
		},
		{
			name: "result with top-level usage fields",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"result","input_tokens":1500,"output_tokens":250,"total_cost_usd":0.016}`,
			}, "\n") + "\n",
			expected: struct {
				input  int
				output int
				cost   float64
			}{1500, 250, 0.016},
		},
	}

	for _, tc := range eventTypes {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.input)
			var output bytes.Buffer

			_, usage, _, err := processCodexStream(reader, &output, nil, nil)
			if err != nil {
				t.Fatalf("processCodexStream() error = %v", err)
			}

			if usage == nil {
				t.Fatal("usage is nil, want non-nil for valid usage data")
			}
			if usage.InputTokens != tc.expected.input {
				t.Errorf("usage.InputTokens = %d, want %d", usage.InputTokens, tc.expected.input)
			}
			if usage.OutputTokens != tc.expected.output {
				t.Errorf("usage.OutputTokens = %d, want %d", usage.OutputTokens, tc.expected.output)
			}
			if usage.TotalCostUSD != tc.expected.cost {
				t.Errorf("usage.TotalCostUSD = %f, want %f", usage.TotalCostUSD, tc.expected.cost)
			}
		})
	}
}

// TestCodexStreamMergeSemanticMatrixContractMergesBehavior documents the contract that
// when multiple events emit usage data, each event's usage replaces (overwrites) values
// in the merged state when they are non-zero, across all event types.
func TestCodexStreamMergeSemanticMatrixContractMergesBehavior(t *testing.T) {
	scenarios := []struct {
		name             string
		input            string
		expectedInput    int
		expectedOutput   int
		expectedCost     float64
		description      string
	}{
		{
			name: "turn.completed replaces with latest usage",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"First"}}`,
				`{"type":"turn.completed","input_tokens":1000,"output_tokens":100,"total_cost_usd":0.010}`,
				`{"type":"item.completed","item":{"type":"agent_message","text":"Second"}}`,
				`{"type":"turn.completed","input_tokens":500,"output_tokens":50,"total_cost_usd":0.005}`,
			}, "\n") + "\n",
			expectedInput:  500,
			expectedOutput: 50,
			expectedCost:   0.005,
			description:    "Latest turn.completed event replaces previous tokens",
		},
		{
			name: "result and turn.completed merge with replacement semantics",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"First"}}`,
				`{"type":"turn.completed","input_tokens":800,"output_tokens":80,"total_cost_usd":0.008}`,
				`{"type":"item.completed","item":{"type":"agent_message","text":"Second"}}`,
				`{"type":"result","result":{"usage":{"input_tokens":200,"output_tokens":20,"total_cost_usd":0.002}}}`,
			}, "\n") + "\n",
			expectedInput:  200,
			expectedOutput: 20,
			expectedCost:   0.002,
			description:    "Result event replaces turn.completed usage values",
		},
		{
			name: "all three event types merge with replacement of newer values",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"1"}}`,
				`{"type":"turn.completed","input_tokens":500,"output_tokens":50,"total_cost_usd":0.005}`,
				`{"type":"item.completed","item":{"type":"agent_message","text":"2"}}`,
				`{"type":"response.completed","response":{"usage":{"input_tokens":300,"output_tokens":30,"total_cost_usd":0.003}}}`,
				`{"type":"item.completed","item":{"type":"agent_message","text":"3"}}`,
				`{"type":"result","input_tokens":200,"output_tokens":20,"total_cost_usd":0.002}`,
			}, "\n") + "\n",
			expectedInput:  200,
			expectedOutput: 20,
			expectedCost:   0.002,
			description:    "Final result event replaces all prior usage values",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			reader := strings.NewReader(sc.input)
			var output bytes.Buffer

			_, usage, _, err := processCodexStream(reader, &output, nil, nil)
			if err != nil {
				t.Fatalf("processCodexStream() error = %v", err)
			}

			if usage == nil {
				t.Fatalf("usage is nil, want non-nil: %s", sc.description)
			}
			if usage.InputTokens != sc.expectedInput {
				t.Errorf("usage.InputTokens = %d, want %d (%s)", usage.InputTokens, sc.expectedInput, sc.description)
			}
			if usage.OutputTokens != sc.expectedOutput {
				t.Errorf("usage.OutputTokens = %d, want %d (%s)", usage.OutputTokens, sc.expectedOutput, sc.description)
			}
			if usage.TotalCostUSD != sc.expectedCost {
				t.Errorf("usage.TotalCostUSD = %f, want %f (%s)", usage.TotalCostUSD, sc.expectedCost, sc.description)
			}
		})
	}
}

// TestCodexStreamMergeSemanticMatrixContractPreservesPartialData documents the contract that
// when events contain partial usage data, existing accumulated usage is preserved
// across all event types.
func TestCodexStreamMergeSemanticMatrixContractPreservesPartialData(t *testing.T) {
	preservationCases := []struct {
		name         string
		input        string
		expectedUsage struct {
			input  int
			output int
		}
		description string
	}{
		{
			name: "preserves output when turn.completed provides only input",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"A"}}`,
				`{"type":"turn.completed","input_tokens":100,"output_tokens":50}`,
				`{"type":"turn.completed","input_tokens":200}`,
			}, "\n") + "\n",
			expectedUsage: struct {
				input  int
				output int
			}{200, 50},
			description: "Partial turn.completed data preserves prior output tokens",
		},
		{
			name: "preserves input when result provides only output",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"A"}}`,
				`{"type":"result","input_tokens":100,"output_tokens":50}`,
				`{"type":"result","output_tokens":60}`,
			}, "\n") + "\n",
			expectedUsage: struct {
				input  int
				output int
			}{100, 60},
			description: "Partial result data preserves prior input tokens",
		},
		{
			name: "merges partial data from different event types",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"A"}}`,
				`{"type":"turn.completed","input_tokens":150}`,
				`{"type":"response.completed","response":{"usage":{"output_tokens":75}}}`,
			}, "\n") + "\n",
			expectedUsage: struct {
				input  int
				output int
			}{150, 75},
			description: "Different event types provide partial data that merges",
		},
	}

	for _, pc := range preservationCases {
		t.Run(pc.name, func(t *testing.T) {
			reader := strings.NewReader(pc.input)
			var output bytes.Buffer

			_, usage, _, err := processCodexStream(reader, &output, nil, nil)
			if err != nil {
				t.Fatalf("processCodexStream() error = %v", err)
			}

			if usage == nil {
				t.Fatalf("usage is nil, want non-nil: %s", pc.description)
			}
			if usage.InputTokens != pc.expectedUsage.input {
				t.Errorf("usage.InputTokens = %d, want %d (%s)",
					usage.InputTokens, pc.expectedUsage.input, pc.description)
			}
			if usage.OutputTokens != pc.expectedUsage.output {
				t.Errorf("usage.OutputTokens = %d, want %d (%s)",
					usage.OutputTokens, pc.expectedUsage.output, pc.description)
			}
		})
	}
}

// TestCodexStreamMergeSemanticMatrixContractEventTypeEmissions documents the contract that
// all event types (turn.completed, response.completed, result) emit stream events when
// usage data is present, following the same emission pattern.
func TestCodexStreamMergeSemanticMatrixContractEventTypeEmissions(t *testing.T) {
	emissionCases := []struct {
		name           string
		input          string
		shouldEmitType string
		description    string
	}{
		{
			name: "turn.completed with usage emits result event",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"turn.completed","input_tokens":100,"output_tokens":50}`,
			}, "\n") + "\n",
			shouldEmitType: "result",
			description:    "turn.completed emits result stream event",
		},
		{
			name: "response.completed with usage emits result event",
			input: strings.Join([]string{
				`{"type":"assistant","message":{"content":[{"type":"text","text":"Done"}]}}`,
				`{"type":"response.completed","response":{"usage":{"input_tokens":100,"output_tokens":50}}}`,
			}, "\n") + "\n",
			shouldEmitType: "result",
			description:    "response.completed emits result stream event",
		},
		{
			name: "result with usage emits result event",
			input: strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"Done"}}`,
				`{"type":"result","input_tokens":100,"output_tokens":50}`,
			}, "\n") + "\n",
			shouldEmitType: "result",
			description:    "result event emits result stream event",
		},
	}

	for _, ec := range emissionCases {
		t.Run(ec.name, func(t *testing.T) {
			reader := strings.NewReader(ec.input)
			var output bytes.Buffer

			emittedEvents := []map[string]interface{}{}
			handler := func(eventJSON []byte) {
				var event map[string]interface{}
				if err := json.Unmarshal(eventJSON, &event); err != nil {
					t.Errorf("failed to unmarshal event JSON: %v", err)
					return
				}
				emittedEvents = append(emittedEvents, event)
			}

			_, _, _, err := processCodexStream(reader, &output, handler, nil)
			if err != nil {
				t.Fatalf("processCodexStream() error = %v", err)
			}

			// Check that at least one result event was emitted
			found := false
			for _, event := range emittedEvents {
				if eventType, ok := event["type"]; ok {
					if eventType == ec.shouldEmitType {
						found = true
						// Verify the result event has usage data
						if _, hasInput := event["input_tokens"]; !hasInput {
							t.Errorf("emitted result event missing input_tokens: %+v", event)
						}
						if _, hasOutput := event["output_tokens"]; !hasOutput {
							t.Errorf("emitted result event missing output_tokens: %+v", event)
						}
						break
					}
				}
			}
			if !found {
				t.Errorf("expected %s event to be emitted: %s", ec.shouldEmitType, ec.description)
			}
		})
	}
}
