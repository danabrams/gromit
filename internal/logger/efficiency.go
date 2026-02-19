package logger

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

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
	FilesTouched      int
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
	CurrentIterations       []IterationEfficiency
	CurrentModels           map[string]ModelEfficiency
	CurrentProviderFamilies map[string]ModelEfficiency

	// Historical data (all previous runs)
	HistoricalModels           map[string]ModelEfficiency
	HistoricalProviderFamilies map[string]ModelEfficiency

	// Overall aggregates
	CurrentAvgCostPerBead        float64
	CurrentAvgDurationPerBead    time.Duration
	HistoricalAvgCostPerBead     float64
	HistoricalAvgDurationPerBead time.Duration

	// Provider family state
	MixedProviderFamilies bool

	// Deltas
	CostDelta     float64       // Positive means current run is more expensive
	DurationDelta time.Duration // Positive means current run is slower

	// Context window flags
	HighContextIterations []IterationEfficiency // Iterations exceeding 80% threshold
}

// Model context window sizes (input token limits)
var modelContextWindows = map[string]int{
	"opus":   200000,
	"sonnet": 200000,
	"haiku":  200000,
}

// ReadEfficiencyReport reads JSONL log files and computes efficiency aggregates.
// currentRunID specifies which run is "current" (all others are historical).
// If currentRunID is empty, all runs are treated as historical.
func ReadEfficiencyReport(logsDir string, currentRunID string) (*EfficiencyReport, error) {
	return ReadEfficiencyReportFiltered(logsDir, currentRunID, nil)
}

// ReadEfficiencyReportFiltered reads JSONL log files and computes efficiency aggregates,
// optionally filtered by bead ID. If beadFilter is nil or empty, all entries are included.
// currentRunID specifies which run is "current" (all others are historical).
func ReadEfficiencyReportFiltered(logsDir string, currentRunID string, beadFilter map[string]bool) (*EfficiencyReport, error) {
	report := &EfficiencyReport{
		CurrentModels:              make(map[string]ModelEfficiency),
		HistoricalModels:           make(map[string]ModelEfficiency),
		CurrentProviderFamilies:    make(map[string]ModelEfficiency),
		HistoricalProviderFamilies: make(map[string]ModelEfficiency),
	}

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	// Track per-model totals for both current and historical
	currentModelTotals := make(map[string]*modelAccumulator)
	historicalModelTotals := make(map[string]*modelAccumulator)
	currentFamilyTotals := make(map[string]*modelAccumulator)
	historicalFamilyTotals := make(map[string]*modelAccumulator)

	for _, f := range files {
		runID := extractRunID(f)
		isCurrent := (runID == currentRunID)

		entries, err := readLogFile(f)
		if err != nil {
			continue // Skip unreadable files
		}

		for _, entry := range entries {
			// Apply filter if provided
			if len(beadFilter) > 0 && !beadFilter[entry.BeadID] {
				continue
			}

			// Build IterationEfficiency
			ie := IterationEfficiency{
				BeadID:       entry.BeadID,
				Model:        entry.Model,
				Duration:     time.Duration(entry.DurationMs) * time.Millisecond,
				CostUSD:      entry.CostUSD,
				InputTokens:  entry.InputTokens,
				OutputTokens: entry.OutputTokens,
				FilesTouched: entry.FilesTouched,
			}

			// Calculate context window usage
			if contextWindow, ok := modelContextWindows[entry.Model]; ok && contextWindow > 0 {
				ie.ContextWindowUsed = float64(entry.InputTokens) / float64(contextWindow)
				ie.ExceededThreshold = ie.ContextWindowUsed >= 0.8
			}

			if isCurrent {
				report.CurrentIterations = append(report.CurrentIterations, ie)
				if ie.ExceededThreshold {
					report.HighContextIterations = append(report.HighContextIterations, ie)
				}

				// Accumulate per-model stats
				if currentModelTotals[entry.Model] == nil {
					currentModelTotals[entry.Model] = &modelAccumulator{}
				}
				currentModelTotals[entry.Model].add(ie)

				// Accumulate per-provider-family stats
				if family := providerFamilyForModel(entry.Model); family != "" {
					if currentFamilyTotals[family] == nil {
						currentFamilyTotals[family] = &modelAccumulator{}
					}
					currentFamilyTotals[family].add(ie)
				}
			} else {
				// Historical
				if historicalModelTotals[entry.Model] == nil {
					historicalModelTotals[entry.Model] = &modelAccumulator{}
				}
				historicalModelTotals[entry.Model].add(ie)

				if family := providerFamilyForModel(entry.Model); family != "" {
					if historicalFamilyTotals[family] == nil {
						historicalFamilyTotals[family] = &modelAccumulator{}
					}
					historicalFamilyTotals[family].add(ie)
				}
			}
		}
	}

	// Compute per-model aggregates for current run
	for model, acc := range currentModelTotals {
		report.CurrentModels[model] = acc.toModelEfficiency(model)
	}

	// Compute per-model aggregates for historical runs
	for model, acc := range historicalModelTotals {
		report.HistoricalModels[model] = acc.toModelEfficiency(model)
	}

	// Compute per-provider-family aggregates for current run
	for family, acc := range currentFamilyTotals {
		report.CurrentProviderFamilies[family] = acc.toModelEfficiency(family)
	}

	// Compute per-provider-family aggregates for historical runs
	for family, acc := range historicalFamilyTotals {
		report.HistoricalProviderFamilies[family] = acc.toModelEfficiency(family)
	}

	report.MixedProviderFamilies = hasMixedProviderFamilies(report.CurrentProviderFamilies, report.HistoricalProviderFamilies)

	// Compute overall averages
	report.CurrentAvgCostPerBead = computeOverallAvgCost(report.CurrentModels)
	report.CurrentAvgDurationPerBead = computeOverallAvgDuration(report.CurrentModels)
	report.HistoricalAvgCostPerBead = computeOverallAvgCost(report.HistoricalModels)
	report.HistoricalAvgDurationPerBead = computeOverallAvgDuration(report.HistoricalModels)

	// Compute deltas
	report.CostDelta = report.CurrentAvgCostPerBead - report.HistoricalAvgCostPerBead
	report.DurationDelta = report.CurrentAvgDurationPerBead - report.HistoricalAvgDurationPerBead

	return report, nil
}

func providerFamilyForModel(model string) string {
	switch model {
	case "opus", "sonnet", "haiku":
		return "claude"
	default:
		if strings.HasPrefix(model, "gpt-") && strings.HasSuffix(model, "-codex") {
			return "codex"
		}
	}
	return ""
}

func hasMixedProviderFamilies(current map[string]ModelEfficiency, historical map[string]ModelEfficiency) bool {
	families := make(map[string]bool)
	for family := range current {
		families[family] = true
	}
	for family := range historical {
		families[family] = true
	}
	return len(families) > 1
}

// modelAccumulator tracks running totals for a single model
type modelAccumulator struct {
	count         int
	totalCost     float64
	totalDuration time.Duration
	totalInput    int
	totalOutput   int
}

func (m *modelAccumulator) add(ie IterationEfficiency) {
	m.count++
	m.totalCost += ie.CostUSD
	m.totalDuration += ie.Duration
	m.totalInput += ie.InputTokens
	m.totalOutput += ie.OutputTokens
}

func (m *modelAccumulator) toModelEfficiency(model string) ModelEfficiency {
	if m.count == 0 {
		return ModelEfficiency{Model: model}
	}
	return ModelEfficiency{
		Model:             model,
		IterationCount:    m.count,
		AvgCostUSD:        m.totalCost / float64(m.count),
		AvgDuration:       m.totalDuration / time.Duration(m.count),
		AvgInputTokens:    float64(m.totalInput) / float64(m.count),
		AvgOutputTokens:   float64(m.totalOutput) / float64(m.count),
		TotalCostUSD:      m.totalCost,
		TotalDuration:     m.totalDuration,
		TotalInputTokens:  m.totalInput,
		TotalOutputTokens: m.totalOutput,
	}
}

// computeOverallAvgCost computes the weighted average cost across all models
func computeOverallAvgCost(models map[string]ModelEfficiency) float64 {
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

// computeOverallAvgDuration computes the weighted average duration across all models
func computeOverallAvgDuration(models map[string]ModelEfficiency) time.Duration {
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

// extractRunID extracts the run ID from a log file path like "run-20060102-150405.jsonl"
func extractRunID(path string) string {
	base := filepath.Base(path)
	// Remove "run-" prefix and ".jsonl" suffix
	if len(base) < 9 {
		return ""
	}
	return base[4 : len(base)-6]
}
