package logger

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

// ReliabilityMetrics captures reliability KPIs derived from iteration logs.
type ReliabilityMetrics struct {
	AutonomyRate           float64            `json:"autonomy_rate"`
	FirstPassSuccessRate   float64            `json:"first_pass_success_rate"`
	MTTRProxyMs            int64              `json:"mttr_proxy_ms"`
	EscalationRatesByClass map[string]float64 `json:"escalation_rates_by_class"`
	RecurrenceCounters     map[string]int     `json:"recurrence_counters"`
}

// ReadReliabilityMetrics derives reliability metrics from run JSONL files.
func ReadReliabilityMetrics(logsDir string) (ReliabilityMetrics, error) {
	metrics := ReliabilityMetrics{
		EscalationRatesByClass: map[string]float64{},
		RecurrenceCounters:     map[string]int{},
	}

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return metrics, fmt.Errorf("globbing log files: %w", err)
	}
	sort.Strings(files)

	var autonomyEligible int
	var autonomySuccess int
	var firstPassTotal int
	var firstPassSuccess int
	var escalationTotal int
	var latestMTTRAt time.Time

	for _, file := range files {
		entries, err := readLogFile(file)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.AutonomyEligible {
				autonomyEligible++
				if entry.AutonomySuccess {
					autonomySuccess++
				}
			}
			firstPassTotal++
			if entry.FirstPassSuccess {
				firstPassSuccess++
			}

			if entry.MTTRProxyMs > 0 && !entry.Timestamp.Before(latestMTTRAt) {
				latestMTTRAt = entry.Timestamp
				metrics.MTTRProxyMs = entry.MTTRProxyMs
			}

			if entry.EscalationClass != "" {
				escalationTotal++
				metrics.EscalationRatesByClass[entry.EscalationClass]++
				if entry.RecurrenceCount > metrics.RecurrenceCounters[entry.EscalationClass] {
					metrics.RecurrenceCounters[entry.EscalationClass] = entry.RecurrenceCount
				}
			}
		}
	}

	if autonomyEligible > 0 {
		metrics.AutonomyRate = float64(autonomySuccess) / float64(autonomyEligible)
	}
	if firstPassTotal > 0 {
		metrics.FirstPassSuccessRate = float64(firstPassSuccess) / float64(firstPassTotal)
	}
	if escalationTotal > 0 {
		for class, count := range metrics.EscalationRatesByClass {
			metrics.EscalationRatesByClass[class] = count / float64(escalationTotal)
		}
	}

	return metrics, nil
}
