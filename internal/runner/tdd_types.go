package runner

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TDDCycleRunner is the interface for running TDD cycles within the runner
// package. It was previously defined in the execute package but belongs here
// since the runner is the only consumer.
type TDDCycleRunner interface {
	RunCycles(ctx context.Context, b *bead.Bead, cfg *config.Config) (TDDCycleResult, error)
}

// TDDCycleResult holds the aggregated telemetry from a TDD cycle run.
type TDDCycleResult struct {
	PhaseMetrics []pipeline.PhaseMetric
	OriginalTier string
	ActualTier   string
	Model        string
	DurationMs   int64
	CostUSD      float64
	InputTokens  int
	OutputTokens int
}
