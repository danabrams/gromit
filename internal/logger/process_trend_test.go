package logger

import (
	"bytes"
	"encoding/json"
	"math"
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

func TestBuildIterationMetrics_CopiesFailurePhaseAndCategory(t *testing.T) {
	entries := []IterationLog{
		{
			Timestamp:       time.Now(),
			Iteration:       1,
			BeadID:          "b-1",
			Model:           "sonnet",
			FailurePhase:    "validation",
			FailureCategory: "rate_limited",
			Success:         false,
		},
	}

	metrics := buildIterationMetrics(entries, 10)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	got := metrics[0]
	if got.FailurePhase != "validation" {
		t.Errorf("FailurePhase = %q, want %q", got.FailurePhase, "validation")
	}
	if got.FailureCategory != "rate_limited" {
		t.Errorf("FailureCategory = %q, want %q", got.FailureCategory, "rate_limited")
	}
}

func TestSummarizeWindow_AllSuccessPhaseRatesZero(t *testing.T) {
	window := []IterationLog{
		{Success: true},
		{Success: true},
		{Success: true},
	}

	summary := summarizeWindow(window)
	if summary.PreflightFailureRate != 0 {
		t.Errorf("PreflightFailureRate = %v, want 0", summary.PreflightFailureRate)
	}
	if summary.BuildFailureRate != 0 {
		t.Errorf("BuildFailureRate = %v, want 0", summary.BuildFailureRate)
	}
	if summary.ValidationFailureRate != 0 {
		t.Errorf("ValidationFailureRate = %v, want 0", summary.ValidationFailureRate)
	}
	if summary.TimeoutFailureRate != 0 {
		t.Errorf("TimeoutFailureRate = %v, want 0", summary.TimeoutFailureRate)
	}
}

func TestSummarizeWindow_MixedPhaseRates(t *testing.T) {
	window := []IterationLog{
		makeIterationLog(true, ""),
		makeIterationLog(false, "preflight"),
		makeIterationLog(false, "build"),
		makeIterationLog(false, "validation"),
		makeIterationLog(false, "timeout"),
	}

	summary := summarizeWindow(window)
	assertFloatNear(t, summary.PreflightFailureRate, 0.2, "PreflightFailureRate")
	assertFloatNear(t, summary.BuildFailureRate, 0.2, "BuildFailureRate")
	assertFloatNear(t, summary.ValidationFailureRate, 0.2, "ValidationFailureRate")
	assertFloatNear(t, summary.TimeoutFailureRate, 0.2, "TimeoutFailureRate")
}

func TestBuildIterationMetrics_SinglePhaseRollingRates(t *testing.T) {
	entries := []IterationLog{
		makeIterationLog(false, "build"),
		makeIterationLog(false, "build"),
		makeIterationLog(false, "build"),
	}

	metrics := buildIterationMetrics(entries, 3)
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(metrics))
	}

	last := metrics[2]
	assertFloatNear(t, last.RollingBuildFailureRate, 1, "RollingBuildFailureRate")
	assertFloatNear(t, last.RollingPreflightFailureRate, 0, "RollingPreflightFailureRate")
	assertFloatNear(t, last.RollingValidationFailureRate, 0, "RollingValidationFailureRate")
	assertFloatNear(t, last.RollingTimeoutFailureRate, 0, "RollingTimeoutFailureRate")
}

func makeIterationLog(success bool, phase string) IterationLog {
	return IterationLog{
		Success:      success,
		FailurePhase: phase,
	}
}

func assertFloatNear(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}
