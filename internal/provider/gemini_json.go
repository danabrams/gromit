package provider

import (
	"encoding/json"
	"fmt"
	"strings"
)

// geminiCostEstimator exposes the subset of config.ProviderDef needed to compute costs.
type geminiCostEstimator interface {
	EstimateCostForModel(model string, inputTokens, outputTokens int) float64
}

// parseGeminiJSONResult parses a Gemini --output-format json response and builds a Result.
// It pulls the assistant text from the top-level response field and reads token data
// from stats.models.<model>.tokens before computing CostUSD via the supplied estimator.
func parseGeminiJSONResult(data []byte, requestedModel string, estimator geminiCostEstimator) (*Result, error) {
	var payload geminiJSONResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshaling Gemini JSON response: %w", err)
	}

	result := &Result{Success: true}

	if payload.Response != "" {
		result.Output = payload.Response
	}

	preferredModel := strings.TrimSpace(payload.Model)
	if preferredModel == "" {
		preferredModel = strings.TrimSpace(requestedModel)
	}

	selectedModel, tokens := selectGeminiModelStats(payload.Stats, preferredModel)
	if selectedModel == "" {
		selectedModel = preferredModel
	}

	result.Model = selectedModel

	if tokens != nil {
		result.InputTokens = tokens.Input
		result.OutputTokens = tokens.outputTokens()
		result.CachedInputTokens = tokens.Cached
	} else if payload.Usage != nil {
		result.InputTokens = payload.Usage.InputTokens
		result.OutputTokens = payload.Usage.OutputTokens
		result.CachedInputTokens = payload.Usage.CachedInputTokens
	}

	if estimator != nil {
		result.CostUSD = estimator.EstimateCostForModel(result.Model, result.InputTokens, result.OutputTokens)
	}

	return result, nil
}

func selectGeminiModelStats(stats *geminiJSONStats, preferredModel string) (string, *geminiJSONTokenStats) {
	if stats == nil || len(stats.Models) == 0 {
		return "", nil
	}

	if preferredModel != "" {
		if entry, ok := stats.Models[preferredModel]; ok && entry != nil && entry.Tokens != nil {
			return preferredModel, entry.Tokens
		}
	}

	for modelName, entry := range stats.Models {
		if entry != nil && entry.Tokens != nil {
			return modelName, entry.Tokens
		}
	}

	return "", nil
}

type geminiJSONResponse struct {
	Response string           `json:"response"`
	Model    string           `json:"model"`
	Stats    *geminiJSONStats `json:"stats"`
	Usage    *geminiJSONUsage `json:"usage"`
}

type geminiJSONStats struct {
	Models map[string]*geminiJSONModelStats `json:"models"`
}

type geminiJSONModelStats struct {
	Tokens *geminiJSONTokenStats `json:"tokens"`
}

type geminiJSONTokenStats struct {
	Input      int `json:"input"`
	Prompt     int `json:"prompt"`
	Candidates int `json:"candidates"`
	Total      int `json:"total"`
	Cached     int `json:"cached"`
	Thoughts   int `json:"thoughts"`
	Tool       int `json:"tool"`
}

type geminiJSONUsage struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
}

func (t *geminiJSONTokenStats) outputTokens() int {
	if t == nil {
		return 0
	}
	diff := t.Total - t.Input
	if diff < 0 {
		return 0
	}
	return diff
}
