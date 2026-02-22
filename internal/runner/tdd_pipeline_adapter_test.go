package runner

import (
	"context"
	"fmt"
	"strings"
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

// TestTDDPipelineAdapter_RunCycles_ConvertsPhaseMetricsToResult verifies that when
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

// TestTDDPipelineAdapter_RunCycles_PropagatesOrchestratorError verifies that when
// the tdd orchestrator returns an error, RunCycles propagates it.
func TestTDDPipelineAdapter_RunCycles_PropagatesOrchestratorError(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{},
		tddOrchestrator: &tddOrchestrator{
			runCyclesFn: func(_ context.Context, _ *runtypes.BeadContext, _ *coverage.CoverageTracker, _ []coverage.Criterion) error {
				return fmt.Errorf("orchestrator failed: red phase timed out")
			},
		},
	}
	b := &bead.Bead{ID: "b1", Title: "Test feature", ExpectedOutputs: []string{"implement X"}}
	cfg := &config.Config{}

	adapter := &TDDPipelineAdapter{runner: r}
	_, err := adapter.RunCycles(context.Background(), b, cfg)
	if err == nil {
		t.Fatal("want error from failing orchestrator, got nil")
	}
	if !strings.Contains(err.Error(), "orchestrator failed") {
		t.Errorf("error %q does not contain expected message %q", err.Error(), "orchestrator failed")
	}
}

// TestTDDPipelineAdapter_RunCycles_AppliesLayer1_3WhenNoExpectedOutputs verifies
// that when a bead has no ExpectedOutputs, RunCycles derives them from the title
// (Layer 1/3 fallback logic) and populates bead.ExpectedOutputs before calling
// the orchestrator.
func TestTDDPipelineAdapter_RunCycles_AppliesLayer1_3WhenNoExpectedOutputs(t *testing.T) {
	var capturedBead *bead.Bead
	r := &Runner{
		cfg: &config.Config{},
		tddOrchestrator: &tddOrchestrator{
			runCyclesFn: func(_ context.Context, bc *runtypes.BeadContext, _ *coverage.CoverageTracker, _ []coverage.Criterion) error {
				capturedBead = bc.Bead
				return nil
			},
		},
	}
	b := &bead.Bead{ID: "b1", Title: "Implement feature X"}
	cfg := &config.Config{}

	adapter := &TDDPipelineAdapter{runner: r}
	_, err := adapter.RunCycles(context.Background(), b, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBead == nil {
		t.Fatal("orchestrator was not called")
	}
	if len(capturedBead.ExpectedOutputs) == 0 {
		t.Fatal("want ExpectedOutputs populated from title, got empty")
	}
	if capturedBead.ExpectedOutputs[0] != "Implement feature X" {
		t.Errorf("ExpectedOutputs[0] = %q, want title %q", capturedBead.ExpectedOutputs[0], "Implement feature X")
	}
}
