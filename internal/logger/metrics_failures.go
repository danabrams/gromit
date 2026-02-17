package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ReadConsecutiveFailureCounts reads iteration_metrics.jsonl and returns the
// most recent consecutive failure count per bead.
func ReadConsecutiveFailureCounts(metricsDir string) (map[string]int, error) {
	counts := make(map[string]int)
	if metricsDir == "" {
		return counts, nil
	}

	path := filepath.Join(metricsDir, "iteration_metrics.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return counts, nil
		}
		return counts, fmt.Errorf("reading iteration metrics: %w", err)
	}

	metrics := []IterationMetric{}
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var metric IterationMetric
		if err := dec.Decode(&metric); err != nil {
			break
		}
		metrics = append(metrics, metric)
	}

	seenSuccess := make(map[string]bool)
	for i := len(metrics) - 1; i >= 0; i-- {
		metric := metrics[i]
		if seenSuccess[metric.BeadID] {
			continue
		}
		if metric.Success {
			seenSuccess[metric.BeadID] = true
			continue
		}
		counts[metric.BeadID]++
	}

	return counts, nil
}
