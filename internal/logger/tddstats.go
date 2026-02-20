package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	recordTypeField = "type"
	tddPhaseType    = "tdd_phase"
	tddSummaryType  = "tdd_summary"
)

// TDDStats captures aggregate TDD performance metrics from run JSONL logs.
type TDDStats struct {
	BeadRuns             int                `json:"bead_runs"`
	TotalCycles          int                `json:"total_cycles"`
	TotalPhases          int                `json:"total_phases"`
	AvgCyclesPerBead     float64            `json:"avg_cycles_per_bead"`
	AvgCostUSDPerCycle   float64            `json:"avg_cost_usd_per_cycle"`
	AvgInputTokensCycle  float64            `json:"avg_input_tokens_cycle"`
	AvgOutputTokensCycle float64            `json:"avg_output_tokens_cycle"`
	PhaseSuccessRates    map[string]float64 `json:"phase_success_rates"`
	EscalationPatterns   map[string]int     `json:"escalation_patterns"`
}

// ReadTDDPhaseRecords reads all run JSONL files and returns only tdd_phase records.
func ReadTDDPhaseRecords(logsDir string) ([]TDDPhaseRecord, error) {
	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	records := []TDDPhaseRecord{}
	for _, f := range files {
		runRecords, err := readTDDPhaseFile(f)
		if err != nil {
			continue
		}
		records = append(records, runRecords...)
	}

	return records, nil
}

// ReadTDDSummaries reads all run JSONL files and returns only tdd_summary records.
func ReadTDDSummaries(logsDir string) ([]TDDSummaryRecord, error) {
	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	records := []TDDSummaryRecord{}
	for _, f := range files {
		runRecords, err := readTDDSummaryFile(f)
		if err != nil {
			continue
		}
		records = append(records, runRecords...)
	}

	return records, nil
}

// AggregateTDDStats reads TDD phase/summary records and computes aggregate metrics.
func AggregateTDDStats(logsDir string) (TDDStats, error) {
	stats := TDDStats{
		PhaseSuccessRates:  map[string]float64{},
		EscalationPatterns: map[string]int{},
	}

	phases, err := ReadTDDPhaseRecords(logsDir)
	if err != nil {
		return stats, err
	}

	summaries, err := ReadTDDSummaries(logsDir)
	if err != nil {
		return stats, err
	}

	phaseTotals := map[string]int{}
	phaseSuccess := map[string]int{}
	beadIDs := map[string]bool{}
	var totalInputTokens int
	var totalOutputTokens int

	for _, phase := range phases {
		phaseTotals[phase.Phase]++
		if phase.Success {
			phaseSuccess[phase.Phase]++
		}
		totalInputTokens += phase.InputTokens
		totalOutputTokens += phase.OutputTokens

		if phase.Escalated {
			pattern := escalationPattern(phase)
			stats.EscalationPatterns[pattern]++
		}
	}

	for _, summary := range summaries {
		stats.BeadRuns++
		stats.TotalCycles += summary.TotalCycles
		stats.TotalPhases += summary.TotalPhases
		beadIDs[summary.BeadID] = true
	}

	if stats.BeadRuns > 0 {
		stats.AvgCyclesPerBead = float64(stats.TotalCycles) / float64(stats.BeadRuns)
	}

	for phase, total := range phaseTotals {
		if total == 0 {
			stats.PhaseSuccessRates[phase] = 0
			continue
		}
		stats.PhaseSuccessRates[phase] = float64(phaseSuccess[phase]) / float64(total)
	}

	if stats.TotalCycles > 0 {
		stats.AvgInputTokensCycle = float64(totalInputTokens) / float64(stats.TotalCycles)
		stats.AvgOutputTokensCycle = float64(totalOutputTokens) / float64(stats.TotalCycles)
	}

	totalTDDCostUSD, err := readIterationCostForBeads(logsDir, beadIDs)
	if err != nil {
		return stats, err
	}
	if stats.TotalCycles > 0 {
		stats.AvgCostUSDPerCycle = totalTDDCostUSD / float64(stats.TotalCycles)
	}

	return stats, nil
}

func readTDDPhaseFile(path string) ([]TDDPhaseRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	records := []TDDPhaseRecord{}
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			break
		}
		if !hasRecordType(raw, tddPhaseType) {
			continue
		}

		var rec TDDPhaseRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}

	return records, nil
}

func readTDDSummaryFile(path string) ([]TDDSummaryRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	records := []TDDSummaryRecord{}
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			break
		}
		if !hasRecordType(raw, tddSummaryType) {
			continue
		}

		var rec TDDSummaryRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}

	return records, nil
}

func hasRecordType(raw json.RawMessage, recordType string) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	typeField, ok := fields[recordTypeField]
	if !ok {
		return false
	}

	var gotType string
	if err := json.Unmarshal(typeField, &gotType); err != nil {
		return false
	}

	return gotType == recordType
}

func escalationPattern(phase TDDPhaseRecord) string {
	from := phase.EscalatedFrom
	if from == "" {
		from = "(unknown)"
	}
	return from + "->" + phase.Model
}

func readIterationCostForBeads(logsDir string, beadFilter map[string]bool) (float64, error) {
	if len(beadFilter) == 0 {
		return 0, nil
	}

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return 0, fmt.Errorf("globbing log files: %w", err)
	}

	var totalCost float64
	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if beadFilter[entry.BeadID] {
				totalCost += entry.CostUSD
			}
		}
	}

	return totalCost, nil
}
