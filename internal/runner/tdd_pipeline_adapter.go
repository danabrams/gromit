package runner

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TDDPipelineAdapter bridges the runner's TDD orchestration to the pipeline's
// TDDCycleRunner interface.
type TDDPipelineAdapter struct {
	runner *Runner
}

// RunCycles implements execute.TDDCycleRunner. It constructs a BeadContext from
// b and cfg, delegates to the runner's tddOrchestrator, aggregates per-phase
// metrics, and returns them as pipeline.PhaseMetrics.
func (a *TDDPipelineAdapter) RunCycles(ctx context.Context, b *bead.Bead, cfg *config.Config) (execute.TDDCycleResult, error) {
	bc := &runtypes.BeadContext{
		Bead:   b,
		Result: &runtypes.IterationResult{},
	}

	coverageTracker, coverageCriteria, err := buildCoverageTrackerFromSpec(bc)
	if err != nil {
		return execute.TDDCycleResult{}, err
	}

	if err := a.runner.tddOrchestrator.RunCycles(ctx, bc, coverageTracker, coverageCriteria); err != nil {
		return execute.TDDCycleResult{}, err
	}

	aggregateTDDPhaseMetricsToResult(bc)

	return execute.TDDCycleResult{
		PhaseMetrics: convertPhaseMetrics(bc.Result.PhaseMetrics),
	}, nil
}

// convertPhaseMetrics maps runtypes.PhaseMetric entries to pipeline.PhaseMetrics.
func convertPhaseMetrics(metrics []runtypes.PhaseMetric) []pipeline.PhaseMetrics {
	out := make([]pipeline.PhaseMetrics, len(metrics))
	for i, m := range metrics {
		out[i] = pipeline.PhaseMetrics{
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
