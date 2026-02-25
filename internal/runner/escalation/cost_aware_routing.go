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

const (
	broadScopeFileThreshold = 5
	cheaperCodexModel       = "gpt-5.2-codex"
)

// SelectCostAwareModel selects between models based on scope size.
// When file count exceeds broadScopeFileThreshold, prefers cheaper alternatives (gpt-5.2-codex).
// Otherwise returns the original model.
func SelectCostAwareModel(cfg *config.Config, b *bead.Bead, originalModel, providerName string) string {
	if b == nil || cfg == nil {
		return originalModel
	}

	fileCount := bead.EstimatedFileCount(b)

	// For broad scope (> 5 files), prefer cheaper model if available
	if fileCount > broadScopeFileThreshold {
		provider, ok := cfg.Providers[providerName]
		if ok && provider.ModelCosts != nil {
			if _, hasCheaper := provider.ModelCosts[cheaperCodexModel]; hasCheaper {
				return cheaperCodexModel
			}
		}
	}

	return originalModel
}

// SelectModelWithCostAwareness returns a concrete model name considering cost awareness for broad scopes.
// For codex provider with broad scopes (> 5 files), prefers the cheaper gpt-5.2-codex.
// Returns the concrete model name from the provider, or empty string if provider not found.
func SelectModelWithCostAwareness(cfg *config.Config, b *bead.Bead, providerName string) string {
	if cfg == nil || b == nil {
		return ""
	}

	provider, ok := cfg.Providers[providerName]
	if !ok {
		return ""
	}

	fileCount := bead.EstimatedFileCount(b)

	// For broad scope, prefer cheaper model if available
	if fileCount > broadScopeFileThreshold && provider.ModelCosts != nil {
		if _, hasCheaper := provider.ModelCosts[cheaperCodexModel]; hasCheaper {
			return cheaperCodexModel
		}
	}

	// Otherwise, fallback to default behavior (empty, will be filled by caller)
	// This is just a routing hint that returns the concrete model name
	return ""
}
