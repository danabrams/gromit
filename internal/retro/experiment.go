package retro

import "time"

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
