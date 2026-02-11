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

// AnalyzeTimeouts reads all JSONL log files in the directory and aggregates timeout statistics
func AnalyzeTimeouts(logsDir string) (TimeoutAnalysis, error) {
	analysis := TimeoutAnalysis{
		ByModel: make(map[string]ModelTimeoutStats),
	}

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return analysis, err
	}

	// Track per-model aggregates for computing averages
	type modelAggregates struct {
		totalTimeToFirstEventMs int64
		totalToolCallCount      int
	}
	aggregates := make(map[string]*modelAggregates)

	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue // Skip unreadable files
		}

		for _, entry := range entries {
			// Count all iterations
			analysis.TotalIterations++

			// Initialize model stats if needed
			if _, exists := analysis.ByModel[entry.Model]; !exists {
				analysis.ByModel[entry.Model] = ModelTimeoutStats{}
				aggregates[entry.Model] = &modelAggregates{}
			}

			stats := analysis.ByModel[entry.Model]
			agg := aggregates[entry.Model]

			// Count total iterations for this model
			stats.TotalIterations++

			// Accumulate values for averages
			agg.totalTimeToFirstEventMs += entry.TimeToFirstEventMs
			agg.totalToolCallCount += entry.ToolCallCount

			// Detect timeouts
			isTimeout := entry.TimeoutType != ""
			if isTimeout {
				analysis.TotalTimeouts++
				stats.TimeoutCount++

				// Breakdown by timeout type
				switch entry.TimeoutType {
				case "stall":
					stats.StallTimeouts++
				case "bead":
					stats.BeadTimeouts++
				case "invocation":
					stats.InvocationTimeouts++
				}

				// Rate limit correlation: count timeouts with rate_limit_hits > 0
				if entry.RateLimitHits > 0 {
					stats.RateLimitCorrelation++
				}
			}

			// Update the map with modified stats
			analysis.ByModel[entry.Model] = stats
		}
	}

	// Compute averages for each model
	for model, stats := range analysis.ByModel {
		agg := aggregates[model]
		if stats.TotalIterations > 0 {
			stats.AvgTimeToFirstEventMs = agg.totalTimeToFirstEventMs / int64(stats.TotalIterations)
			stats.AvgToolCallCount = agg.totalToolCallCount / stats.TotalIterations
		}
		analysis.ByModel[model] = stats
	}

	return analysis, nil
}
