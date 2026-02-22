package runner

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestTDDPipelineAdapter_ImplementsInterface verifies that TDDPipelineAdapter
// satisfies the execute.TDDCycleRunner interface.
func TestTDDPipelineAdapter_ImplementsInterface(t *testing.T) {
	var a *TDDPipelineAdapter
	if _, ok := any(a).(execute.TDDCycleRunner); !ok {
		t.Fatal("TDDPipelineAdapter does not implement execute.TDDCycleRunner")
	}
}

// TestTDDPipelineAdapter_RunCycles_ConvertsPhaseMet ricsToResult verifies that when
// the underlying tdd orchestrator appends PhaseMetrics to the BeadContext,
// RunCycles returns them converted to pipeline.PhaseMetrics in TDDCycleResult.
func TestTDDPipelineAdapter_RunCycles_ConvertsPhaseMetricsToResult(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{},
		tddOrchestrator: &tddOrchestrator{
			runCyclesFn: func(_ context.Context, bc *runtypes.BeadContext, _ *coverage.CoverageTracker, _ []coverage.Criterion) error {
				bc.Result.PhaseMetrics = []runtypes.PhaseMetric{
					{Phase: "red", DurationMs: 100, CostUSD: 0.01, InputTokens: 100, OutputTokens: 50, Model: "sonnet", Tier: "medium"},
					{Phase: "green", DurationMs: 200, CostUSD: 0.02, InputTokens: 200, OutputTokens: 100, Model: "sonnet", Tier: "medium"},
				}
				return nil
			},
		},
	}
	b := &bead.Bead{ID: "b1", Title: "Test feature", ExpectedOutputs: []string{"implement X"}}
	cfg := &config.Config{}

	adapter := &TDDPipelineAdapter{runner: r}
	result, err := adapter.RunCycles(context.Background(), b, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.PhaseMetrics) != 2 {
		t.Fatalf("want 2 phase metrics, got %d", len(result.PhaseMetrics))
	}
	if result.PhaseMetrics[0].Phase != "red" {
		t.Errorf("PhaseMetrics[0].Phase = %q, want %q", result.PhaseMetrics[0].Phase, "red")
	}
	if result.PhaseMetrics[1].Phase != "green" {
		t.Errorf("PhaseMetrics[1].Phase = %q, want %q", result.PhaseMetrics[1].Phase, "green")
	}
	if result.PhaseMetrics[0].CostUSD != 0.01 {
		t.Errorf("PhaseMetrics[0].CostUSD = %f, want 0.01", result.PhaseMetrics[0].CostUSD)
	}
}
