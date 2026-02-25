package logger

import (
	"fmt"
	"path/filepath"
)

// ModelStats represents aggregated statistics for a specific model
type ModelStats struct {
	Model           string  `json:"model"`
	Iterations      int     `json:"iterations"`
	Successes       int     `json:"successes"`
	Failures        int     `json:"failures"`
	EscalationsTo   int     `json:"escalations_to"`
	EscalationsFrom int     `json:"escalations_from"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
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
	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	return aggregateModelStats(files)
}

// ReadRunModelStats aggregates per-model statistics filtered by run ID
func ReadRunModelStats(logsDir string, runID string) (map[string]ModelStats, error) {
	// Empty runID returns empty stats
	if runID == "" {
		return make(map[string]ModelStats), nil
	}

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	// Filter files to only those matching the runID
	var filtered []string
	for _, f := range files {
		if extractRunID(f) == runID {
			filtered = append(filtered, f)
		}
	}

	return aggregateModelStats(filtered)
}

// aggregateModelStats aggregates per-model statistics from the given log files
func aggregateModelStats(files []string) (map[string]ModelStats, error) {
	modelMap := make(map[string]ModelStats)

	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue // Skip unreadable files
		}

		for _, entry := range entries {
			updateModelStats(modelMap, entry)
		}

		trackEscalationTargets(modelMap, entries)
	}

	return modelMap, nil
}

// updateModelStats updates the statistics for the model in the given entry
func updateModelStats(modelMap map[string]ModelStats, entry IterationLog) {
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

	// Track escalations from this model
	if entry.Escalated {
		stats.EscalationsFrom++
	}

	modelMap[entry.Model] = stats
}

// trackEscalationTargets tracks how many times each model was escalated to
func trackEscalationTargets(modelMap map[string]ModelStats, entries []IterationLog) {
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

// SpecCost represents aggregated cost and usage statistics for a spec
type SpecCost struct {
	TotalCostUSD float64        `json:"total_cost_usd"`
	Iterations   int            `json:"iterations"`
	Beads        int            `json:"beads"`
	ModelMix     map[string]int `json:"model_mix"`
}

const UnassignedSpecID = "(unassigned)"

// CostPerSpec aggregates cost and usage statistics grouped by spec_id.
// Entries with an empty spec_id are grouped under "unassigned".
func CostPerSpec(logsDir string) (map[string]SpecCost, error) {
	result := make(map[string]SpecCost)

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return result, fmt.Errorf("globbing log files: %w", err)
	}

	type specAccum struct {
		totalCostUSD float64
		iterations   int
		beadIDs      map[string]struct{}
		modelMix     map[string]int
	}
	accum := make(map[string]*specAccum)

	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			specID := entry.SpecID
			if specID == "" {
				specID = UnassignedSpecID
			}
			spec := accum[specID]
			if spec == nil {
				spec = &specAccum{
					beadIDs:  make(map[string]struct{}),
					modelMix: make(map[string]int),
				}
				accum[specID] = spec
			}
			spec.totalCostUSD += entry.CostUSD
			spec.iterations++
			spec.beadIDs[entry.BeadID] = struct{}{}
			spec.modelMix[entry.Model]++
		}
	}

	for specID, spec := range accum {
		result[specID] = SpecCost{
			TotalCostUSD: spec.totalCostUSD,
			Iterations:   spec.iterations,
			Beads:        len(spec.beadIDs),
			ModelMix:     spec.modelMix,
		}
	}

	return result, nil
}

// CostPerCompletedBead computes the total cost per completed bead
// including all retry attempts and escalations leading to completion
func CostPerCompletedBead(logsDir string) (map[string]float64, error) {
	beadCosts := make(map[string]float64)
	beadCompleted := make(map[string]bool)

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return beadCosts, fmt.Errorf("globbing log files: %w", err)
	}

	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue // Skip unreadable files
		}

		for _, entry := range entries {
			// Accumulate cost for this bead
			beadCosts[entry.BeadID] += entry.CostUSD

			// Track if bead ever succeeded
			if entry.Success {
				beadCompleted[entry.BeadID] = true
			}
		}
	}

	// Filter to only completed beads
	result := make(map[string]float64)
	for beadID, cost := range beadCosts {
		if beadCompleted[beadID] {
			result[beadID] = cost
		}
	}

	return result, nil
}
