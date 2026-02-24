package benchmark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReport_WritesJSONAndMarkdownArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	_, err := WriteReport(ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest: ManifestMetadata{
			ID:         "tdd-vs-single-pass",
			BaseCommit: "abc123",
			Beads:      []string{"gromit-1", "gromit-2", "gromit-3"},
		},
		Modes: []ModeSummary{{Mode: "single_pass"}},
	})
	if err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.json")
	mdPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.md")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("json artifact missing: %v", err)
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("markdown artifact missing: %v", err)
	}
}

func TestWriteReport_JSONIncludesMetadataModeTierQualityAndWinners(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	_, err := WriteReport(ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest: ManifestMetadata{
			ID:              "tdd-vs-single-pass",
			BaseCommit:      "abc123",
			Beads:           []string{"gromit-1", "gromit-2", "gromit-3"},
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
		Modes: []ModeSummary{
			{
				Mode:             "single_pass",
				ElapsedSeconds:   120,
				TotalInput:       1000,
				TotalOutput:      500,
				TotalCostUSD:     1.25,
				TierTotals:       TierTotals{Low: TierTotalsRow{InputTokens: 600, OutputTokens: 300, CostUSD: 0.5}},
				Quality:          QualityMetrics{AverageScore: 0.88, FirstPassRate: 0.67, ReviewFindings: 3, ReviewFixesApplied: 2, FinalValidationPassed: true},
				CostQualityRatio: 1.42,
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.json")
	reportBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json artifact: %v", err)
	}

	var payload struct {
		Manifest struct {
			ID          string `json:"id"`
			BaseCommit  string `json:"base_commit"`
			Provider    string `json:"provider"`
			ModelFamily string `json:"model_family"`
		} `json:"manifest"`
		Modes []struct {
			Mode       string `json:"mode"`
			Elapsed    int    `json:"elapsed_seconds"`
			TierTotals struct {
				Low struct {
					Input int `json:"input_tokens"`
				} `json:"low"`
			} `json:"tier_totals"`
			Quality struct {
				Average float64 `json:"average_score"`
			} `json:"quality"`
		} `json:"modes"`
		WinnerHints struct {
			Fastest         string `json:"fastest"`
			Cheapest        string `json:"cheapest"`
			BestQuality     string `json:"best_quality"`
			BestCostQuality string `json:"best_cost_quality"`
		} `json:"winner_hints"`
	}
	if err := json.Unmarshal(reportBytes, &payload); err != nil {
		t.Fatalf("unmarshal json artifact: %v", err)
	}

	if payload.Manifest.ID != "tdd-vs-single-pass" || payload.Manifest.BaseCommit != "abc123" {
		t.Fatalf("manifest mismatch: %+v", payload.Manifest)
	}
	if payload.Manifest.Provider != "openai" || payload.Manifest.ModelFamily != "gpt-5" {
		t.Fatalf("manifest provider/model mismatch: %+v", payload.Manifest)
	}
	if len(payload.Modes) != 1 {
		t.Fatalf("mode count = %d, want 1", len(payload.Modes))
	}
	if payload.Modes[0].Mode != "single_pass" || payload.Modes[0].Elapsed != 120 {
		t.Fatalf("mode summary mismatch: %+v", payload.Modes[0])
	}
	if payload.Modes[0].TierTotals.Low.Input != 600 {
		t.Fatalf("tier low input = %d, want 600", payload.Modes[0].TierTotals.Low.Input)
	}
	if payload.Modes[0].Quality.Average != 0.88 {
		t.Fatalf("quality average = %v, want 0.88", payload.Modes[0].Quality.Average)
	}
	if payload.WinnerHints.Fastest != "single_pass" || payload.WinnerHints.Cheapest != "single_pass" || payload.WinnerHints.BestQuality != "single_pass" || payload.WinnerHints.BestCostQuality != "single_pass" {
		t.Fatalf("winner hints mismatch: %+v", payload.WinnerHints)
	}
}

func TestWriteReport_MarkdownRendersStableOrderedTables(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	input := ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest:  ManifestMetadata{ID: "tdd-vs-single-pass", BaseCommit: "abc123"},
		Modes: []ModeSummary{
			{
				Mode:             "tdd_shared_context",
				ElapsedSeconds:   140,
				TotalInput:       1200,
				TotalOutput:      600,
				TotalCostUSD:     1.5,
				TierTotals:       TierTotals{Low: TierTotalsRow{InputTokens: 700, CostUSD: 0.6}},
				Quality:          QualityMetrics{AverageScore: 0.85, FirstPassRate: 0.5, ReviewFindings: 2, ReviewFixesApplied: 2, FinalValidationPassed: true},
				CostQualityRatio: 1.1,
			},
			{
				Mode:             "single_pass",
				ElapsedSeconds:   110,
				TotalInput:       900,
				TotalOutput:      450,
				TotalCostUSD:     1.2,
				TierTotals:       TierTotals{Low: TierTotalsRow{InputTokens: 600, CostUSD: 0.5}},
				Quality:          QualityMetrics{AverageScore: 0.8, FirstPassRate: 0.33, ReviewFindings: 3, ReviewFixesApplied: 1, FinalValidationPassed: false},
				CostQualityRatio: 1.0,
			},
		},
	}

	_, err := WriteReport(input)
	if err != nil {
		t.Fatalf("first WriteReport() error = %v", err)
	}

	mdPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.md")
	first, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read first markdown artifact: %v", err)
	}

	_, err = WriteReport(input)
	if err != nil {
		t.Fatalf("second WriteReport() error = %v", err)
	}
	second, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read second markdown artifact: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("markdown artifact changed across repeated runs\nfirst:\n%s\nsecond:\n%s", string(first), string(second))
	}

	content := string(first)
	for _, section := range []string{"## Per-Mode Summary", "## By-Tier Totals", "## Quality Metrics", "## Winner Hints"} {
		if !strings.Contains(content, section) {
			t.Fatalf("markdown missing section %q\n%s", section, content)
		}
	}
	idxSingle := strings.Index(content, "| single_pass |")
	idxShared := strings.Index(content, "| tdd_shared_context |")
	if idxSingle == -1 || idxShared == -1 {
		t.Fatalf("mode rows missing from markdown\n%s", content)
	}
	if idxSingle > idxShared {
		t.Fatalf("mode ordering not stable: single_pass appears after tdd_shared_context\n%s", content)
	}
}

func TestWriteReport_WinnerHintsUseStableTieBreakAndLowerCostQualityRatio(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	_, err := WriteReport(ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest:  ManifestMetadata{ID: "tdd-vs-single-pass", BaseCommit: "abc123"},
		Modes: []ModeSummary{
			{
				Mode:             "z_mode",
				ElapsedSeconds:   100,
				TotalCostUSD:     1.0,
				Quality:          QualityMetrics{AverageScore: 0.9},
				CostQualityRatio: 1.3,
			},
			{
				Mode:             "a_mode",
				ElapsedSeconds:   100,
				TotalCostUSD:     1.0,
				Quality:          QualityMetrics{AverageScore: 0.9},
				CostQualityRatio: 0.9,
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.json")
	reportBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json artifact: %v", err)
	}

	var payload struct {
		WinnerHints WinnerHints `json:"winner_hints"`
	}
	if err := json.Unmarshal(reportBytes, &payload); err != nil {
		t.Fatalf("unmarshal json artifact: %v", err)
	}

	if payload.WinnerHints.Fastest != "a_mode" || payload.WinnerHints.Cheapest != "a_mode" || payload.WinnerHints.BestQuality != "a_mode" {
		t.Fatalf("tie-break winner hints mismatch: %+v", payload.WinnerHints)
	}
	if payload.WinnerHints.BestCostQuality != "a_mode" {
		t.Fatalf("best_cost_quality = %q, want %q", payload.WinnerHints.BestCostQuality, "a_mode")
	}
}
func TestRunPhase3Measurement_ComputesMediansAndCacheHitRatesByPromptClass(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	baselineLog := filepath.Join(tmpDir, "baseline.jsonl")
	optimizedLog := filepath.Join(tmpDir, "optimized.jsonl")

	baselineContent := "" +
		"{\"iteration\":1,\"input_tokens\":100,\"cost_usd\":1.0,\"success\":true}\n" +
		"{\"iteration\":2,\"input_tokens\":140,\"cost_usd\":1.4,\"success\":false}\n" +
		"{\"iteration\":3,\"input_tokens\":120,\"cost_usd\":1.2,\"success\":true}\n"
	optimizedContent := "" +
		"{\"iteration\":1,\"input_tokens\":80,\"cost_usd\":0.8,\"success\":true,\"cache_class\":\"render_static_build\",\"cache_hit\":true}\n" +
		"{\"iteration\":2,\"input_tokens\":100,\"cost_usd\":1.0,\"success\":true,\"cache_class\":\"render_static_build\",\"cache_hit\":false,\"cache_miss\":true}\n" +
		"{\"iteration\":3,\"input_tokens\":90,\"cost_usd\":0.9,\"success\":true,\"cache_class\":\"utility_summarization\",\"cache_hit\":true}\n"

	if err := os.WriteFile(baselineLog, []byte(baselineContent), 0o644); err != nil {
		t.Fatalf("write baseline log: %v", err)
	}
	if err := os.WriteFile(optimizedLog, []byte(optimizedContent), 0o644); err != nil {
		t.Fatalf("write optimized log: %v", err)
	}

	report, err := RunPhase3Measurement(Phase3MeasurementInput{
		Timestamp:        "20260224T120000Z",
		BaselineLogPath:  baselineLog,
		OptimizedLogPath: optimizedLog,
	})
	if err != nil {
		t.Fatalf("RunPhase3Measurement() error = %v", err)
	}

	if report.Baseline.MedianInputTokens != 120 {
		t.Fatalf("baseline median input = %d, want 120", report.Baseline.MedianInputTokens)
	}
	if report.Baseline.MedianCostUSD != 1.2 {
		t.Fatalf("baseline median cost = %v, want 1.2", report.Baseline.MedianCostUSD)
	}
	if report.Baseline.MedianSuccessRate != 0.67 {
		t.Fatalf("baseline median success = %v, want 0.67", report.Baseline.MedianSuccessRate)
	}

	if report.Optimized.MedianInputTokens != 90 {
		t.Fatalf("optimized median input = %d, want 90", report.Optimized.MedianInputTokens)
	}
	if report.Optimized.MedianCostUSD != 0.9 {
		t.Fatalf("optimized median cost = %v, want 0.9", report.Optimized.MedianCostUSD)
	}
	if report.Optimized.MedianSuccessRate != 1.0 {
		t.Fatalf("optimized median success = %v, want 1.0", report.Optimized.MedianSuccessRate)
	}

	if len(report.CacheHitRatesByClass) != 2 {
		t.Fatalf("cache hit-rate class count = %d, want 2", len(report.CacheHitRatesByClass))
	}
	if report.CacheHitRatesByClass["render_static_build"] != 0.5 {
		t.Fatalf("cache hit-rate render_static_build = %v, want 0.5", report.CacheHitRatesByClass["render_static_build"])
	}
	if report.CacheHitRatesByClass["utility_summarization"] != 1.0 {
		t.Fatalf("cache hit-rate utility_summarization = %v, want 1.0", report.CacheHitRatesByClass["utility_summarization"])
	}
}

func TestRunPhase3Measurement_FlagsKillSwitchRollbackOnSuccessRegression(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	baselineLog := filepath.Join(tmpDir, "baseline.jsonl")
	optimizedLog := filepath.Join(tmpDir, "optimized.jsonl")

	baselineContent := "" +
		"{\"iteration\":1,\"input_tokens\":100,\"cost_usd\":1.0,\"success\":true}\n" +
		"{\"iteration\":2,\"input_tokens\":120,\"cost_usd\":1.2,\"success\":true}\n" +
		"{\"iteration\":3,\"input_tokens\":110,\"cost_usd\":1.1,\"success\":true}\n"
	optimizedContent := "" +
		"{\"iteration\":1,\"input_tokens\":90,\"cost_usd\":0.9,\"success\":false}\n" +
		"{\"iteration\":2,\"input_tokens\":95,\"cost_usd\":0.95,\"success\":false}\n" +
		"{\"iteration\":3,\"input_tokens\":92,\"cost_usd\":0.92,\"success\":true}\n"

	if err := os.WriteFile(baselineLog, []byte(baselineContent), 0o644); err != nil {
		t.Fatalf("write baseline log: %v", err)
	}
	if err := os.WriteFile(optimizedLog, []byte(optimizedContent), 0o644); err != nil {
		t.Fatalf("write optimized log: %v", err)
	}

	report, err := RunPhase3Measurement(Phase3MeasurementInput{
		Timestamp:        "20260224T121500Z",
		BaselineLogPath:  baselineLog,
		OptimizedLogPath: optimizedLog,
	})
	if err != nil {
		t.Fatalf("RunPhase3Measurement() error = %v", err)
	}

	if !report.Rollback.KillSwitchRecommended {
		t.Fatal("KillSwitchRecommended = false, want true")
	}
	if len(report.Rollback.Triggers) == 0 {
		t.Fatal("rollback triggers = empty, want non-empty")
	}
	if report.Rollback.Triggers[0] != "success_rate_regression" {
		t.Fatalf("first rollback trigger = %q, want %q", report.Rollback.Triggers[0], "success_rate_regression")
	}
}
