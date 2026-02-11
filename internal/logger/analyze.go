package logger

import (
	"path/filepath"
)

// TimeoutAnalysis holds aggregate timeout statistics across all log files
type TimeoutAnalysis struct {
	TotalIterations int
	TotalTimeouts   int
	ByModel         map[string]ModelTimeoutStats
}

// ModelTimeoutStats holds per-model timeout statistics
type ModelTimeoutStats struct {
	TotalIterations       int
	TimeoutCount          int
	StallTimeouts         int
	BeadTimeouts          int
	InvocationTimeouts    int
	AvgTimeToFirstEventMs int64
	AvgToolCallCount      int
	RateLimitCorrelation  int
}

// updateTimeoutBreakdown increments the appropriate timeout counter based on type
func updateTimeoutBreakdown(stats *ModelTimeoutStats, timeoutType string) {
	switch timeoutType {
	case "stall":
		stats.StallTimeouts++
	case "bead":
		stats.BeadTimeouts++
	case "invocation":
		stats.InvocationTimeouts++
	}
}

// AnalyzeTimeouts reads all JSONL log files in the directory and aggregates timeout statistics
func AnalyzeTimeouts(logsDir string) (TimeoutAnalysis, error) {
	analysis := TimeoutAnalysis{
		ByModel: make(map[string]ModelTimeoutStats),
	}

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return analysis, err
	}

	// Track per-model running totals for computing averages
	type modelTotals struct {
		timeToFirstEventMs int64
		toolCallCount      int
	}
	totals := make(map[string]*modelTotals)

	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue // Skip unreadable files
		}

		for _, entry := range entries {
			analysis.TotalIterations++

			// Initialize model stats and totals if needed
			if _, exists := analysis.ByModel[entry.Model]; !exists {
				analysis.ByModel[entry.Model] = ModelTimeoutStats{}
				totals[entry.Model] = &modelTotals{}
			}

			stats := analysis.ByModel[entry.Model]
			stats.TotalIterations++

			// Accumulate values for averaging
			totals[entry.Model].timeToFirstEventMs += entry.TimeToFirstEventMs
			totals[entry.Model].toolCallCount += entry.ToolCallCount

			// Process timeout if present
			if entry.TimeoutType != "" {
				analysis.TotalTimeouts++
				stats.TimeoutCount++
				updateTimeoutBreakdown(&stats, entry.TimeoutType)

				// Correlation with rate limiting: count timeouts with rate_limit_hits > 0
				if entry.RateLimitHits > 0 {
					stats.RateLimitCorrelation++
				}
			}

			// Write back updated stats
			analysis.ByModel[entry.Model] = stats
		}
	}

	// Compute averages for each model
	for model, stats := range analysis.ByModel {
		if stats.TotalIterations > 0 {
			modelTotal := totals[model]
			stats.AvgTimeToFirstEventMs = modelTotal.timeToFirstEventMs / int64(stats.TotalIterations)
			stats.AvgToolCallCount = modelTotal.toolCallCount / stats.TotalIterations
		}
		analysis.ByModel[model] = stats
	}

	return analysis, nil
}
