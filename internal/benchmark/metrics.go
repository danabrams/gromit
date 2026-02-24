package benchmark

import (
	"bufio"
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	stdstrings "strings"
	"time"
)

type ModeLogInput struct {
	Mode          string
	RunStartedAt  time.Time
	RunFinishedAt time.Time
	LogPath       string
}

type iterationMetricRecord struct {
	Model        string  `json:"model"`
	ActualTier   string  `json:"actual_tier"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	QualityScore float64 `json:"quality_score"`
	FirstPass    bool    `json:"first_pass_success"`
	Validated    bool    `json:"validated"`
}

type reviewMetricRecord struct {
	Type           string `json:"type"`
	FixesApplied   int    `json:"fixes_applied"`
	BeadsCreated   int    `json:"beads_created"`
	BacklogCreated int    `json:"backlog_created"`
}

type modeMetricRecords struct {
	Iterations []iterationMetricRecord
	Reviews    []reviewMetricRecord
}

func AggregateModeMetrics(inputs []ModeLogInput) ([]ModeSummary, error) {
	summaries := make([]ModeSummary, 0, len(inputs))
	for _, input := range inputs {
		recs, err := readModeMetricRecords(input.LogPath)
		if err != nil {
			return nil, err
		}

		summary := ModeSummary{Mode: input.Mode}
		modelTotals := make(map[string]TierTotalsRow)
		for _, rec := range recs.Iterations {
			summary.TotalInput += rec.InputTokens
			summary.TotalOutput += rec.OutputTokens
			summary.TotalCostUSD = roundUSD(summary.TotalCostUSD + rec.CostUSD)
			if model := stdstrings.TrimSpace(rec.Model); model != "" {
				row := modelTotals[model]
				row.InputTokens += rec.InputTokens
				row.OutputTokens += rec.OutputTokens
				row.CostUSD = roundUSD(row.CostUSD + rec.CostUSD)
				modelTotals[model] = row
			}
			switch normalizeTier(rec.ActualTier) {
			case "low":
				summary.TierTotals.Low.InputTokens += rec.InputTokens
				summary.TierTotals.Low.OutputTokens += rec.OutputTokens
				summary.TierTotals.Low.CostUSD = roundUSD(summary.TierTotals.Low.CostUSD + rec.CostUSD)
			case "high":
				summary.TierTotals.High.InputTokens += rec.InputTokens
				summary.TierTotals.High.OutputTokens += rec.OutputTokens
				summary.TierTotals.High.CostUSD = roundUSD(summary.TierTotals.High.CostUSD + rec.CostUSD)
			default:
				summary.TierTotals.Medium.InputTokens += rec.InputTokens
				summary.TierTotals.Medium.OutputTokens += rec.OutputTokens
				summary.TierTotals.Medium.CostUSD = roundUSD(summary.TierTotals.Medium.CostUSD + rec.CostUSD)
			}
		}
		if len(recs.Iterations) > 0 {
			qualityTotal := 0.0
			firstPassCount := 0
			for _, rec := range recs.Iterations {
				qualityTotal += rec.QualityScore
				if rec.FirstPass {
					firstPassCount++
				}
			}
			summary.Quality.AverageScore = roundUSD(qualityTotal / float64(len(recs.Iterations)))
			summary.Quality.FirstPassRate = roundUSD(float64(firstPassCount) / float64(len(recs.Iterations)))
			summary.Quality.FinalValidationPassed = recs.Iterations[len(recs.Iterations)-1].Validated
		}
		if summary.Quality.AverageScore > 0 {
			summary.CostQualityRatio = roundUSD(summary.TotalCostUSD / summary.Quality.AverageScore)
		} else {
			summary.CostQualityRatio = summary.TotalCostUSD
		}
		for _, rec := range recs.Reviews {
			summary.Quality.ReviewFixesApplied += rec.FixesApplied
			summary.Quality.ReviewFindings += rec.BeadsCreated + rec.BacklogCreated
		}
		if !input.RunStartedAt.IsZero() && !input.RunFinishedAt.IsZero() && input.RunFinishedAt.After(input.RunStartedAt) {
			summary.ElapsedSeconds = int(input.RunFinishedAt.Sub(input.RunStartedAt).Seconds())
		}
		if len(modelTotals) > 0 {
			models := make([]string, 0, len(modelTotals))
			for model := range modelTotals {
				models = append(models, model)
			}
			sort.Strings(models)
			summary.ModelTotals = make([]ModelTotalsRow, 0, len(models))
			for _, model := range models {
				row := modelTotals[model]
				summary.ModelTotals = append(summary.ModelTotals, ModelTotalsRow{
					Model:        model,
					InputTokens:  row.InputTokens,
					OutputTokens: row.OutputTokens,
					CostUSD:      row.CostUSD,
				})
			}
		}
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Mode < summaries[j].Mode
	})
	return summaries, nil
}

func roundUSD(v float64) float64 {
	return math.Round(v*1_000_000) / 1_000_000
}

func readModeMetricRecords(path string) (modeMetricRecords, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return modeMetricRecords{}, fmt.Errorf("read benchmark log %q: %w", path, err)
	}

	recs := modeMetricRecords{Iterations: []iterationMetricRecord{}, Reviews: []reviewMetricRecord{}}
	scanner := bufio.NewScanner(bytes.NewReader(file))
	for scanner.Scan() {
		line := stdstrings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var typed struct {
			Type string `json:"type"`
		}
		if err := stdjson.Unmarshal([]byte(line), &typed); err != nil {
			return modeMetricRecords{}, fmt.Errorf("decode benchmark log line: %w", err)
		}
		if typed.Type == "review" {
			var rec reviewMetricRecord
			if err := stdjson.Unmarshal([]byte(line), &rec); err != nil {
				return modeMetricRecords{}, fmt.Errorf("decode benchmark review line: %w", err)
			}
			recs.Reviews = append(recs.Reviews, rec)
			continue
		}
		var rec iterationMetricRecord
		if err := stdjson.Unmarshal([]byte(line), &rec); err != nil {
			return modeMetricRecords{}, fmt.Errorf("decode benchmark iteration line: %w", err)
		}
		recs.Iterations = append(recs.Iterations, rec)
	}
	if err := scanner.Err(); err != nil {
		return modeMetricRecords{}, fmt.Errorf("scan benchmark log: %w", err)
	}
	return recs, nil
}

func normalizeTier(tier string) string {
	switch stdstrings.ToLower(stdstrings.TrimSpace(tier)) {
	case "low", "medium", "high":
		return stdstrings.ToLower(stdstrings.TrimSpace(tier))
	default:
		return "medium"
	}
}
