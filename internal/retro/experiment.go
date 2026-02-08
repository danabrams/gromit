package retro

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// BaselineMetrics holds efficiency metrics captured at the start of an experiment
type BaselineMetrics struct {
	AvgCostPerBead  float64 `json:"avg_cost_per_bead"`
	AvgDurationMs   float64 `json:"avg_duration_ms"`
	AvgInputTokens  float64 `json:"avg_input_tokens"`
	AvgOutputTokens float64 `json:"avg_output_tokens"`
	FailureRate     float64 `json:"failure_rate"`
}

// Experiment represents an active process improvement experiment
type Experiment struct {
	Name            string          `json:"name"`
	Hypothesis      string          `json:"hypothesis"`
	Change          string          `json:"change"`
	Measurement     string          `json:"measurement"`
	Risk            string          `json:"risk"`
	StartedAt       time.Time       `json:"started_at"`
	BaselineMetrics BaselineMetrics `json:"baseline_metrics"`
}

// LoadExperiment reads the experiment from the given file path.
// Returns nil when the file does not exist.
func LoadExperiment(path string) (*Experiment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading experiment file: %w", err)
	}

	var exp Experiment
	if err := json.Unmarshal(data, &exp); err != nil {
		return nil, fmt.Errorf("parsing experiment file: %w", err)
	}

	return &exp, nil
}

// SaveExperiment writes the experiment to the given file path.
func SaveExperiment(path string, exp *Experiment) error {
	if exp == nil {
		return fmt.Errorf("experiment is nil")
	}

	data, err := json.MarshalIndent(exp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling experiment: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing experiment file: %w", err)
	}

	return nil
}

// DeleteExperiment removes the experiment file at the given path.
// Does not return an error if the file does not exist.
func DeleteExperiment(path string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("deleting experiment file: %w", err)
	}
	return nil
}

// ModelEfficiencyData represents aggregated efficiency metrics for a model.
// This is a subset of logger.ModelEfficiency used for baseline computation.
type ModelEfficiencyData struct {
	TotalCostUSD      float64
	TotalDuration     time.Duration
	TotalInputTokens  int
	TotalOutputTokens int
	IterationCount    int
}

// EfficiencyData contains the per-model efficiency data needed for baseline computation.
type EfficiencyData struct {
	Models map[string]ModelEfficiencyData
}

// StatsData contains the stats needed for failure rate computation.
type StatsData interface {
	FailureRate() float64
}

// ComputeBaselineMetrics aggregates efficiency metrics from all JSONL log files
// in the specified logs directory. This provides a baseline snapshot for experiment
// comparison.
//
// The readStats function should return aggregate run statistics.
// The readEfficiency function should return efficiency data for all runs (pass empty currentRunID).
func ComputeBaselineMetrics(
	logsDir string,
	readStats func(string) (StatsData, error),
	readEfficiency func(string, string) (EfficiencyData, error),
) (BaselineMetrics, error) {
	// Read overall run stats for failure rate
	stats, err := readStats(logsDir)
	if err != nil {
		return BaselineMetrics{}, fmt.Errorf("reading run stats: %w", err)
	}

	// Read efficiency data treating all runs as historical (empty currentRunID)
	efficiency, err := readEfficiency(logsDir, "")
	if err != nil {
		return BaselineMetrics{}, fmt.Errorf("reading efficiency data: %w", err)
	}

	// Compute averages from all models
	avgCost := computeOverallAvgCost(efficiency.Models)
	avgDuration := computeOverallAvgDuration(efficiency.Models)
	avgInput := computeOverallAvgTokens(efficiency.Models, true)
	avgOutput := computeOverallAvgTokens(efficiency.Models, false)

	return BaselineMetrics{
		AvgCostPerBead:  avgCost,
		AvgDurationMs:   float64(avgDuration.Milliseconds()),
		AvgInputTokens:  avgInput,
		AvgOutputTokens: avgOutput,
		FailureRate:     stats.FailureRate(),
	}, nil
}

// computeOverallAvgCost computes weighted average cost across all models
func computeOverallAvgCost(models map[string]ModelEfficiencyData) float64 {
	if len(models) == 0 {
		return 0
	}
	totalCost := 0.0
	totalCount := 0
	for _, m := range models {
		totalCost += m.TotalCostUSD
		totalCount += m.IterationCount
	}
	if totalCount == 0 {
		return 0
	}
	return totalCost / float64(totalCount)
}

// computeOverallAvgDuration computes weighted average duration across all models
func computeOverallAvgDuration(models map[string]ModelEfficiencyData) time.Duration {
	if len(models) == 0 {
		return 0
	}
	totalDuration := time.Duration(0)
	totalCount := 0
	for _, m := range models {
		totalDuration += m.TotalDuration
		totalCount += m.IterationCount
	}
	if totalCount == 0 {
		return 0
	}
	return totalDuration / time.Duration(totalCount)
}

// computeOverallAvgTokens computes weighted average token count across all models
func computeOverallAvgTokens(models map[string]ModelEfficiencyData, input bool) float64 {
	if len(models) == 0 {
		return 0
	}
	totalTokens := 0
	totalCount := 0
	for _, m := range models {
		if input {
			totalTokens += m.TotalInputTokens
		} else {
			totalTokens += m.TotalOutputTokens
		}
		totalCount += m.IterationCount
	}
	if totalCount == 0 {
		return 0
	}
	return float64(totalTokens) / float64(totalCount)
}
