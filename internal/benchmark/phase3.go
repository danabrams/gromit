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
)

type Phase3MeasurementInput struct {
	Timestamp        string
	BaselineLogPath  string
	OptimizedLogPath string
}

type Phase3RunMetrics struct {
	MedianInputTokens int
	MedianCostUSD     float64
	MedianSuccessRate float64
}

type Phase3MeasurementReport struct {
	Baseline            Phase3RunMetrics
	Optimized           Phase3RunMetrics
	CacheHitRatesByClass map[string]float64
	Rollback            Phase3RollbackAssessment
}

type Phase3RollbackAssessment struct {
	KillSwitchRecommended bool
	Triggers              []string
}

type phase3IterationRecord struct {
	InputTokens int     `json:"input_tokens"`
	CostUSD     float64 `json:"cost_usd"`
	Success     bool    `json:"success"`
	CacheClass  string  `json:"cache_class"`
	CacheHit    bool    `json:"cache_hit"`
	CacheMiss   bool    `json:"cache_miss"`
}

func RunPhase3Measurement(input Phase3MeasurementInput) (Phase3MeasurementReport, error) {
	baselineRecords, err := readPhase3IterationRecords(input.BaselineLogPath)
	if err != nil {
		return Phase3MeasurementReport{}, err
	}
	optimizedRecords, err := readPhase3IterationRecords(input.OptimizedLogPath)
	if err != nil {
		return Phase3MeasurementReport{}, err
	}

	report := Phase3MeasurementReport{
		Baseline:             summarizePhase3Run(baselineRecords),
		Optimized:            summarizePhase3Run(optimizedRecords),
		CacheHitRatesByClass: computeCacheHitRatesByClass(optimizedRecords),
	}
	report.Rollback = evaluatePhase3Rollback(report.Baseline, report.Optimized)
	return report, nil
}

func evaluatePhase3Rollback(baseline, optimized Phase3RunMetrics) Phase3RollbackAssessment {
	triggers := make([]string, 0, 3)
	if baseline.MedianSuccessRate-optimized.MedianSuccessRate > 0.05 {
		triggers = append(triggers, "success_rate_regression")
	}
	if optimized.MedianInputTokens > baseline.MedianInputTokens {
		triggers = append(triggers, "median_input_tokens_regression")
	}
	if optimized.MedianCostUSD > baseline.MedianCostUSD {
		triggers = append(triggers, "median_cost_regression")
	}
	return Phase3RollbackAssessment{
		KillSwitchRecommended: len(triggers) > 0,
		Triggers:              triggers,
	}
}

func summarizePhase3Run(records []phase3IterationRecord) Phase3RunMetrics {
	if len(records) == 0 {
		return Phase3RunMetrics{}
	}

	inputs := make([]int, 0, len(records))
	costs := make([]float64, 0, len(records))
	successCount := 0
	for _, rec := range records {
		inputs = append(inputs, rec.InputTokens)
		costs = append(costs, rec.CostUSD)
		if rec.Success {
			successCount++
		}
	}

	return Phase3RunMetrics{
		MedianInputTokens: medianInt(inputs),
		MedianCostUSD:     medianFloat(costs),
		MedianSuccessRate: round2(float64(successCount) / float64(len(records))),
	}
}

func medianInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	cp := append([]int(nil), values...)
	sort.Ints(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return int(math.Round(float64(cp[mid-1]+cp[mid]) / 2.0))
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return round2(cp[mid])
	}
	return round2((cp[mid-1] + cp[mid]) / 2.0)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func computeCacheHitRatesByClass(records []phase3IterationRecord) map[string]float64 {
	type bucket struct {
		hits  int
		total int
	}
	buckets := map[string]bucket{}
	for _, rec := range records {
		class := stdstrings.TrimSpace(rec.CacheClass)
		if class == "" {
			continue
		}
		row := buckets[class]
		if rec.CacheHit || rec.CacheMiss {
			row.total++
		}
		if rec.CacheHit {
			row.hits++
		}
		buckets[class] = row
	}

	out := make(map[string]float64, len(buckets))
	for class, row := range buckets {
		if row.total == 0 {
			out[class] = 0
			continue
		}
		out[class] = round2(float64(row.hits) / float64(row.total))
	}
	return out
}

func readPhase3IterationRecords(path string) ([]phase3IterationRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read phase-3 log %q: %w", path, err)
	}
	records := []phase3IterationRecord{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := stdstrings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec phase3IterationRecord
		if err := stdjson.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("decode phase-3 log line: %w", err)
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan phase-3 log: %w", err)
	}
	return records, nil
}
