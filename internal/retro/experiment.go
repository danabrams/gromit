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
	AvgDurationMs   int64   `json:"avg_duration_ms"`
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
