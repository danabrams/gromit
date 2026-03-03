package execute

import (
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

func TestTDDCycleResultFields(t *testing.T) {
	result := TDDCycleResult{
		PhaseMetrics: []pipeline.PhaseMetric{{Phase: "red", DurationMs: 1}},
		OriginalTier: "original",
		ActualTier:   "actual",
		Model:        "test-model",
		DurationMs:   123,
		CostUSD:      4.56,
		InputTokens:  10,
		OutputTokens: 20,
	}

	if len(result.PhaseMetrics) != 1 {
		t.Fatalf("expected 1 phase metric, got %d", len(result.PhaseMetrics))
	}
	if result.DurationMs != 123 {
		t.Fatalf("unexpected DurationMs: %d", result.DurationMs)
	}
}
