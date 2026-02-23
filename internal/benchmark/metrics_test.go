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

func TestAggregateModeMetrics_UsesRunStartFinishForElapsedAndStableOrdering(t *testing.T) {
	tmpDir := t.TempDir()
	pathA := filepath.Join(tmpDir, "a.jsonl")
	pathZ := filepath.Join(tmpDir, "z.jsonl")
	if err := os.WriteFile(pathA, []byte("{\"iteration\":1}\n"), 0o644); err != nil {
		t.Fatalf("write a fixture: %v", err)
	}
	if err := os.WriteFile(pathZ, []byte("{\"iteration\":1}\n"), 0o644); err != nil {
		t.Fatalf("write z fixture: %v", err)
	}

	summaries, err := AggregateModeMetrics([]ModeLogInput{
		{
			Mode:          "z_mode",
			RunStartedAt:  time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC),
			RunFinishedAt: time.Date(2026, 2, 23, 12, 3, 5, 0, time.UTC),
			LogPath:       pathZ,
		},
		{
			Mode:          "a_mode",
			RunStartedAt:  time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC),
			RunFinishedAt: time.Date(2026, 2, 23, 12, 1, 30, 0, time.UTC),
			LogPath:       pathA,
		},
	})
	if err != nil {
		t.Fatalf("AggregateModeMetrics() error = %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}

	if summaries[0].Mode != "a_mode" || summaries[1].Mode != "z_mode" {
		t.Fatalf("mode ordering = [%s, %s], want [a_mode, z_mode]", summaries[0].Mode, summaries[1].Mode)
	}
	if summaries[0].ElapsedSeconds != 90 {
		t.Fatalf("a_mode elapsed = %d, want 90", summaries[0].ElapsedSeconds)
	}
	if summaries[1].ElapsedSeconds != 185 {
		t.Fatalf("z_mode elapsed = %d, want 185", summaries[1].ElapsedSeconds)
	}
}

func TestAggregateModeMetrics_AggregatesQualityAndFirstPassWithMissingOptionalFields(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "quality.jsonl")
	content := "" +
		"{\"iteration\":1,\"quality_score\":0.9,\"first_pass_success\":true}\n" +
		"{\"iteration\":2}\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture log: %v", err)
	}

	summaries, err := AggregateModeMetrics([]ModeLogInput{{
		Mode:          "single_pass",
		RunStartedAt:  time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC),
		RunFinishedAt: time.Date(2026, 2, 23, 12, 0, 2, 0, time.UTC),
		LogPath:       logPath,
	}})
	if err != nil {
		t.Fatalf("AggregateModeMetrics() error = %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(summaries))
	}
	got := summaries[0]

	if got.Quality.AverageScore != 0.45 {
		t.Fatalf("average quality = %v, want 0.45", got.Quality.AverageScore)
	}
	if got.Quality.FirstPassRate != 0.5 {
		t.Fatalf("first pass rate = %v, want 0.5", got.Quality.FirstPassRate)
	}
}
