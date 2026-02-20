package logger

import (
	"math"
	"strings"
)

const (
	legacyCodex5_3InputPer1k  = 0.00875
	legacyCodex5_3OutputPer1k = 0.07000

	currentCodex5_3InputPer1k       = 0.00175
	currentCodex5_3OutputPer1k      = 0.01400
	currentCodex5_3SparkInputPer1k  = 0.00047
	currentCodex5_3SparkOutputPer1k = 0.00374
)

func normalizeHistoricalIterationCost(entry IterationLog) IterationLog {
	if !isLegacyCodex5_3Model(entry.Model) {
		return entry
	}
	if entry.Provider != "" && entry.Provider != "codex" && entry.Provider != "openai" {
		return entry
	}
	if entry.CostUSD <= 0 {
		return entry
	}
	if entry.InputTokens == 0 && entry.OutputTokens == 0 {
		return entry
	}

	correctedIn, correctedOut, ok := codex5_3CurrentRates(entry.Model)
	if !ok {
		return entry
	}
	legacyEstimate := estimatedCost(entry.InputTokens, entry.OutputTokens, legacyCodex5_3InputPer1k, legacyCodex5_3OutputPer1k)
	correctedEstimate := estimatedCost(entry.InputTokens, entry.OutputTokens, correctedIn, correctedOut)
	if correctedEstimate <= 0 {
		return entry
	}

	// Only rewrite entries that clearly match the old inflated fallback estimate.
	if !nearlyEqual(entry.CostUSD, legacyEstimate) {
		return entry
	}
	if entry.CostUSD <= correctedEstimate*2 {
		return entry
	}

	entry.CostUSD = correctedEstimate
	return entry
}

func isLegacyCodex5_3Model(model string) bool {
	return strings.HasPrefix(model, "gpt-5.3-codex")
}

func codex5_3CurrentRates(model string) (float64, float64, bool) {
	if strings.Contains(model, "spark") {
		return currentCodex5_3SparkInputPer1k, currentCodex5_3SparkOutputPer1k, true
	}
	if model == "gpt-5.3-codex" {
		return currentCodex5_3InputPer1k, currentCodex5_3OutputPer1k, true
	}
	return 0, 0, false
}

func estimatedCost(inputTokens, outputTokens int, inputPer1k, outputPer1k float64) float64 {
	return float64(inputTokens)/1000.0*inputPer1k + float64(outputTokens)/1000.0*outputPer1k
}

func nearlyEqual(a, b float64) bool {
	diff := math.Abs(a - b)
	scale := math.Max(1.0, math.Max(math.Abs(a), math.Abs(b)))
	return diff <= scale*1e-9
}
