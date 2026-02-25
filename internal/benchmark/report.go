package benchmark

import (
	stdjson "encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	stdstrings "strings"
)

var json = struct {
	Unmarshal func([]byte, any) error
}{
	Unmarshal: stdjson.Unmarshal,
}

var strings = struct {
	Contains func(string, string) bool
	Index    func(string, string) int
}{
	Contains: stdstrings.Contains,
	Index:    stdstrings.Index,
}

type ReportInput struct {
	Timestamp string
	Manifest  ManifestMetadata
	Modes     []ModeSummary
}

type ManifestMetadata struct {
	ID              string
	BaseCommit      string
	Beads           []string
	Provider        string
	ModelFamily     string
	LowTierModel    string
	MediumTierModel string
	HighTierModel   string
}

type ModeSummary struct {
	Mode             string           `json:"mode"`
	ElapsedSeconds   int              `json:"elapsed_seconds"`
	TotalInput       int              `json:"total_input_tokens"`
	TotalOutput      int              `json:"total_output_tokens"`
	TotalCostUSD     float64          `json:"total_cost_usd"`
	TierTotals       TierTotals       `json:"tier_totals"`
	ModelTotals      []ModelTotalsRow `json:"model_totals,omitempty"`
	Quality          QualityMetrics   `json:"quality"`
	CostQualityRatio float64          `json:"cost_quality_ratio"`
}

type TierTotals struct {
	Low    TierTotalsRow `json:"low"`
	Medium TierTotalsRow `json:"medium"`
	High   TierTotalsRow `json:"high"`
}

type TierTotalsRow struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type ModelTotalsRow struct {
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type QualityMetrics struct {
	AverageScore          float64 `json:"average_score"`
	FirstPassRate         float64 `json:"first_pass_rate"`
	ReviewFindings        int     `json:"review_findings"`
	ReviewFixesApplied    int     `json:"review_fixes_applied"`
	FinalValidationPassed bool    `json:"final_validation_passed"`
}

type WinnerHints struct {
	Fastest         string `json:"fastest"`
	Cheapest        string `json:"cheapest"`
	BestQuality     string `json:"best_quality"`
	BestCostQuality string `json:"best_cost_quality"`
}

type ReportPaths struct {
	JSONPath string
	MDPath   string
}

func WriteReport(input ReportInput) (ReportPaths, error) {
	resultDir := filepath.Join(".gromit", "benchmarks", "results", input.Manifest.ID)
	if err := os.MkdirAll(resultDir, 0o755); err != nil {
		return ReportPaths{}, fmt.Errorf("create results directory: %w", err)
	}

	modes := append([]ModeSummary(nil), input.Modes...)
	sort.Slice(modes, func(i, j int) bool {
		return modes[i].Mode < modes[j].Mode
	})
	for i := range modes {
		modes[i].ModelTotals = append([]ModelTotalsRow(nil), modes[i].ModelTotals...)
		sort.Slice(modes[i].ModelTotals, func(j, k int) bool {
			return modes[i].ModelTotals[j].Model < modes[i].ModelTotals[k].Model
		})
	}
	winners := computeWinnerHints(modes)

	jsonPath := filepath.Join(resultDir, input.Timestamp+".json")
	mdPath := filepath.Join(resultDir, input.Timestamp+".md")

	payload := struct {
		Manifest struct {
			ID              string   `json:"id"`
			BaseCommit      string   `json:"base_commit"`
			Beads           []string `json:"beads"`
			Provider        string   `json:"provider,omitempty"`
			ModelFamily     string   `json:"model_family,omitempty"`
			LowTierModel    string   `json:"low_tier_model,omitempty"`
			MediumTierModel string   `json:"medium_tier_model,omitempty"`
			HighTierModel   string   `json:"high_tier_model,omitempty"`
		} `json:"manifest"`
		Modes       []ModeSummary `json:"modes"`
		WinnerHints WinnerHints   `json:"winner_hints"`
	}{}
	payload.Manifest.ID = input.Manifest.ID
	payload.Manifest.BaseCommit = input.Manifest.BaseCommit
	payload.Manifest.Beads = append([]string(nil), input.Manifest.Beads...)
	payload.Manifest.Provider = input.Manifest.Provider
	payload.Manifest.ModelFamily = input.Manifest.ModelFamily
	payload.Manifest.LowTierModel = input.Manifest.LowTierModel
	payload.Manifest.MediumTierModel = input.Manifest.MediumTierModel
	payload.Manifest.HighTierModel = input.Manifest.HighTierModel
	payload.Modes = modes
	payload.WinnerHints = winners

	jsonBytes, err := stdjson.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ReportPaths{}, fmt.Errorf("marshal report json: %w", err)
	}
	jsonBytes = append(jsonBytes, '\n')
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		return ReportPaths{}, fmt.Errorf("write report json: %w", err)
	}

	md := stdstrings.Builder{}
	md.WriteString("# Benchmark Report\n\n")
	md.WriteString("Manifest: " + input.Manifest.ID + "\n")
	md.WriteString("\n## Per-Mode Summary\n\n")
	md.WriteString("| Mode | Elapsed Seconds | Total Input | Total Output | Total Cost USD |\n")
	md.WriteString("| --- | ---: | ---: | ---: | ---: |\n")
	for _, mode := range modes {
		md.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %.2f |\n", mode.Mode, mode.ElapsedSeconds, mode.TotalInput, mode.TotalOutput, mode.TotalCostUSD))
	}

	md.WriteString("\n## By-Tier Totals\n\n")
	md.WriteString("| Mode | Tier | Input Tokens | Output Tokens | Cost USD |\n")
	md.WriteString("| --- | --- | ---: | ---: | ---: |\n")
	for _, mode := range modes {
		for _, tier := range []struct {
			name string
			row  TierTotalsRow
		}{
			{name: "low", row: mode.TierTotals.Low},
			{name: "medium", row: mode.TierTotals.Medium},
			{name: "high", row: mode.TierTotals.High},
		} {
			md.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %.2f |\n", mode.Mode, tier.name, tier.row.InputTokens, tier.row.OutputTokens, tier.row.CostUSD))
		}
	}

	md.WriteString("\n## By-Model Totals\n\n")
	md.WriteString("| Mode | Model | Input Tokens | Output Tokens | Cost USD |\n")
	md.WriteString("| --- | --- | ---: | ---: | ---: |\n")
	for _, mode := range modes {
		for _, model := range mode.ModelTotals {
			md.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %.2f |\n", mode.Mode, model.Model, model.InputTokens, model.OutputTokens, model.CostUSD))
		}
	}

	md.WriteString("\n## Quality Metrics\n\n")
	md.WriteString("| Mode | Average Score | First Pass Rate | Review Findings | Review Fixes Applied | Final Validation Passed |\n")
	md.WriteString("| --- | ---: | ---: | ---: | ---: | --- |\n")
	for _, mode := range modes {
		md.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %d | %d | %t |\n", mode.Mode, mode.Quality.AverageScore, mode.Quality.FirstPassRate, mode.Quality.ReviewFindings, mode.Quality.ReviewFixesApplied, mode.Quality.FinalValidationPassed))
	}

	md.WriteString("\n## Winner Hints\n\n")
	md.WriteString(fmt.Sprintf("- fastest: %s\n", winners.Fastest))
	md.WriteString(fmt.Sprintf("- cheapest: %s\n", winners.Cheapest))
	md.WriteString(fmt.Sprintf("- best_quality: %s\n", winners.BestQuality))
	md.WriteString(fmt.Sprintf("- best_cost_quality: %s\n", winners.BestCostQuality))
	if err := os.WriteFile(mdPath, []byte(md.String()), 0o644); err != nil {
		return ReportPaths{}, fmt.Errorf("write report markdown: %w", err)
	}

	return ReportPaths{JSONPath: jsonPath, MDPath: mdPath}, nil
}

func computeWinnerHints(modes []ModeSummary) WinnerHints {
	if len(modes) == 0 {
		return WinnerHints{}
	}
	winners := WinnerHints{Fastest: modes[0].Mode, Cheapest: modes[0].Mode, BestQuality: modes[0].Mode, BestCostQuality: modes[0].Mode}
	bestElapsed := modes[0].ElapsedSeconds
	bestCost := modes[0].TotalCostUSD
	bestQuality := modes[0].Quality.AverageScore
	bestCostQuality := modes[0].CostQualityRatio

	for i := 1; i < len(modes); i++ {
		m := modes[i]
		if m.ElapsedSeconds < bestElapsed {
			bestElapsed = m.ElapsedSeconds
			winners.Fastest = m.Mode
		}
		if m.TotalCostUSD < bestCost {
			bestCost = m.TotalCostUSD
			winners.Cheapest = m.Mode
		}
		if m.Quality.AverageScore > bestQuality {
			bestQuality = m.Quality.AverageScore
			winners.BestQuality = m.Mode
		}
		if m.CostQualityRatio < bestCostQuality {
			bestCostQuality = m.CostQualityRatio
			winners.BestCostQuality = m.Mode
		}
	}
	return winners
}
