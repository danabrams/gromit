package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildContinuousMetrics_FilesTouched(t *testing.T) {
	dir := t.TempDir()
	metricsDir := t.TempDir()

	l, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.LogIteration(&IterationLog{
		Timestamp:       time.Now(),
		Iteration:       1,
		BeadID:          "ft-1",
		Model:           "sonnet",
		Provider:        "codex",
		FailureCategory: "rate_limited",
		Success:         true,
		FilesTouched:    3,
	}); err != nil {
		t.Fatal(err)
	}
	l.Close()

	_, err = BuildContinuousMetrics(dir, metricsDir, 30)
	if err != nil {
		t.Fatal(err)
	}

	// Read back metrics JSONL and check FilesTouched
	data, err := os.ReadFile(filepath.Join(metricsDir, "iteration_metrics.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	var metric IterationMetric
	if err := json.Unmarshal(bytes.TrimSpace(data), &metric); err != nil {
		t.Fatal(err)
	}
	if metric.FilesTouched != 3 {
		t.Errorf("FilesTouched = %d, want 3", metric.FilesTouched)
	}
	if metric.Provider != "codex" {
		t.Errorf("Provider = %q, want %q", metric.Provider, "codex")
	}
	if metric.FailureCategory != "rate_limited" {
		t.Errorf("FailureCategory = %q, want %q", metric.FailureCategory, "rate_limited")
	}
}
