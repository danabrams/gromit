package escalation

import (
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

const (
	// defaultTokensPerFile is an estimate of tokens per file for cost calculation.
	// Used when estimating total cost impact of a bead with multiple files.
	defaultTokensPerFile = 2000
)

// EstimateBreadScopeAndCost estimates the total cost of invoking a bead based on
// the number of files it will touch and the model's pricing.
// Returns 0 if the bead is nil or provider is not configured.
func EstimateBreadScopeAndCost(cfg *config.Config, b *bead.Bead, model, providerName string) float64 {
	if b == nil || cfg == nil {
		return 0
	}

	provider, ok := cfg.Providers[providerName]
	if !ok {
		return 0
	}

	fileCount := bead.EstimatedFileCount(b)
	if fileCount == 0 {
		return 0
	}

	// Estimate tokens based on file count
	inputTokens := fileCount * defaultTokensPerFile
	outputTokens := fileCount * defaultTokensPerFile / 2 // output is typically smaller

	return provider.EstimateCostForModel(model, inputTokens, outputTokens)
}

// CheckAndLogCostCeiling checks if estimated cost exceeds the ceiling and logs a warning if it does.
// Returns true if cost exceeds ceiling, false otherwise.
// Does nothing if logFn is nil.
func CheckAndLogCostCeiling(ceiling, estimatedCost float64, logFn func(format string, args ...interface{})) bool {
	if estimatedCost > ceiling {
		if logFn != nil {
			logFn("Warning: estimated cost $%.2f exceeds per-iteration ceiling of $%.2f", estimatedCost, ceiling)
		}
		return true
	}
	return false
}
