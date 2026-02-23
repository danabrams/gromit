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
	ActualTier   string  `json:"actual_tier"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

func AggregateModeMetrics(inputs []ModeLogInput) ([]ModeSummary, error) {
	summaries := make([]ModeSummary, 0, len(inputs))
	for _, input := range inputs {
		recs, err := readIterationRecords(input.LogPath)
		if err != nil {
			return nil, err
		}

		summary := ModeSummary{Mode: input.Mode}
		for _, rec := range recs {
			summary.TotalInput += rec.InputTokens
			summary.TotalOutput += rec.OutputTokens
			summary.TotalCostUSD = roundUSD(summary.TotalCostUSD + rec.CostUSD)
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
		if !input.RunStartedAt.IsZero() && !input.RunFinishedAt.IsZero() && input.RunFinishedAt.After(input.RunStartedAt) {
			summary.ElapsedSeconds = int(input.RunFinishedAt.Sub(input.RunStartedAt).Seconds())
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

func readIterationRecords(path string) ([]iterationMetricRecord, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read benchmark log %q: %w", path, err)
	}

	recs := []iterationMetricRecord{}
	scanner := bufio.NewScanner(bytes.NewReader(file))
	for scanner.Scan() {
		line := stdstrings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec iterationMetricRecord
		if err := stdjson.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("decode benchmark log line: %w", err)
		}
		recs = append(recs, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan benchmark log: %w", err)
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
