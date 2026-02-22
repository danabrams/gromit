package runner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// buildTDDCycleRunner creates a TDDCycleRunner adapter backed by a Runner with a
// configured tddOrchestrator. Inject the result into execute.Build via WithTDDCycleRunner.
func buildTDDCycleRunner(_ *config.Config, _ *prompt.Renderer, _ *provider.Router, _ io.Writer) execute.TDDCycleRunner {
	return nil
}

func snapshotIterationUsage(result *runtypes.IterationResult) (costUSD float64, inputTokens int, outputTokens int) {
	if result == nil {
		return 0, 0, 0
	}
	return result.CostUSD, result.InputTokens, result.OutputTokens
}

func phaseUsageDelta(
	result *runtypes.IterationResult,
	beforeCostUSD float64,
	beforeInputTokens int,
	beforeOutputTokens int,
) (costUSD float64, inputTokens int, outputTokens int) {
	afterCostUSD, afterInputTokens, afterOutputTokens := snapshotIterationUsage(result)
	costUSD = afterCostUSD - beforeCostUSD
	inputTokens = afterInputTokens - beforeInputTokens
	outputTokens = afterOutputTokens - beforeOutputTokens
	if costUSD < 0 {
		costUSD = 0
	}
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	return costUSD, inputTokens, outputTokens
}

// appendTDDPhaseMetric records a per-invocation PhaseMetric for a TDD cycle.
// It computes the cost/token delta since the before-snapshot and appends a
// PhaseMetric entry to bc.Result.PhaseMetrics.
func appendTDDPhaseMetric(
	bc *runtypes.BeadContext,
	phase string,
	cycleNumber int,
	beforeCostUSD float64,
	beforeInputTokens int,
	beforeOutputTokens int,
	start time.Time,
) {
	if bc == nil || bc.Result == nil || bc.Bead == nil {
		return
	}
	costUSD, inputTokens, outputTokens := phaseUsageDelta(bc.Result, beforeCostUSD, beforeInputTokens, beforeOutputTokens)
	durationMs := int64(0)
	if !start.IsZero() {
		if d := time.Since(start).Milliseconds(); d > 0 {
			durationMs = d
		}
	}
	if cycleNumber < 1 {
		cycleNumber = 1
	}
	bc.Result.PhaseMetrics = append(bc.Result.PhaseMetrics, runtypes.PhaseMetric{
		Phase:        phase,
		CycleNumber:  cycleNumber,
		BeadID:       bc.Bead.ID,
		Model:        bc.Model,
		Tier:         bc.Tier,
		CostUSD:      costUSD,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		DurationMs:   durationMs,
		Success:      true,
	})
}

type tddOrchestrator struct {
	runCyclesFn func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error
}

func (o *tddOrchestrator) RunCycles(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
	if o != nil && o.runCyclesFn != nil {
		return o.runCyclesFn(ctx, bc, tracker, criteria)
	}
	return fmt.Errorf("tdd orchestrator is not configured")
}
