package runner

import (
	"math"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestAppendTDDPhaseMetric_AppendsPhaseMetricToBcResult verifies that
// appendTDDPhaseMetric appends one PhaseMetric entry to bc.Result.PhaseMetrics
// with the correct phase, cycle, model, tier, delta tokens/cost, duration, and
// success flag — capturing the data that InvokeFn previously discarded.
func TestAppendTDDPhaseMetric_AppendsPhaseMetricToBcResult(t *testing.T) {
	bc := &runtypes.BeadContext{
		Bead:  &bead.Bead{ID: "bead-1"},
		Model: "claude-sonnet-4-6",
		Tier:  "medium",
		Result: &runtypes.IterationResult{
			CostUSD:      0.05,
			InputTokens:  500,
			OutputTokens: 250,
		},
	}
	start := time.Now().Add(-200 * time.Millisecond)

	appendTDDPhaseMetric(bc, "red", 1, 0.02, 200, 100, start)

	if len(bc.Result.PhaseMetrics) != 1 {
		t.Fatalf("expected 1 PhaseMetric, got %d", len(bc.Result.PhaseMetrics))
	}
	pm := bc.Result.PhaseMetrics[0]
	if pm.Phase != "red" {
		t.Errorf("Phase = %q, want %q", pm.Phase, "red")
	}
	if pm.CycleNumber != 1 {
		t.Errorf("CycleNumber = %d, want 1", pm.CycleNumber)
	}
	if pm.Tier != "medium" {
		t.Errorf("Tier = %q, want %q", pm.Tier, "medium")
	}
	if pm.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", pm.Model, "claude-sonnet-4-6")
	}
	if pm.BeadID != "bead-1" {
		t.Errorf("BeadID = %q, want %q", pm.BeadID, "bead-1")
	}
	// Use tolerance for float64 comparison to avoid constant-vs-runtime precision mismatch.
	wantCostUSD := float64(0.05) - float64(0.02)
	if math.Abs(pm.CostUSD-wantCostUSD) > 1e-10 {
		t.Errorf("CostUSD = %.20f, want %.20f", pm.CostUSD, wantCostUSD)
	}
	wantInputTokens := 500 - 200
	if pm.InputTokens != wantInputTokens {
		t.Errorf("InputTokens = %d, want %d", pm.InputTokens, wantInputTokens)
	}
	wantOutputTokens := 250 - 100
	if pm.OutputTokens != wantOutputTokens {
		t.Errorf("OutputTokens = %d, want %d", pm.OutputTokens, wantOutputTokens)
	}
	if pm.DurationMs <= 0 {
		t.Error("DurationMs should be positive")
	}
	if !pm.Success {
		t.Error("Success should be true")
	}
}
