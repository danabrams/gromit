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

func TestComputeProviderMetrics(t *testing.T) {
	testCases := []struct {
		name    string
		entries []IterationMetric
		want    map[string]ProviderMetrics
	}{
		{
			name: "groups_by_explicit_provider_and_calculates_rates",
			entries: []IterationMetric{
				{
					Provider:        "claude",
					Success:         true,
					DurationMs:      1000,
					CostUSD:         0.10,
					InputTokens:     100,
					OutputTokens:    10,
					FailureCategory: "",
				},
				{
					Provider:        "claude",
					Success:         false,
					DurationMs:      3000,
					CostUSD:         0.30,
					InputTokens:     200,
					OutputTokens:    20,
					Escalated:       true,
					FailureCategory: transportDisconnectFailure,
				},
				{
					Provider:        "openai",
					Success:         true,
					DurationMs:      500,
					CostUSD:         0.05,
					InputTokens:     50,
					OutputTokens:    5,
					FailureCategory: "",
				},
			},
			want: map[string]ProviderMetrics{
				"claude": {
					Name:                 "claude",
					TotalInvocations:     2,
					Successes:            1,
					SuccessRate:          0.5,
					TransportFailures:    1,
					TransportFailureRate: 0.5,
					FallbacksTriggered:   1,
					AvgDurationMs:        2000,
					TotalCostUSD:         0.40,
					TotalInputTokens:     300,
					TotalOutputTokens:    30,
				},
				"openai": {
					Name:                 "openai",
					TotalInvocations:     1,
					Successes:            1,
					SuccessRate:          1,
					TransportFailures:    0,
					TransportFailureRate: 0,
					FallbacksTriggered:   0,
					AvgDurationMs:        500,
					TotalCostUSD:         0.05,
					TotalInputTokens:     50,
					TotalOutputTokens:    5,
				},
			},
		},
		{
			name: "infers_provider_from_model_when_provider_missing",
			entries: []IterationMetric{
				{
					Model:        "gpt-5.3-codex",
					Success:      true,
					DurationMs:   1500,
					CostUSD:      0.20,
					InputTokens:  300,
					OutputTokens: 30,
				},
				{
					Model:           "sonnet",
					Success:         false,
					DurationMs:      2500,
					CostUSD:         0.15,
					InputTokens:     400,
					OutputTokens:    40,
					FailureCategory: transportDisconnectFailure,
				},
			},
			want: map[string]ProviderMetrics{
				"openai": {
					Name:                 "openai",
					TotalInvocations:     1,
					Successes:            1,
					SuccessRate:          1,
					TransportFailures:    0,
					TransportFailureRate: 0,
					FallbacksTriggered:   0,
					AvgDurationMs:        1500,
					TotalCostUSD:         0.20,
					TotalInputTokens:     300,
					TotalOutputTokens:    30,
				},
				"claude": {
					Name:                 "claude",
					TotalInvocations:     1,
					Successes:            0,
					SuccessRate:          0,
					TransportFailures:    1,
					TransportFailureRate: 1,
					FallbacksTriggered:   0,
					AvgDurationMs:        2500,
					TotalCostUSD:         0.15,
					TotalInputTokens:     400,
					TotalOutputTokens:    40,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeProviderMetrics(tc.entries)
			if len(got) != len(tc.want) {
				t.Fatalf("len(got) = %d, want %d", len(got), len(tc.want))
			}
			for _, metric := range got {
				wantMetric, ok := tc.want[metric.Name]
				if !ok {
					t.Fatalf("unexpected provider metric for %q", metric.Name)
				}
				assertProviderMetricsEqual(t, metric, wantMetric)
			}
		})
	}
}

func TestBuildProcessTrend_IncludesProviderMetrics(t *testing.T) {
	metrics := []IterationMetric{
		{
			Model:        "gpt-5.3-codex",
			Provider:     "",
			Success:      true,
			DurationMs:   1000,
			CostUSD:      0.25,
			InputTokens:  200,
			OutputTokens: 20,
		},
		{
			Model:           "sonnet",
			Provider:        "",
			Success:         false,
			DurationMs:      2000,
			CostUSD:         0.10,
			InputTokens:     150,
			OutputTokens:    15,
			FailureCategory: transportDisconnectFailure,
		},
	}

	trend := buildProcessTrend(metrics, 30)
	if len(trend.ProviderMetrics) != 2 {
		t.Fatalf("len(ProviderMetrics) = %d, want 2", len(trend.ProviderMetrics))
	}

	foundOpenAI := false
	foundClaude := false
	for _, metric := range trend.ProviderMetrics {
		switch metric.Name {
		case "openai":
			foundOpenAI = true
		case "claude":
			foundClaude = true
		}
	}
	if !foundOpenAI {
		t.Error("ProviderMetrics missing inferred openai entry")
	}
	if !foundClaude {
		t.Error("ProviderMetrics missing inferred claude entry")
	}
}

func TestSummarizeWindow_AvgCostPerBead_SumsRetriesPerBead(t *testing.T) {
	// Bead A: 3 iterations at $2 each, 3rd is successful → total $6
	// Bead B: 1 iteration at $3, successful → total $3
	// Expected: avg = ($6 + $3) / 2 = $4.50
	window := []IterationLog{
		{BeadID: "bead-a", CostUSD: 2.0, Success: false},
		{BeadID: "bead-a", CostUSD: 2.0, Success: false},
		{BeadID: "bead-a", CostUSD: 2.0, Success: true},
		{BeadID: "bead-b", CostUSD: 3.0, Success: true},
	}

	summary := summarizeWindow(window)
	assertFloatNear(t, summary.AvgCostPerBeadUSD, 4.50, "AvgCostPerBeadUSD")
}

func TestSummarizeWindow_AvgCostPerBead_ExcludesIncompleteBeads(t *testing.T) {
	// Bead A: 1 successful iteration at $5 → total $5
	// Bead B: 2 failed iterations at $2 each → not counted (no success in window)
	// Expected: avg = $5 / 1 = $5
	window := []IterationLog{
		{BeadID: "bead-a", CostUSD: 5.0, Success: true},
		{BeadID: "bead-b", CostUSD: 2.0, Success: false},
		{BeadID: "bead-b", CostUSD: 2.0, Success: false},
	}

	summary := summarizeWindow(window)
	assertFloatNear(t, summary.AvgCostPerBeadUSD, 5.0, "AvgCostPerBeadUSD")
}

func TestSummarizeWindow_AvgCostPerBead_ZeroWhenNoCompletedBeads(t *testing.T) {
	window := []IterationLog{
		{BeadID: "bead-a", CostUSD: 2.0, Success: false},
	}

	summary := summarizeWindow(window)
	assertFloatNear(t, summary.AvgCostPerBeadUSD, 0.0, "AvgCostPerBeadUSD")
}

func TestBuildIterationMetrics_RollingAvgCostPerBeadUSD(t *testing.T) {
	// Window of 3: bead-a has 2 retries + success ($1+$1+$1=$3), bead-b succeeds once ($2)
	// AvgCostPerBeadUSD = ($3 + $2) / 2 = $2.50
	entries := []IterationLog{
		{Timestamp: time.Now(), BeadID: "bead-a", CostUSD: 1.0, Success: false},
		{Timestamp: time.Now().Add(time.Second), BeadID: "bead-a", CostUSD: 1.0, Success: false},
		{Timestamp: time.Now().Add(2 * time.Second), BeadID: "bead-a", CostUSD: 1.0, Success: true},
		{Timestamp: time.Now().Add(3 * time.Second), BeadID: "bead-b", CostUSD: 2.0, Success: true},
	}

	metrics := buildIterationMetrics(entries, 10)
	if len(metrics) != 4 {
		t.Fatalf("len(metrics) = %d, want 4", len(metrics))
	}

	last := metrics[len(metrics)-1]
	assertFloatNear(t, last.RollingAvgCostPerBeadUSD, 2.50, "RollingAvgCostPerBeadUSD")
}

func TestBuildProcessTrend_IncludesRollingAvgCostPerBeadControlLimit(t *testing.T) {
	entries := []IterationLog{
		{BeadID: "b1", CostUSD: 1.0, Success: true},
		{BeadID: "b2", CostUSD: 2.0, Success: true},
		{BeadID: "b3", CostUSD: 3.0, Success: true},
	}

	metrics := buildIterationMetrics(entries, 3)
	trend := buildProcessTrend(metrics, 3)

	if _, ok := findControlLimit(trend.ControlLimits, metricRollingAvgCostPerBeadUSD); !ok {
		t.Fatalf("control limits missing %q", metricRollingAvgCostPerBeadUSD)
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

func assertProviderMetricsEqual(t *testing.T, got, want ProviderMetrics) {
	t.Helper()
	if got.TotalInvocations != want.TotalInvocations {
		t.Errorf("%s.TotalInvocations = %d, want %d", got.Name, got.TotalInvocations, want.TotalInvocations)
	}
	if got.Successes != want.Successes {
		t.Errorf("%s.Successes = %d, want %d", got.Name, got.Successes, want.Successes)
	}
	assertFloatNear(t, got.SuccessRate, want.SuccessRate, got.Name+".SuccessRate")
	if got.TransportFailures != want.TransportFailures {
		t.Errorf("%s.TransportFailures = %d, want %d", got.Name, got.TransportFailures, want.TransportFailures)
	}
	assertFloatNear(t, got.TransportFailureRate, want.TransportFailureRate, got.Name+".TransportFailureRate")
	if got.FallbacksTriggered != want.FallbacksTriggered {
		t.Errorf("%s.FallbacksTriggered = %d, want %d", got.Name, got.FallbacksTriggered, want.FallbacksTriggered)
	}
	assertFloatNear(t, got.AvgDurationMs, want.AvgDurationMs, got.Name+".AvgDurationMs")
	assertFloatNear(t, got.TotalCostUSD, want.TotalCostUSD, got.Name+".TotalCostUSD")
	if got.TotalInputTokens != want.TotalInputTokens {
		t.Errorf("%s.TotalInputTokens = %d, want %d", got.Name, got.TotalInputTokens, want.TotalInputTokens)
	}
	if got.TotalOutputTokens != want.TotalOutputTokens {
		t.Errorf("%s.TotalOutputTokens = %d, want %d", got.Name, got.TotalOutputTokens, want.TotalOutputTokens)
	}
}
