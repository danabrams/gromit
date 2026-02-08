package logger

import "time"

// IterationEfficiency represents efficiency data for a single iteration
type IterationEfficiency struct {
	BeadID            string
	Model             string
	Duration          time.Duration
	CostUSD           float64
	InputTokens       int
	OutputTokens      int
	ContextWindowUsed float64 // Percentage of model's context window used by input tokens
	ExceededThreshold bool    // True if input tokens exceeded 80% of context window
}

// ModelEfficiency represents aggregated efficiency metrics for a specific model
type ModelEfficiency struct {
	Model             string
	IterationCount    int
	AvgCostUSD        float64
	AvgDuration       time.Duration
	AvgInputTokens    float64
	AvgOutputTokens   float64
	TotalCostUSD      float64
	TotalDuration     time.Duration
	TotalInputTokens  int
	TotalOutputTokens int
}

// EfficiencyReport holds efficiency data for current run and historical comparison
type EfficiencyReport struct {
	// Current run data
	CurrentIterations []IterationEfficiency
	CurrentModels     map[string]ModelEfficiency

	// Historical data (all previous runs)
	HistoricalModels map[string]ModelEfficiency

	// Overall aggregates
	CurrentAvgCostPerBead        float64
	CurrentAvgDurationPerBead    time.Duration
	HistoricalAvgCostPerBead     float64
	HistoricalAvgDurationPerBead time.Duration

	// Deltas
	CostDelta     float64       // Positive means current run is more expensive
	DurationDelta time.Duration // Positive means current run is slower

	// Context window flags
	HighContextIterations []IterationEfficiency // Iterations exceeding 80% threshold
}
