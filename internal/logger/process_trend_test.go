package logger

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/failurephase"
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
			FailurePhase:    failurephase.Validation,
			FailureCategory: "rate_limited",
			Success:         false,
		},
	}

	metrics := buildIterationMetrics(entries, 10)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	got := metrics[0]
	if got.FailurePhase != failurephase.Validation {
		t.Errorf("FailurePhase = %q, want %q", got.FailurePhase, failurephase.Validation)
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
		makeIterationLog(false, failurephase.Preflight),
		makeIterationLog(false, failurephase.Build),
		makeIterationLog(false, failurephase.Validation),
		makeIterationLog(false, failurephase.Timeout),
	}

	summary := summarizeWindow(window)
	assertFloatNear(t, summary.PreflightFailureRate, 0.2, "PreflightFailureRate")
	assertFloatNear(t, summary.BuildFailureRate, 0.2, "BuildFailureRate")
	assertFloatNear(t, summary.ValidationFailureRate, 0.2, "ValidationFailureRate")
	assertFloatNear(t, summary.TimeoutFailureRate, 0.2, "TimeoutFailureRate")
}

func TestBuildIterationMetrics_SinglePhaseRollingRates(t *testing.T) {
	entries := []IterationLog{
		makeIterationLog(false, failurephase.Build),
		makeIterationLog(false, failurephase.Build),
		makeIterationLog(false, failurephase.Build),
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

func TestBuildProcessTrend_PhaseRatesSumToFailureRate(t *testing.T) {
	entries := []IterationLog{
		makeIterationLog(false, failurephase.Preflight),
		makeIterationLog(false, failurephase.Build),
		makeIterationLog(true, ""),
	}

	metrics := buildIterationMetrics(entries, 3)
	trend := buildProcessTrend(metrics, 3)

	sum := trend.LatestWindow.PreflightFailureRate +
		trend.LatestWindow.BuildFailureRate +
		trend.LatestWindow.ValidationFailureRate +
		trend.LatestWindow.TimeoutFailureRate

	assertFloatNear(t, sum, trend.LatestWindow.FailureRate, "PhaseFailureRateSum")
}

func TestBuildProcessTrend_HasNineControlLimitsWithPhaseRates(t *testing.T) {
	entries := []IterationLog{
		makeIterationLog(false, failurephase.Preflight),
		makeIterationLog(false, failurephase.Build),
		makeIterationLog(false, failurephase.Validation),
		makeIterationLog(false, failurephase.Timeout),
		makeIterationLog(true, ""),
	}

	metrics := buildIterationMetrics(entries, 5)
	trend := buildProcessTrend(metrics, 5)

	if len(trend.ControlLimits) != 9 {
		t.Errorf("expected 9 control limits, got %d", len(trend.ControlLimits))
	}

	phaseRateNames := make(map[string]bool, len(phaseRateMetrics))
	for _, name := range phaseRateMetrics {
		phaseRateNames[name] = false
	}
	for _, cl := range trend.ControlLimits {
		if _, ok := phaseRateNames[cl.Metric]; ok {
			phaseRateNames[cl.Metric] = true
		}
	}
	for name, found := range phaseRateNames {
		if !found {
			t.Errorf("control limit %q not found in trend", name)
		}
	}
}

func TestBuildProcessTrend_BuildRateSpikeTriggersHighSeverityAnomaly(t *testing.T) {
	// 29 metrics with 0 build failure rate, then 1 spike to 1.0.
	// Mean ≈ 0.033, stddev ≈ 0.18; spike is >5σ away → high severity.
	metrics := make([]IterationMetric, 30)
	for i := range metrics {
		metrics[i] = IterationMetric{RollingBuildFailureRate: 0}
	}
	metrics[29].RollingBuildFailureRate = 1.0

	trend := buildProcessTrend(metrics, 30)

	var found bool
	for _, a := range trend.Anomalies {
		if a.Metric == metricRollingBuildFailure && a.Severity == "high" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected high-severity anomaly for rolling_build_failure_rate spike, none found")
	}
}

func TestBuildProcessTrend_PhaseRateControlLimitsClampedToZeroOne(t *testing.T) {
	// High-variance data: alternating 0/1 build failure rates produce UCL>1 and LCL<0 before clamping.
	metrics := make([]IterationMetric, 10)
	for i := range metrics {
		if i%2 == 0 {
			metrics[i].RollingBuildFailureRate = 1.0
		}
	}

	trend := buildProcessTrend(metrics, 10)

	phaseRates := make(map[string]bool, len(phaseRateMetrics))
	for _, name := range phaseRateMetrics {
		phaseRates[name] = false
	}
	for _, cl := range trend.ControlLimits {
		if _, ok := phaseRates[cl.Metric]; !ok {
			continue
		}
		phaseRates[cl.Metric] = true
		if cl.UCL > 1.0 {
			t.Errorf("%s UCL = %v, want <= 1.0", cl.Metric, cl.UCL)
		}
		if cl.LCL < 0.0 {
			t.Errorf("%s LCL = %v, want >= 0.0", cl.Metric, cl.LCL)
		}
	}
	for name, seen := range phaseRates {
		if !seen {
			t.Errorf("phase-rate control limit %q not found", name)
		}
	}
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
