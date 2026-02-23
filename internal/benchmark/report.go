package benchmark

import (
	stdjson "encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var json = struct {
	Unmarshal func([]byte, any) error
}{
	Unmarshal: stdjson.Unmarshal,
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
	Mode             string         `json:"mode"`
	ElapsedSeconds   int            `json:"elapsed_seconds"`
	TotalInput       int            `json:"total_input_tokens"`
	TotalOutput      int            `json:"total_output_tokens"`
	TotalCostUSD     float64        `json:"total_cost_usd"`
	TierTotals       TierTotals     `json:"tier_totals"`
	Quality          QualityMetrics `json:"quality"`
	CostQualityRatio float64        `json:"cost_quality_ratio"`
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

	md := strings.Builder{}
	md.WriteString("# Benchmark Report\n\n")
	md.WriteString("Manifest: " + input.Manifest.ID + "\n")
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
		if m.CostQualityRatio > bestCostQuality {
			bestCostQuality = m.CostQualityRatio
			winners.BestCostQuality = m.Mode
		}
	}
	return winners
}
