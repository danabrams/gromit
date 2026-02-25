package benchmark

import (
	"bytes"
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
			LowTierModel:    "gpt-5.1-codex-mini",
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
				ModelTotals:      []ModelTotalsRow{{Model: "gpt-5.1-codex-mini", InputTokens: 600, OutputTokens: 300, CostUSD: 0.5}},
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
			ModelTotals []struct {
				Model string `json:"model"`
				Input int    `json:"input_tokens"`
			} `json:"model_totals"`
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
	if len(payload.Modes[0].ModelTotals) != 1 || payload.Modes[0].ModelTotals[0].Model != "gpt-5.1-codex-mini" || payload.Modes[0].ModelTotals[0].Input != 600 {
		t.Fatalf("model totals mismatch: %+v", payload.Modes[0].ModelTotals)
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
				ModelTotals:      []ModelTotalsRow{{Model: "gpt-5.1-codex-mini", InputTokens: 700, OutputTokens: 350, CostUSD: 0.6}},
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
				ModelTotals:      []ModelTotalsRow{{Model: "gpt-5.1-codex-mini", InputTokens: 600, OutputTokens: 300, CostUSD: 0.5}},
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
	for _, section := range []string{"## Per-Mode Summary", "## By-Tier Totals", "## By-Model Totals", "## Quality Metrics", "## Winner Hints"} {
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
func TestWriteReport_JSONArtifactModelTotalsAreSorted(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	unsortedInput := ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest: ManifestMetadata{
			ID:         "test-determinism",
			BaseCommit: "abc123",
		},
		Modes: []ModeSummary{
			{
				Mode: "test_mode",
				ModelTotals: []ModelTotalsRow{
					{Model: "z-model", InputTokens: 100, OutputTokens: 50, CostUSD: 1.0},
					{Model: "a-model", InputTokens: 150, OutputTokens: 75, CostUSD: 1.5},
					{Model: "m-model", InputTokens: 120, OutputTokens: 60, CostUSD: 1.2},
				},
			},
		},
	}

	_, err := WriteReport(unsortedInput)
	if err != nil {
		t.Fatalf("first WriteReport() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "test-determinism", "20260223T120000Z.json")
	firstJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read first json artifact: %v", err)
	}

	sortedInput := ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest: ManifestMetadata{
			ID:         "test-determinism",
			BaseCommit: "abc123",
		},
		Modes: []ModeSummary{
			{
				Mode: "test_mode",
				ModelTotals: []ModelTotalsRow{
					{Model: "a-model", InputTokens: 150, OutputTokens: 75, CostUSD: 1.5},
					{Model: "m-model", InputTokens: 120, OutputTokens: 60, CostUSD: 1.2},
					{Model: "z-model", InputTokens: 100, OutputTokens: 50, CostUSD: 1.0},
				},
			},
		},
	}

	_, err = WriteReport(sortedInput)
	if err != nil {
		t.Fatalf("second WriteReport() error = %v", err)
	}

	secondJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read second json artifact: %v", err)
	}

	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("json artifact not deterministic for unordered ModelTotals\nfirst:\n%s\nsecond:\n%s", string(firstJSON), string(secondJSON))
	}
}

func TestWriteReport_BeadsArrayIsSorted(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	unsortedBeads := ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest: ManifestMetadata{
			ID:         "test-beads-determinism",
			BaseCommit: "abc123",
			Beads:      []string{"zebra", "apple", "monkey", "banana"},
		},
		Modes: []ModeSummary{{Mode: "test_mode"}},
	}

	_, err := WriteReport(unsortedBeads)
	if err != nil {
		t.Fatalf("first WriteReport() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "test-beads-determinism", "20260223T120000Z.json")
	firstJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read first json artifact: %v", err)
	}

	sortedBeads := ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest: ManifestMetadata{
			ID:         "test-beads-determinism",
			BaseCommit: "abc123",
			Beads:      []string{"apple", "banana", "monkey", "zebra"},
		},
		Modes: []ModeSummary{{Mode: "test_mode"}},
	}

	_, err = WriteReport(sortedBeads)
	if err != nil {
		t.Fatalf("second WriteReport() error = %v", err)
	}

	secondJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read second json artifact: %v", err)
	}

	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("json artifact not deterministic for unordered Beads\nfirst:\n%s\nsecond:\n%s", string(firstJSON), string(secondJSON))
	}
}

func TestWriteReport_NoArtifactDuplication(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	input := ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest: ManifestMetadata{
			ID:         "test-no-duplication",
			BaseCommit: "abc123",
		},
		Modes: []ModeSummary{{Mode: "test_mode"}},
	}

	_, err := WriteReport(input)
	if err != nil {
		t.Fatalf("first WriteReport() error = %v", err)
	}

	resultDir := filepath.Join(".gromit", "benchmarks", "results", "test-no-duplication")
	files, err := os.ReadDir(resultDir)
	if err != nil {
		t.Fatalf("read results directory: %v", err)
	}
	firstCount := len(files)

	_, err = WriteReport(input)
	if err != nil {
		t.Fatalf("second WriteReport() error = %v", err)
	}

	files, err = os.ReadDir(resultDir)
	if err != nil {
		t.Fatalf("read results directory after second write: %v", err)
	}
	secondCount := len(files)

	if firstCount != secondCount {
		t.Fatalf("artifact count changed after repeated writes: first = %d, second = %d", firstCount, secondCount)
	}
	if firstCount != 2 {
		t.Fatalf("expected 2 artifacts (json + markdown), got %d", firstCount)
	}
}

func TestWriteReport_CanonicalBenchmarkReportFixture(t *testing.T) {
	tmpDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tmp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	repoRoot := repoRootFrom(t, origWD)
	fixtureDir := filepath.Join(repoRoot, ".gromit", "reports", "curated", "benchmark-report")

	input := ReportInput{
		Timestamp: "20260225T120000Z",
		Manifest: ManifestMetadata{
			ID:              "tdd-vs-single-pass",
			BaseCommit:      "abc123",
			Beads:           []string{"gromit-1", "gromit-2", "gromit-3", "gromit-4", "gromit-5"},
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
		Modes: []ModeSummary{
			{
				Mode:           "single_pass",
				ElapsedSeconds: 60,
				TotalInput:     1200,
				TotalOutput:    600,
				TotalCostUSD:   1.5,
				TierTotals: TierTotals{
					Low:    TierTotalsRow{InputTokens: 1200, OutputTokens: 600, CostUSD: 1.5},
					Medium: TierTotalsRow{InputTokens: 0, OutputTokens: 0, CostUSD: 0},
					High:   TierTotalsRow{InputTokens: 0, OutputTokens: 0, CostUSD: 0},
				},
				Quality: QualityMetrics{
					AverageScore:          0.9,
					FirstPassRate:         0.75,
					ReviewFindings:        1,
					ReviewFixesApplied:    0,
					FinalValidationPassed: true,
				},
				CostQualityRatio: 1.666667,
			},
		},
	}

	if _, err := WriteReport(input); err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	resultDir := filepath.Join(".gromit", "benchmarks", "results", input.Manifest.ID)
	jsonPath := filepath.Join(resultDir, input.Timestamp+".json")
	mdPath := filepath.Join(resultDir, input.Timestamp+".md")

	actualJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json artifact: %v", err)
	}
	actualMD, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read markdown artifact: %v", err)
	}

	expectedJSON, err := os.ReadFile(filepath.Join(fixtureDir, "benchmark-report.json"))
	if err != nil {
		t.Fatalf("read json fixture: %v", err)
	}
	expectedMD, err := os.ReadFile(filepath.Join(fixtureDir, "benchmark-report.md"))
	if err != nil {
		t.Fatalf("read markdown fixture: %v", err)
	}

	if !bytes.Equal(actualJSON, expectedJSON) {
		t.Fatalf("json artifact mismatch\nactual:\n%s\nexpected:\n%s", string(actualJSON), string(expectedJSON))
	}
	if !bytes.Equal(actualMD, expectedMD) {
		t.Fatalf("markdown artifact mismatch\nactual:\n%s\nexpected:\n%s", string(actualMD), string(expectedMD))
	}
}

func TestWritePhase3MeasurementReport_CacheHitRatesAreSortedArray(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	baselineLog := filepath.Join(tmpDir, "baseline.jsonl")
	optimizedLog := filepath.Join(tmpDir, "optimized.jsonl")

	baselineContent := "" +
		"{\"iteration\":1,\"input_tokens\":100,\"cost_usd\":1.0,\"success\":true}\n"
	optimizedContent := "" +
		"{\"iteration\":1,\"input_tokens\":90,\"cost_usd\":0.9,\"success\":true,\"cache_class\":\"zebra_class\",\"cache_hit\":true}\n" +
		"{\"iteration\":2,\"input_tokens\":95,\"cost_usd\":0.95,\"success\":true,\"cache_class\":\"apple_class\",\"cache_hit\":true}\n" +
		"{\"iteration\":3,\"input_tokens\":88,\"cost_usd\":0.88,\"success\":true,\"cache_class\":\"monkey_class\",\"cache_hit\":true}\n"

	if err := os.WriteFile(baselineLog, []byte(baselineContent), 0o644); err != nil {
		t.Fatalf("write baseline log: %v", err)
	}
	if err := os.WriteFile(optimizedLog, []byte(optimizedContent), 0o644); err != nil {
		t.Fatalf("write optimized log: %v", err)
	}

	_, err := WritePhase3MeasurementReport(Phase3MeasurementInput{
		Timestamp:        "20260224T120000Z",
		BaselineLogPath:  baselineLog,
		OptimizedLogPath: optimizedLog,
	})
	if err != nil {
		t.Fatalf("WritePhase3MeasurementReport() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "reports", "phase3-measurement-20260224T120000Z.json")
	reportBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json artifact: %v", err)
	}

	var payload struct {
		CacheHitRatesByClass []struct {
			Class   string  `json:"class"`
			HitRate float64 `json:"hit_rate"`
		} `json:"cache_hit_rates_by_class"`
	}
	if err := json.Unmarshal(reportBytes, &payload); err != nil {
		t.Fatalf("unmarshal json artifact: %v", err)
	}

	if len(payload.CacheHitRatesByClass) != 3 {
		t.Fatalf("cache hit rate entries count = %d, want 3", len(payload.CacheHitRatesByClass))
	}
	if payload.CacheHitRatesByClass[0].Class != "apple_class" {
		t.Fatalf("first entry class = %q, want apple_class (sorted order)", payload.CacheHitRatesByClass[0].Class)
	}
	if payload.CacheHitRatesByClass[1].Class != "monkey_class" {
		t.Fatalf("second entry class = %q, want monkey_class (sorted order)", payload.CacheHitRatesByClass[1].Class)
	}
	if payload.CacheHitRatesByClass[2].Class != "zebra_class" {
		t.Fatalf("third entry class = %q, want zebra_class (sorted order)", payload.CacheHitRatesByClass[2].Class)
	}
}

func TestWritePhase3MeasurementReport_JSONArtifactIsDeterministic(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	baselineLog := filepath.Join(tmpDir, "baseline.jsonl")
	optimizedLog := filepath.Join(tmpDir, "optimized.jsonl")

	baselineContent := "" +
		"{\"iteration\":1,\"input_tokens\":100,\"cost_usd\":1.0,\"success\":true}\n" +
		"{\"iteration\":2,\"input_tokens\":120,\"cost_usd\":1.2,\"success\":true}\n"
	optimizedContent := "" +
		"{\"iteration\":1,\"input_tokens\":90,\"cost_usd\":0.9,\"success\":true,\"cache_class\":\"zebra_class\",\"cache_hit\":true}\n" +
		"{\"iteration\":2,\"input_tokens\":95,\"cost_usd\":0.95,\"success\":true,\"cache_class\":\"apple_class\",\"cache_hit\":true}\n" +
		"{\"iteration\":3,\"input_tokens\":88,\"cost_usd\":0.88,\"success\":true,\"cache_class\":\"monkey_class\",\"cache_hit\":false,\"cache_miss\":true}\n"

	if err := os.WriteFile(baselineLog, []byte(baselineContent), 0o644); err != nil {
		t.Fatalf("write baseline log: %v", err)
	}
	if err := os.WriteFile(optimizedLog, []byte(optimizedContent), 0o644); err != nil {
		t.Fatalf("write optimized log: %v", err)
	}

	_, err := WritePhase3MeasurementReport(Phase3MeasurementInput{
		Timestamp:        "20260224T120000Z",
		BaselineLogPath:  baselineLog,
		OptimizedLogPath: optimizedLog,
	})
	if err != nil {
		t.Fatalf("first WritePhase3MeasurementReport() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "reports", "phase3-measurement-20260224T120000Z.json")
	firstJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read first json artifact: %v", err)
	}

	_, err = WritePhase3MeasurementReport(Phase3MeasurementInput{
		Timestamp:        "20260224T120000Z",
		BaselineLogPath:  baselineLog,
		OptimizedLogPath: optimizedLog,
	})
	if err != nil {
		t.Fatalf("second WritePhase3MeasurementReport() error = %v", err)
	}

	secondJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read second json artifact: %v", err)
	}

	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("phase3 json artifact not deterministic\nfirst:\n%s\nsecond:\n%s", string(firstJSON), string(secondJSON))
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

	hitRateByClass := make(map[string]float64)
	for _, entry := range report.CacheHitRatesByClass {
		hitRateByClass[entry.Class] = entry.HitRate
	}

	if hitRateByClass["render_static_build"] != 0.5 {
		t.Fatalf("cache hit-rate render_static_build = %v, want 0.5", hitRateByClass["render_static_build"])
	}
	if hitRateByClass["utility_summarization"] != 1.0 {
		t.Fatalf("cache hit-rate utility_summarization = %v, want 1.0", hitRateByClass["utility_summarization"])
	}
}

func TestRunPhase3Measurement_RollbackTriggersAreSorted(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	baselineLog := filepath.Join(tmpDir, "baseline.jsonl")
	optimizedLog := filepath.Join(tmpDir, "optimized.jsonl")

	baselineContent := "" +
		"{\"iteration\":1,\"input_tokens\":100,\"cost_usd\":1.0,\"success\":true}\n" +
		"{\"iteration\":2,\"input_tokens\":120,\"cost_usd\":1.2,\"success\":true}\n" +
		"{\"iteration\":3,\"input_tokens\":110,\"cost_usd\":1.1,\"success\":true}\n"
	optimizedContent := "" +
		"{\"iteration\":1,\"input_tokens\":150,\"cost_usd\":1.5,\"success\":false}\n" +
		"{\"iteration\":2,\"input_tokens\":165,\"cost_usd\":1.65,\"success\":false}\n" +
		"{\"iteration\":3,\"input_tokens\":155,\"cost_usd\":1.55,\"success\":false}\n"

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
	if len(report.Rollback.Triggers) < 3 {
		t.Fatalf("rollback triggers count = %d, want 3 (all conditions triggered)", len(report.Rollback.Triggers))
	}

	// Verify all three triggers are present and sorted alphabetically
	expectedTriggers := []string{"median_cost_regression", "median_input_tokens_regression", "success_rate_regression"}
	if len(report.Rollback.Triggers) != len(expectedTriggers) {
		t.Fatalf("trigger count = %d, want %d", len(report.Rollback.Triggers), len(expectedTriggers))
	}
	for i, expected := range expectedTriggers {
		if report.Rollback.Triggers[i] != expected {
			t.Fatalf("trigger[%d] = %q, want %q (sorted)", i, report.Rollback.Triggers[i], expected)
		}
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
	// Should contain success_rate_regression when success drops significantly
	found := false
	for _, t := range report.Rollback.Triggers {
		if t == "success_rate_regression" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("success_rate_regression not in triggers: %v", report.Rollback.Triggers)
	}
}

func TestWritePhase3MeasurementReport_WritesReportAndRunArtifactsToReportsDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	baselineLog := filepath.Join(tmpDir, "baseline.jsonl")
	optimizedLog := filepath.Join(tmpDir, "optimized.jsonl")

	baselineContent := "" +
		"{\"iteration\":1,\"input_tokens\":100,\"cost_usd\":1.0,\"success\":true}\n" +
		"{\"iteration\":2,\"input_tokens\":120,\"cost_usd\":1.2,\"success\":true}\n" +
		"{\"iteration\":3,\"input_tokens\":110,\"cost_usd\":1.1,\"success\":true}\n"
	optimizedContent := "" +
		"{\"iteration\":1,\"input_tokens\":90,\"cost_usd\":0.9,\"success\":true,\"cache_class\":\"render_static_build\",\"cache_hit\":true}\n" +
		"{\"iteration\":2,\"input_tokens\":95,\"cost_usd\":0.95,\"success\":true,\"cache_class\":\"render_static_build\",\"cache_hit\":false,\"cache_miss\":true}\n" +
		"{\"iteration\":3,\"input_tokens\":92,\"cost_usd\":0.92,\"success\":true,\"cache_class\":\"utility_summarization\",\"cache_hit\":true}\n"

	if err := os.WriteFile(baselineLog, []byte(baselineContent), 0o644); err != nil {
		t.Fatalf("write baseline log: %v", err)
	}
	if err := os.WriteFile(optimizedLog, []byte(optimizedContent), 0o644); err != nil {
		t.Fatalf("write optimized log: %v", err)
	}

	paths, err := WritePhase3MeasurementReport(Phase3MeasurementInput{
		Timestamp:        "20260224T123000Z",
		BaselineLogPath:  baselineLog,
		OptimizedLogPath: optimizedLog,
	})
	if err != nil {
		t.Fatalf("WritePhase3MeasurementReport() error = %v", err)
	}

	if _, err := os.Stat(paths.JSONPath); err != nil {
		t.Fatalf("json report missing: %v", err)
	}
	if _, err := os.Stat(paths.MarkdownPath); err != nil {
		t.Fatalf("markdown report missing: %v", err)
	}
	if _, err := os.Stat(paths.BaselineArtifactPath); err != nil {
		t.Fatalf("baseline artifact missing: %v", err)
	}
	if _, err := os.Stat(paths.OptimizedArtifactPath); err != nil {
		t.Fatalf("optimized artifact missing: %v", err)
	}

	md, err := os.ReadFile(paths.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown report: %v", err)
	}

	content := string(md)
	for _, section := range []string{
		"## Median Comparison",
		"## Cache Hit Rates By Prompt Class",
		"## Kill-Switch Rollback Assessment",
	} {
		if !strings.Contains(content, section) {
			t.Fatalf("missing markdown section %q\n%s", section, content)
		}
	}
}

func TestBuildReportInput_ConstructsReportInputFromManifestAndModes(t *testing.T) {
	input := BuildReportInput(
		"test-manifest-id",
		"abc123def456",
		[]string{"bead-1", "bead-2", "bead-3"},
		"openai",
		"gpt-5",
		"gpt-5.1-codex-mini",
		"gpt-5.3-codex",
		"gpt-5.3-codex",
		[]ModeSummary{
			{
				Mode:             "single_pass",
				ElapsedSeconds:   120,
				TotalInput:       1000,
				TotalOutput:      500,
				TotalCostUSD:     1.25,
				CostQualityRatio: 1.42,
				Quality:          QualityMetrics{AverageScore: 0.88, FirstPassRate: 0.67},
			},
		},
		"20260225T120000Z",
	)

	if input.Timestamp != "20260225T120000Z" {
		t.Errorf("Timestamp = %q, want %q", input.Timestamp, "20260225T120000Z")
	}
	if input.Manifest.ID != "test-manifest-id" {
		t.Errorf("Manifest.ID = %q, want %q", input.Manifest.ID, "test-manifest-id")
	}
	if input.Manifest.BaseCommit != "abc123def456" {
		t.Errorf("Manifest.BaseCommit = %q, want %q", input.Manifest.BaseCommit, "abc123def456")
	}
	if len(input.Manifest.Beads) != 3 {
		t.Errorf("Manifest.Beads length = %d, want 3", len(input.Manifest.Beads))
	}
	if input.Manifest.Provider != "openai" {
		t.Errorf("Manifest.Provider = %q, want %q", input.Manifest.Provider, "openai")
	}
	if len(input.Modes) != 1 {
		t.Errorf("Modes length = %d, want 1", len(input.Modes))
	}
	if input.Modes[0].Mode != "single_pass" {
		t.Errorf("Modes[0].Mode = %q, want %q", input.Modes[0].Mode, "single_pass")
	}
}

func repoRootFrom(t *testing.T, start string) string {
	t.Helper()
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %q", start)
		}
		dir = parent
	}
}
