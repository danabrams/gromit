package provider

import (
	"math"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestParseGeminiJSONResultTokensAndCost(t *testing.T) {
	t.Parallel()

	sampleJSON := `{
  "response": "READY response",
  "stats": {
    "models": {
      "gemini-2.5-flash": {
        "tokens": {
          "input": 100,
          "prompt": 100,
          "candidates": 5,
          "total": 120,
          "cached": 3,
          "thoughts": 15,
          "tool": 2
        }
      }
    }
  }
}`

	t.Run("with pricing", func(t *testing.T) {
		t.Parallel()
		pricing := config.ProviderDef{
			CostPer1kInput:  0.01,
			CostPer1kOutput: 0.02,
		}
		result, err := parseGeminiJSONResult([]byte(sampleJSON), "gemini-2.5-flash", &pricing)
		if err != nil {
			t.Fatalf("parseGeminiJSONResult failed: %v", err)
		}
		if result.Output != "READY response" {
			t.Fatalf("expected response text, got %q", result.Output)
		}
		if result.InputTokens != 100 {
			t.Fatalf("input tokens = %d, want 100", result.InputTokens)
		}
		if result.OutputTokens != 20 {
			t.Fatalf("output tokens = %d, want 20", result.OutputTokens)
		}
		if result.CachedInputTokens != 3 {
			t.Fatalf("cached input tokens = %d, want 3", result.CachedInputTokens)
		}
		if result.Model != "gemini-2.5-flash" {
			t.Fatalf("model = %q, want gemini-2.5-flash", result.Model)
		}
		wantCost := pricing.EstimateCostForModel("gemini-2.5-flash", 100, 20)
		if math.Abs(result.CostUSD-wantCost) > 1e-12 {
			t.Fatalf("cost USD = %f, want %f", result.CostUSD, wantCost)
		}
	})

	t.Run("without pricing", func(t *testing.T) {
		t.Parallel()
		result, err := parseGeminiJSONResult([]byte(sampleJSON), "gemini-2.5-flash", nil)
		if err != nil {
			t.Fatalf("parseGeminiJSONResult failed: %v", err)
		}
		if result.CostUSD != 0 {
			t.Fatalf("cost USD = %f, want 0", result.CostUSD)
		}
		if result.InputTokens != 100 || result.OutputTokens != 20 {
			t.Fatalf("unexpected tokens: input=%d output=%d", result.InputTokens, result.OutputTokens)
		}
	})
}
