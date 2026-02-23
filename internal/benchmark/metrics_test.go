package benchmark

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAggregateModeMetrics_ComputesTierAndOverallTotals(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "run.jsonl")
	content := "" +
		"{\"iteration\":1,\"actual_tier\":\"low\",\"input_tokens\":100,\"output_tokens\":20,\"cost_usd\":0.10}\n" +
		"{\"iteration\":2,\"actual_tier\":\"medium\",\"input_tokens\":200,\"output_tokens\":40,\"cost_usd\":0.20}\n" +
		"{\"iteration\":3,\"actual_tier\":\"high\",\"input_tokens\":300,\"output_tokens\":60,\"cost_usd\":0.30}\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture log: %v", err)
	}

	summaries, err := AggregateModeMetrics([]ModeLogInput{{
		Mode:          "single_pass",
		RunStartedAt:  time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC),
		RunFinishedAt: time.Date(2026, 2, 23, 12, 2, 0, 0, time.UTC),
		LogPath:       logPath,
	}})
	if err != nil {
		t.Fatalf("AggregateModeMetrics() error = %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(summaries))
	}
	got := summaries[0]

	if got.TotalInput != 600 || got.TotalOutput != 120 {
		t.Fatalf("totals input/output = %d/%d, want 600/120", got.TotalInput, got.TotalOutput)
	}
	if got.TotalCostUSD != 0.60 {
		t.Fatalf("total cost = %v, want 0.60", got.TotalCostUSD)
	}

	if got.TierTotals.Low.InputTokens != 100 || got.TierTotals.Low.OutputTokens != 20 || got.TierTotals.Low.CostUSD != 0.10 {
		t.Fatalf("low tier totals = %+v", got.TierTotals.Low)
	}
	if got.TierTotals.Medium.InputTokens != 200 || got.TierTotals.Medium.OutputTokens != 40 || got.TierTotals.Medium.CostUSD != 0.20 {
		t.Fatalf("medium tier totals = %+v", got.TierTotals.Medium)
	}
	if got.TierTotals.High.InputTokens != 300 || got.TierTotals.High.OutputTokens != 60 || got.TierTotals.High.CostUSD != 0.30 {
		t.Fatalf("high tier totals = %+v", got.TierTotals.High)
	}
}
