package provider

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseGeminiJSONResult parses a Gemini JSON response and returns a Result struct.
// The JSON response contains: output, usage (with input_tokens, output_tokens, cached_input_tokens),
// cost, model, finish_reason, and response fields.
func parseGeminiJSONResult(data []byte) (*Result, error) {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("unmarshaling Gemini JSON response: %w", err)
	}

	result := &Result{
		Success: true,
	}

	// Extract output field
	if output, ok := jsonData["output"].(string); ok {
		result.Output = output
	}

	// Extract model field
	if model, ok := jsonData["model"].(string); ok {
		result.Model = model
	}

	// Extract usage information
	if usage, ok := jsonData["usage"].(map[string]interface{}); ok {
		if inputTokens, ok := usage["input_tokens"].(float64); ok {
			result.InputTokens = int(inputTokens)
		}
		if outputTokens, ok := usage["output_tokens"].(float64); ok {
			result.OutputTokens = int(outputTokens)
		}
		if cachedInputTokens, ok := usage["cached_input_tokens"].(float64); ok {
			result.CachedInputTokens = int(cachedInputTokens)
		}
	}

	return result, nil
}

// parseGeminiStreamEvent parses a single line from Gemini stream-json output.
// Returns a map[string]interface{} containing the parsed event data.
func parseGeminiStreamEvent(line []byte) (map[string]interface{}, error) {
	var event map[string]interface{}
	if err := json.Unmarshal(line, &event); err != nil {
		return nil, fmt.Errorf("unmarshaling Gemini stream event: %w", err)
	}
	return event, nil
}

// extractGeminiAssistantText concatenates all assistant messages from the event stream.
// Collects content from all events with type="message" and role="assistant".
func extractGeminiAssistantText(events []map[string]interface{}) string {
	var sb strings.Builder
	for _, event := range events {
		eventType, ok := event["type"].(string)
		if !ok || eventType != "message" {
			continue
		}
		role, ok := event["role"].(string)
		if !ok || role != "assistant" {
			continue
		}
		content, ok := event["content"].(string)
		if !ok {
			continue
		}
		sb.WriteString(content)
	}
	return sb.String()
}

// extractGeminiTokens extracts token counts from a Gemini response or stream event.
// Handles both JSON response format (usage.input_tokens, usage.output_tokens, usage.cached_input_tokens)
// and stream event format (stats.input_tokens, stats.output_tokens, stats.cached).
// Returns (inputTokens, outputTokens, cachedInputTokens).
func extractGeminiTokens(data map[string]interface{}) (int, int, int) {
	var inputTokens, outputTokens, cachedTokens int

	// Try to extract from usage field (JSON response format)
	if usage, ok := data["usage"].(map[string]interface{}); ok {
		if val, ok := usage["input_tokens"].(float64); ok {
			inputTokens = int(val)
		}
		if val, ok := usage["output_tokens"].(float64); ok {
			outputTokens = int(val)
		}
		if val, ok := usage["cached_input_tokens"].(float64); ok {
			cachedTokens = int(val)
		}
		return inputTokens, outputTokens, cachedTokens
	}

	// Try to extract from stats field (stream event format)
	if stats, ok := data["stats"].(map[string]interface{}); ok {
		if val, ok := stats["input_tokens"].(float64); ok {
			inputTokens = int(val)
		}
		if val, ok := stats["output_tokens"].(float64); ok {
			outputTokens = int(val)
		}
		if val, ok := stats["cached"].(float64); ok {
			cachedTokens = int(val)
		}
	}

	return inputTokens, outputTokens, cachedTokens
}

// extractGeminiCost extracts the cost (in USD) from a Gemini JSON response.
// Looks for the cost.total field in the response.
func extractGeminiCost(data map[string]interface{}) float64 {
	if cost, ok := data["cost"].(map[string]interface{}); ok {
		if total, ok := cost["total"].(float64); ok {
			return total
		}
	}
	return 0.0
}
