package benchmark

import (
	"bufio"
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	CacheHitRatesByClass []CacheHitRateEntry
	Rollback            Phase3RollbackAssessment
}

type CacheHitRateEntry struct {
	Class   string  `json:"class"`
	HitRate float64 `json:"hit_rate"`
}

type Phase3RollbackAssessment struct {
	KillSwitchRecommended bool
	Triggers              []string
}

type Phase3ReportPaths struct {
	JSONPath            string
	MarkdownPath        string
	BaselineArtifactPath string
	OptimizedArtifactPath string
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

func WritePhase3MeasurementReport(input Phase3MeasurementInput) (Phase3ReportPaths, error) {
	report, err := RunPhase3Measurement(input)
	if err != nil {
		return Phase3ReportPaths{}, err
	}

	ts := stdstrings.TrimSpace(input.Timestamp)
	if ts == "" {
		return Phase3ReportPaths{}, fmt.Errorf("timestamp is required")
	}

	reportsDir := filepath.Join(".gromit", "reports")
	artifactsDir := filepath.Join(reportsDir, "phase3-measurement-"+ts)
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return Phase3ReportPaths{}, fmt.Errorf("create phase-3 artifacts dir: %w", err)
	}

	baselineBytes, err := os.ReadFile(input.BaselineLogPath)
	if err != nil {
		return Phase3ReportPaths{}, fmt.Errorf("read baseline log for artifact copy: %w", err)
	}
	optimizedBytes, err := os.ReadFile(input.OptimizedLogPath)
	if err != nil {
		return Phase3ReportPaths{}, fmt.Errorf("read optimized log for artifact copy: %w", err)
	}

	baselineArtifactPath := filepath.Join(artifactsDir, "baseline.jsonl")
	optimizedArtifactPath := filepath.Join(artifactsDir, "optimized.jsonl")
	if err := os.WriteFile(baselineArtifactPath, baselineBytes, 0o644); err != nil {
		return Phase3ReportPaths{}, fmt.Errorf("write baseline artifact copy: %w", err)
	}
	if err := os.WriteFile(optimizedArtifactPath, optimizedBytes, 0o644); err != nil {
		return Phase3ReportPaths{}, fmt.Errorf("write optimized artifact copy: %w", err)
	}

	jsonPath := filepath.Join(reportsDir, "phase3-measurement-"+ts+".json")
	payload := struct {
		Timestamp           string                      `json:"timestamp"`
		Baseline            Phase3RunMetrics           `json:"baseline"`
		Optimized           Phase3RunMetrics           `json:"optimized"`
		CacheHitRatesByClass []CacheHitRateEntry       `json:"cache_hit_rates_by_class"`
		Rollback            Phase3RollbackAssessment    `json:"rollback"`
		BaselineArtifactPath string                     `json:"baseline_artifact_path"`
		OptimizedArtifactPath string                    `json:"optimized_artifact_path"`
	}{
		Timestamp:            ts,
		Baseline:             report.Baseline,
		Optimized:            report.Optimized,
		CacheHitRatesByClass: report.CacheHitRatesByClass,
		Rollback:             report.Rollback,
		BaselineArtifactPath: baselineArtifactPath,
		OptimizedArtifactPath: optimizedArtifactPath,
	}
	jsonBytes, err := stdjson.MarshalIndent(payload, "", "  ")
	if err != nil {
		return Phase3ReportPaths{}, fmt.Errorf("marshal phase-3 report json: %w", err)
	}
	jsonBytes = append(jsonBytes, '\n')
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		return Phase3ReportPaths{}, fmt.Errorf("write phase-3 report json: %w", err)
	}

	mdPath := filepath.Join(reportsDir, "phase3-measurement-"+ts+".md")
	builder := stdstrings.Builder{}
	builder.WriteString("# Phase-3 Measurement Report\n\n")
	builder.WriteString("## Median Comparison\n\n")
	builder.WriteString("| Metric | Baseline | Optimized |\n")
	builder.WriteString("| --- | ---: | ---: |\n")
	builder.WriteString(fmt.Sprintf("| median_input_tokens | %d | %d |\n", report.Baseline.MedianInputTokens, report.Optimized.MedianInputTokens))
	builder.WriteString(fmt.Sprintf("| median_cost_usd | %.2f | %.2f |\n", report.Baseline.MedianCostUSD, report.Optimized.MedianCostUSD))
	builder.WriteString(fmt.Sprintf("| median_success_rate | %.2f | %.2f |\n", report.Baseline.MedianSuccessRate, report.Optimized.MedianSuccessRate))

	builder.WriteString("\n## Cache Hit Rates By Prompt Class\n\n")
	builder.WriteString("| Prompt Class | Hit Rate |\n")
	builder.WriteString("| --- | ---: |\n")
	for _, entry := range report.CacheHitRatesByClass {
		builder.WriteString(fmt.Sprintf("| %s | %.2f |\n", entry.Class, entry.HitRate))
	}

	builder.WriteString("\n## Kill-Switch Rollback Assessment\n\n")
	builder.WriteString(fmt.Sprintf("- kill_switch_recommended: %t\n", report.Rollback.KillSwitchRecommended))
	if len(report.Rollback.Triggers) == 0 {
		builder.WriteString("- triggers: none\n")
	} else {
		builder.WriteString("- triggers: " + stdstrings.Join(report.Rollback.Triggers, ", ") + "\n")
	}
	if err := os.WriteFile(mdPath, []byte(builder.String()), 0o644); err != nil {
		return Phase3ReportPaths{}, fmt.Errorf("write phase-3 report markdown: %w", err)
	}

	return Phase3ReportPaths{
		JSONPath:             jsonPath,
		MarkdownPath:         mdPath,
		BaselineArtifactPath: baselineArtifactPath,
		OptimizedArtifactPath: optimizedArtifactPath,
	}, nil
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
	sort.Strings(triggers)
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

func computeCacheHitRatesByClass(records []phase3IterationRecord) []CacheHitRateEntry {
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

	entries := make([]CacheHitRateEntry, 0, len(buckets))
	for class, row := range buckets {
		hitRate := 0.0
		if row.total > 0 {
			hitRate = round2(float64(row.hits) / float64(row.total))
		}
		entries = append(entries, CacheHitRateEntry{Class: class, HitRate: hitRate})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Class < entries[j].Class
	})
	return entries
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
