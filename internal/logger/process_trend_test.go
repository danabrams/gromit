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
	"github.com/danabrams/gromit/internal/prompt"
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

func TestBuildIterationMetrics_CopiesTokenCounts(t *testing.T) {
	entries := []IterationLog{
		{
			Timestamp:    time.Now(),
			Iteration:    1,
			BeadID:       "bead-tok",
			Model:        "gpt-4",
			Provider:     "codex",
			Success:      true,
			InputTokens:  1200,
			OutputTokens: 350,
		},
	}

	metrics := buildIterationMetrics(entries, 10)
	if len(metrics) != 1 {
		t.Fatalf("len(metrics) = %d, want 1", len(metrics))
	}
	if metrics[0].InputTokens != 1200 {
		t.Errorf("metrics[0].InputTokens = %d, want 1200", metrics[0].InputTokens)
	}
	if metrics[0].OutputTokens != 350 {
		t.Errorf("metrics[0].OutputTokens = %d, want 350", metrics[0].OutputTokens)
	}
}

func TestBuildIterationMetrics_CopiesValidationDurationAndRollingStats(t *testing.T) {
	entries := []IterationLog{
		{
			Timestamp:            time.Now(),
			Iteration:            1,
			BeadID:               "bead-val-1",
			Model:                "gpt-4",
			Success:              true,
			ValidationDurationMs: 100,
		},
		{
			Timestamp:            time.Now().Add(time.Second),
			Iteration:            2,
			BeadID:               "bead-val-2",
			Model:                "gpt-4",
			Success:              true,
			ValidationDurationMs: 0,
		},
		{
			Timestamp:            time.Now().Add(2 * time.Second),
			Iteration:            3,
			BeadID:               "bead-val-3",
			Model:                "gpt-4",
			Success:              true,
			ValidationDurationMs: 300,
		},
	}

	metrics := buildIterationMetrics(entries, 3)
	if len(metrics) != 3 {
		t.Fatalf("len(metrics) = %d, want 3", len(metrics))
	}

	last := metrics[2]
	if last.ValidationDurationMs != 300 {
		t.Errorf("ValidationDurationMs = %d, want 300", last.ValidationDurationMs)
	}
	assertFloatNear(t, last.RollingAvgValidationMs, 200, "RollingAvgValidationMs")
	assertFloatNear(t, last.RollingP95ValidationMs, 290, "RollingP95ValidationMs")
}

func TestBuildIterationMetrics_CopiesPromptDiagnostics(t *testing.T) {
	entries := []IterationLog{
		{
			Timestamp: time.Now(),
			Iteration: 1,
			BeadID:    "bead-prompt",
			Model:     "gpt-4",
			Success:   true,
			PromptDiagnostics: &prompt.PromptDiagnostics{
				PromptType:      "build",
				EstimatedTokens: 321,
			},
		},
	}

	metrics := buildIterationMetrics(entries, 10)
	if len(metrics) != 1 {
		t.Fatalf("len(metrics) = %d, want 1", len(metrics))
	}
	if metrics[0].PromptDiagnostics == nil {
		t.Fatal("PromptDiagnostics = nil, want non-nil")
	}
	if metrics[0].PromptDiagnostics.PromptType != "build" {
		t.Errorf("PromptType = %q, want %q", metrics[0].PromptDiagnostics.PromptType, "build")
	}
	if metrics[0].PromptDiagnostics.EstimatedTokens != 321 {
		t.Errorf("EstimatedTokens = %d, want %d", metrics[0].PromptDiagnostics.EstimatedTokens, 321)
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

func TestSummarizeWindow_ValidationDurationExcludesZeroEntries(t *testing.T) {
	window := []IterationLog{
		{Success: true, ValidationDurationMs: 0},
		{Success: true, ValidationDurationMs: 100},
		{Success: true, ValidationDurationMs: 300},
	}

	summary := summarizeWindow(window)
	assertFloatNear(t, summary.AvgValidationMs, 200, "AvgValidationMs")
	assertFloatNear(t, summary.P95ValidationMs, 290, "P95ValidationMs")
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

func TestBuildProcessTrend_HasExpectedControlLimitsWithPhaseRates(t *testing.T) {
	entries := []IterationLog{
		makeIterationLog(false, failurephase.Preflight),
		makeIterationLog(false, failurephase.Build),
		makeIterationLog(false, failurephase.Validation),
		makeIterationLog(false, failurephase.Timeout),
		makeIterationLog(true, ""),
	}

	metrics := buildIterationMetrics(entries, 5)
	trend := buildProcessTrend(metrics, 5)

	if len(trend.ControlLimits) != len(trendControlLimitSeries) {
		t.Errorf("expected %d control limits, got %d", len(trendControlLimitSeries), len(trend.ControlLimits))
	}

	phaseRateNames := phaseRateMetricPresence()
	for _, cl := range trend.ControlLimits {
		if _, ok := phaseRateNames[cl.Metric]; ok {
			phaseRateNames[cl.Metric] = true
		}
	}
	assertAllPhaseRateMetricsFound(t, phaseRateNames, "control limit")
}

func TestBuildProcessTrend_IncludesRollingAvgValidationControlLimit(t *testing.T) {
	entries := []IterationLog{
		{Success: true, ValidationDurationMs: 50},
		{Success: true, ValidationDurationMs: 100},
		{Success: true, ValidationDurationMs: 200},
	}

	metrics := buildIterationMetrics(entries, 3)
	trend := buildProcessTrend(metrics, 3)

	if _, ok := findControlLimit(trend.ControlLimits, metricRollingAvgValidationMs); !ok {
		t.Fatalf("control limits missing %q", metricRollingAvgValidationMs)
	}
}

func TestBuildProcessTrend_ValidationDurationSpikeTriggersHighSeverityAnomaly(t *testing.T) {
	metrics := make([]IterationMetric, 30)
	for i := 0; i < 29; i++ {
		metrics[i] = IterationMetric{RollingAvgValidationMs: 0}
	}
	metrics[29] = IterationMetric{RollingAvgValidationMs: 1000}

	trend := buildProcessTrend(metrics, 30)

	var found bool
	for _, a := range trend.Anomalies {
		if a.Metric == metricRollingAvgValidationMs && a.Severity == anomalySeverityHigh {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected high-severity anomaly for %s spike", metricRollingAvgValidationMs)
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
		if a.Metric == metricRollingBuildFailure && a.Severity == anomalySeverityHigh {
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

	phaseRates := phaseRateMetricPresence()
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
	assertAllPhaseRateMetricsFound(t, phaseRates, "phase-rate control limit")
}

func TestBuildProcessTrend_AggregatesPromptTokenSummary(t *testing.T) {
	entries := []IterationLog{
		{
			Iteration: 1,
			PromptDiagnostics: &prompt.PromptDiagnostics{
				PromptType:      "build",
				EstimatedTokens: 100,
				SectionTokens: map[string]int{
					"rules": 60,
					"spec":  40,
				},
				ShapeActions:   []string{"trim_rules", "trim_rules"},
				ReportedTokens: 120,
				TokenDeltaPct:  -16.6667,
			},
		},
		{
			Iteration: 2,
			PromptDiagnostics: &prompt.PromptDiagnostics{
				PromptType:      "build",
				EstimatedTokens: 200,
				SectionTokens: map[string]int{
					"rules": 50,
					"diff":  150,
				},
				ShapeActions:   []string{"trim_spec"},
				ReportedTokens: 220,
				TokenDeltaPct:  -9.0909,
			},
		},
		{
			Iteration: 3,
			PromptDiagnostics: &prompt.PromptDiagnostics{
				PromptType:      "plan",
				EstimatedTokens: 50,
				SectionTokens: map[string]int{
					"task_identity": 50,
				},
				ReportedTokens: 0,
				TokenDeltaPct:  80,
			},
		},
		{
			Iteration: 4,
			PromptDiagnostics: &prompt.PromptDiagnostics{
				PromptType:      "review",
				EstimatedTokens: 300,
				SectionTokens: map[string]int{
					"r11": 1, "r10": 2, "r9": 3, "r8": 4, "r7": 5, "r6": 6,
					"r5": 7, "r4": 8, "r3": 9, "r2": 10, "r1": 11,
				},
				ShapeActions:   []string{"trim_rules", "trim_spec", "trim_spec"},
				ReportedTokens: 300,
				TokenDeltaPct:  0,
			},
		},
	}

	metrics := buildIterationMetrics(entries, 3)
	trend := buildProcessTrend(metrics, 3)

	gotTypes := trend.PromptTokenSummary.ByPromptType
	if len(gotTypes) != 3 {
		t.Fatalf("len(ByPromptType) = %d, want 3", len(gotTypes))
	}
	if gotTypes[0].PromptType != "review" || gotTypes[0].InvocationCount != 1 || gotTypes[0].AvgEstimatedTokens != 300 {
		t.Errorf("ByPromptType[0] = %+v, want review with count 1 avg 300", gotTypes[0])
	}
	if gotTypes[1].PromptType != "build" || gotTypes[1].InvocationCount != 1 || gotTypes[1].AvgEstimatedTokens != 200 {
		t.Errorf("ByPromptType[1] = %+v, want build with count 1 avg 200", gotTypes[1])
	}
	if gotTypes[2].PromptType != "plan" || gotTypes[2].InvocationCount != 1 || gotTypes[2].AvgEstimatedTokens != 50 {
		t.Errorf("ByPromptType[2] = %+v, want plan with count 1 avg 50", gotTypes[2])
	}

	gotSections := trend.PromptTokenSummary.BySectionTop10
	if len(gotSections) != 10 {
		t.Fatalf("len(BySectionTop10) = %d, want 10", len(gotSections))
	}
	for _, section := range gotSections {
		if section.Section == "r11" {
			t.Fatal("expected lowest section r11 to be trimmed from top 10")
		}
	}

	if trend.PromptTokenSummary.BudgetActionFrequency["trim_spec"] != 3 {
		t.Errorf("BudgetActionFrequency[trim_spec] = %d, want 3", trend.PromptTokenSummary.BudgetActionFrequency["trim_spec"])
	}
	if trend.PromptTokenSummary.BudgetActionFrequency["trim_rules"] != 1 {
		t.Errorf("BudgetActionFrequency[trim_rules] = %d, want 1", trend.PromptTokenSummary.BudgetActionFrequency["trim_rules"])
	}

	drift := trend.PromptTokenSummary.ReconciliationDrift
	if drift.SampleCount != 2 {
		t.Fatalf("SampleCount = %d, want 2", drift.SampleCount)
	}
	assertFloatNear(t, drift.MeanAbsTokenDeltaPct, 4.54545, "MeanAbsTokenDeltaPct")
	assertFloatNear(t, drift.P95AbsTokenDeltaPct, 8.636355, "P95AbsTokenDeltaPct")
}

func TestBuildContinuousMetrics_ValidationDurationControlLimitInProcessTrendFile(t *testing.T) {
	logsDir := t.TempDir()
	metricsDir := t.TempDir()

	l, err := NewLogger(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.LogIteration(&IterationLog{
		Timestamp:            time.Now(),
		Iteration:            1,
		BeadID:               "vd-1",
		Model:                "sonnet",
		Success:              true,
		DurationMs:           5000,
		ValidationDurationMs: 1200,
	}); err != nil {
		t.Fatal(err)
	}
	if err := l.LogIteration(&IterationLog{
		Timestamp:            time.Now().Add(time.Second),
		Iteration:            2,
		BeadID:               "vd-2",
		Model:                "sonnet",
		Success:              true,
		DurationMs:           6000,
		ValidationDurationMs: 2000,
	}); err != nil {
		t.Fatal(err)
	}
	l.Close()

	trend, err := BuildContinuousMetrics(logsDir, metricsDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	if trend == nil {
		t.Fatal("BuildContinuousMetrics returned nil trend")
	}

	readTrend, err := ReadProcessTrend(filepath.Join(metricsDir, processTrendFilename))
	if err != nil {
		t.Fatal(err)
	}
	if readTrend == nil {
		t.Fatal("ReadProcessTrend returned nil trend")
	}
	if _, ok := findControlLimit(readTrend.ControlLimits, metricRollingAvgValidationMs); !ok {
		t.Fatalf("process_trend.json control limits missing %q", metricRollingAvgValidationMs)
	}
}

func makeIterationLog(success bool, phase string) IterationLog {
	return IterationLog{
		Success:      success,
		FailurePhase: phase,
	}
}

func phaseRateMetricPresence() map[string]bool {
	seen := make(map[string]bool, len(phaseRateMetrics))
	for _, name := range phaseRateMetrics {
		seen[name] = false
	}
	return seen
}

func assertAllPhaseRateMetricsFound(t *testing.T, found map[string]bool, label string) {
	t.Helper()
	for name, present := range found {
		if !present {
			t.Errorf("%s %q not found", label, name)
		}
	}
}

func findControlLimit(controlLimits []TrendControlLimit, metric string) (TrendControlLimit, bool) {
	for _, cl := range controlLimits {
		if cl.Metric == metric {
			return cl, true
		}
	}
	return TrendControlLimit{}, false
}

func assertFloatNear(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}
