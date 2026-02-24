package runner

import (
	"context"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func applyLayer3Requirements(ctx context.Context, cfg *config.Config, outputs []string, title, description string, invoke func(ctx context.Context, prompt, tier string) (*provider.Result, error)) ([]string, bool) {
	if len(outputs) > 1 {
		return outputs, false
	}
	llmOutputs := extractRequirementsViaLLM(ctx, cfg, title, description, invoke)
	if llmOutputs != nil {
		return llmOutputs, true
	}
	return outputs, false
}

// tierRank returns a numeric rank for a tier string (higher = more capable).
func tierRank(tier string) int {
	switch tier {
	case "high":
		return 2
	case "medium":
		return 1
	default:
		return 0
	}
}

// aggregateTDDPhaseMetricsToResult sums cost and token totals from all
// PhaseMetrics into bc.Result and sets Model to the highest-tier model used.
func aggregateTDDPhaseMetricsToResult(bc *runtypes.BeadContext) {
	if bc == nil || bc.Result == nil {
		return
	}
	var totalCost float64
	var totalInput, totalOutput int
	bestTierRank := -1
	bestModel := ""
	for _, pm := range bc.Result.PhaseMetrics {
		totalCost += pm.CostUSD
		totalInput += pm.InputTokens
		totalOutput += pm.OutputTokens
		if r := tierRank(pm.Tier); r > bestTierRank {
			bestTierRank = r
			bestModel = pm.Model
		}
	}
	bc.Result.CostUSD = totalCost
	bc.Result.InputTokens = totalInput
	bc.Result.OutputTokens = totalOutput
	if bestModel != "" {
		bc.Result.Model = bestModel
	}
}
