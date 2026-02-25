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

func TestAggregateModeMetrics_AggregatesReviewMetricsAndFinalValidationOutcome(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "review.jsonl")
	content := "" +
		"{\"iteration\":1,\"validated\":true}\n" +
		"{\"type\":\"review\",\"review_type\":\"light\",\"iteration\":1,\"passed\":true,\"fixes_applied\":4,\"beads_created\":2,\"backlog_created\":1}\n" +
		"{\"iteration\":2,\"validated\":false}\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture log: %v", err)
	}

	summaries, err := AggregateModeMetrics([]ModeLogInput{{
		Mode:          "single_pass",
		RunStartedAt:  time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC),
		RunFinishedAt: time.Date(2026, 2, 23, 12, 0, 3, 0, time.UTC),
		LogPath:       logPath,
	}})
	if err != nil {
		t.Fatalf("AggregateModeMetrics() error = %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(summaries))
	}
	got := summaries[0]

	if got.Quality.ReviewFixesApplied != 4 {
		t.Fatalf("review fixes applied = %d, want 4", got.Quality.ReviewFixesApplied)
	}
	if got.Quality.ReviewFindings != 3 {
		t.Fatalf("review findings = %d, want 3", got.Quality.ReviewFindings)
	}
	if got.Quality.FinalValidationPassed {
		t.Fatal("final validation passed = true, want false")
	}
}

func TestAggregateModeMetrics_ComputesCostQualityRatioFromTotalsAndAverageQuality(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "ratio.jsonl")
	content := "" +
		"{\"iteration\":1,\"cost_usd\":0.40,\"quality_score\":0.5}\n" +
		"{\"iteration\":2,\"cost_usd\":0.20,\"quality_score\":1.0}\n"
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

	if summaries[0].CostQualityRatio != 0.8 {
		t.Fatalf("cost_quality_ratio = %v, want 0.8", summaries[0].CostQualityRatio)
	}
}

func TestAggregateModeMetrics_AggregatesModelTotals(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "models.jsonl")
	content := "" +
		"{\"iteration\":1,\"model\":\"gpt-5.1-codex-mini\",\"input_tokens\":100,\"output_tokens\":20,\"cost_usd\":0.10}\n" +
		"{\"iteration\":2,\"model\":\"gpt-5.3-codex\",\"input_tokens\":200,\"output_tokens\":40,\"cost_usd\":0.20}\n" +
		"{\"iteration\":3,\"model\":\"gpt-5.1-codex-mini\",\"input_tokens\":300,\"output_tokens\":60,\"cost_usd\":0.30}\n"
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

	got := summaries[0].ModelTotals
	if len(got) != 2 {
		t.Fatalf("model total count = %d, want 2", len(got))
	}
	if got[0].Model != "gpt-5.1-codex-mini" || got[0].InputTokens != 400 || got[0].OutputTokens != 80 || got[0].CostUSD != 0.4 {
		t.Fatalf("first model totals = %+v", got[0])
	}
	if got[1].Model != "gpt-5.3-codex" || got[1].InputTokens != 200 || got[1].OutputTokens != 40 || got[1].CostUSD != 0.2 {
		t.Fatalf("second model totals = %+v", got[1])
	}
}
