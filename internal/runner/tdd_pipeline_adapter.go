package runner

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// Compile-time check: *TDDPipelineAdapter must implement execute.TDDCycleRunner.
var _ execute.TDDCycleRunner = (*TDDPipelineAdapter)(nil)

// TDDPipelineAdapter bridges the runner's TDD orchestration to the pipeline's
// TDDCycleRunner interface.
type TDDPipelineAdapter struct {
	runner *Runner
}

// RunCycles implements execute.TDDCycleRunner. It applies Layer 1/3 expected
// outputs logic, builds a coverage tracker, delegates to the runner's
// tddOrchestrator, aggregates per-phase metrics, and returns them as
// pipeline.PhaseMetric.
func (a *TDDPipelineAdapter) RunCycles(ctx context.Context, b *bead.Bead, cfg *config.Config) (execute.TDDCycleResult, error) {
	// Apply Layer 1/3 expected outputs logic: when the bead has no
	// ExpectedOutputs, derive them from description parsing or title fallback.
	if len(b.ExpectedOutputs) == 0 {
		effectiveOutputs := tddExpectedOutputsOrTitle(b)
		b.ExpectedOutputs = append([]string(nil), effectiveOutputs...)
	}

	bc := &runtypes.BeadContext{
		Bead:   b,
		Result: &runtypes.IterationResult{},
	}

	coverageTracker, coverageCriteria, err := buildCoverageTrackerFromSpec(bc)
	if err != nil {
		return execute.TDDCycleResult{}, err
	}

	startTime := time.Now()
	cycleErr := a.runner.tddOrchestrator.RunCycles(ctx, bc, coverageTracker, coverageCriteria)
	durationMs := time.Since(startTime).Milliseconds()

	aggregateTDDPhaseMetricsToResult(bc)
	originalTier, actualTier := tddTierProvenance(bc.Result.PhaseMetrics)
	// Fall back to bc-level tier when PhaseMetrics is empty (telemetry
	// accumulated directly into bc.Result by the InvokeFn).
	if originalTier == "" {
		originalTier = bc.Tier
	}
	if actualTier == "" {
		actualTier = bc.Tier
	}

	result := execute.TDDCycleResult{
		PhaseMetrics: convertPhaseMetrics(bc.Result.PhaseMetrics),
		OriginalTier: originalTier,
		ActualTier:   actualTier,
		Model:        bc.Result.Model,
		DurationMs:   durationMs,
		CostUSD:      bc.Result.CostUSD,
		InputTokens:  bc.Result.InputTokens,
		OutputTokens: bc.Result.OutputTokens,
	}
	if cycleErr != nil {
		return result, cycleErr
	}
	return result, nil
}

func tddTierProvenance(metrics []runtypes.PhaseMetric) (originalTier, actualTier string) {
	bestRank := -1
	for _, m := range metrics {
		if originalTier == "" && m.Tier != "" {
			originalTier = m.Tier
		}
		if r := tierRank(m.Tier); m.Tier != "" && r > bestRank {
			bestRank = r
			actualTier = m.Tier
		}
	}
	return originalTier, actualTier
}

// convertPhaseMetrics maps runtypes.PhaseMetric entries to pipeline.PhaseMetric.
func convertPhaseMetrics(metrics []runtypes.PhaseMetric) []pipeline.PhaseMetric {
	out := make([]pipeline.PhaseMetric, len(metrics))
	for i, m := range metrics {
		out[i] = pipeline.PhaseMetric{
			Phase:        m.Phase,
			DurationMs:   m.DurationMs,
			CostUSD:      m.CostUSD,
			InputTokens:  m.InputTokens,
			OutputTokens: m.OutputTokens,
			Model:        m.Model,
		}
	}
	return out
}
