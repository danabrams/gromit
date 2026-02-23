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
				Mode:            "single_pass",
				ElapsedSeconds:  120,
				TotalInput:      1000,
				TotalOutput:     500,
				TotalCostUSD:    1.25,
				TierTotals:      TierTotals{Low: TierTotalsRow{InputTokens: 600, OutputTokens: 300, CostUSD: 0.5}},
				Quality:         QualityMetrics{AverageScore: 0.88, FirstPassRate: 0.67, ReviewFindings: 3, ReviewFixesApplied: 2, FinalValidationPassed: true},
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
