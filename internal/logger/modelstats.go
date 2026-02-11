package logger

import (
	"fmt"
	"path/filepath"
)

// ModelStats represents aggregated statistics for a specific model
type ModelStats struct {
	Model           string
	Iterations      int
	Successes       int
	Failures        int
	EscalationsTo   int
	EscalationsFrom int
	TotalCostUSD    float64
}

// SuccessRate returns the success rate as a float64 (0.0-1.0)
func (s ModelStats) SuccessRate() float64 {
	if s.Iterations == 0 {
		return 0
	}
	return float64(s.Successes) / float64(s.Iterations)
}

// ReadModelStats aggregates per-model statistics from all JSONL logs
func ReadModelStats(logsDir string) (map[string]ModelStats, error) {
	modelMap := make(map[string]ModelStats)

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return modelMap, fmt.Errorf("globbing log files: %w", err)
	}

	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue // Skip unreadable files
		}

		for _, entry := range entries {
			stats := modelMap[entry.Model]

			// Initialize with model name if first time
			if stats.Model == "" {
				stats.Model = entry.Model
			}

			stats.Iterations++
			if entry.Success {
				stats.Successes++
			} else {
				stats.Failures++
			}

			stats.TotalCostUSD += entry.CostUSD

			// Track escalations
			if entry.Escalated {
				stats.EscalationsFrom++
			}
			if entry.EscalatedTo == entry.Model {
				stats.EscalationsTo++
			}

			modelMap[entry.Model] = stats
		}

		// Track escalation "to" counts separately
		for _, entry := range entries {
			if entry.Escalated && entry.EscalatedTo != "" {
				targetStats := modelMap[entry.EscalatedTo]
				if targetStats.Model == "" {
					targetStats.Model = entry.EscalatedTo
				}
				targetStats.EscalationsTo++
				modelMap[entry.EscalatedTo] = targetStats
			}
		}
	}

	return modelMap, nil
}

// ReadRunModelStats aggregates per-model statistics filtered by run ID
func ReadRunModelStats(logsDir string, runID string) (map[string]ModelStats, error) {
	return nil, nil
}

// CostPerCompletedBead computes the total cost per completed bead
func CostPerCompletedBead(logsDir string) (map[string]float64, error) {
	return nil, nil
}
