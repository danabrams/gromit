package logger

import (
	"errors"
	"testing"
	"time"
)

func TestReadAllIterationLogsSorted_UsesListRunLogFiles(t *testing.T) {
	original := listRunLogFilesFn
	defer func() {
		listRunLogFilesFn = original
	}()

	want := errors.New("boom")
	listRunLogFilesFn = func(string) ([]string, error) {
		return nil, want
	}

	if _, err := readAllIterationLogsSorted("ignored"); !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

// TestBuildIterationMetrics_MapsComplexityAndEstimatedFilesFields verifies that
// buildIterationMetrics correctly maps Complexity, ComplexitySource, and EstimatedFiles
// from IterationLog entries to IterationMetric.
func TestBuildIterationMetrics_MapsComplexityAndEstimatedFilesFields(t *testing.T) {
	t.Parallel()
	entries := []IterationLog{
		{
			BeadID:                   "bead-1",
			Iteration:                1,
			Success:                  true,
			DurationMs:               1000,
			CostUSD:                  1.0,
			InputTokens:              100,
			Complexity:               "high",
			ComplexitySource:         "scope_estimate",
			ComplexityFallbackReason: "none",
			EstimatedFiles:           10,
		},
	}

	metrics := buildIterationMetrics(entries, 10)

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	metric := metrics[0]
	if metric.Complexity != "high" {
		t.Errorf("Complexity = %q, want %q", metric.Complexity, "high")
	}
	if metric.ComplexitySource != "scope_estimate" {
		t.Errorf("ComplexitySource = %q, want %q", metric.ComplexitySource, "scope_estimate")
	}
	if metric.EstimatedFiles != 10 {
		t.Errorf("EstimatedFiles = %d, want %d", metric.EstimatedFiles, 10)
	}
}

// TestBuildIterationMetrics_FieldParityContractForAttributionFields verifies field parity
// between IterationLog and IterationMetric for observability/attribution fields.
// This contract test ensures that when run logs capture Complexity and EstimatedFiles data,
// iteration metrics will also have these fields to support retro analysis.
// Failure of this test indicates a schema divergence that would cause
// insufficient_current_run_data errors in retro analysis.
func TestBuildIterationMetrics_FieldParityContractForAttributionFields(t *testing.T) {
	t.Parallel()
	// Scenario: IterationLog has all attribution fields populated
	entries := []IterationLog{
		{
			BeadID:                   "bead-1",
			Iteration:                1,
			Success:                  true,
			DurationMs:               1000,
			CostUSD:                  1.0,
			InputTokens:              100,
			Complexity:               "medium",
			ComplexitySource:         "label",
			ComplexityFallbackReason: "scope_unavailable",
			EstimatedFiles:           7,
		},
		{
			BeadID:                   "bead-2",
			Iteration:                2,
			Success:                  false,
			DurationMs:               2000,
			CostUSD:                  2.0,
			InputTokens:              200,
			Complexity:               "low",
			ComplexitySource:         "default",
			ComplexityFallbackReason: "scope_and_label_unavailable",
			EstimatedFiles:           3,
		},
	}

	metrics := buildIterationMetrics(entries, 10)

	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	// Verify first iteration metric has non-empty attribution fields
	m1 := metrics[0]
	if m1.Complexity == "" {
		t.Error("Metric 1: Complexity empty when IterationLog.Complexity is non-empty")
	}
	if m1.ComplexitySource == "" {
		t.Error("Metric 1: ComplexitySource empty when IterationLog.ComplexitySource is non-empty")
	}
	if m1.EstimatedFiles == 0 {
		t.Error("Metric 1: EstimatedFiles zero when IterationLog.EstimatedFiles is non-zero")
	}

	// Verify second iteration metric has non-empty attribution fields
	m2 := metrics[1]
	if m2.Complexity == "" {
		t.Error("Metric 2: Complexity empty when IterationLog.Complexity is non-empty")
	}
	if m2.ComplexitySource == "" {
		t.Error("Metric 2: ComplexitySource empty when IterationLog.ComplexitySource is non-empty")
	}
	if m2.EstimatedFiles == 0 {
		t.Error("Metric 2: EstimatedFiles zero when IterationLog.EstimatedFiles is non-zero")
	}
}

// TestBuildIterationMetrics_AllComplexitySourceValues verifies that IterationMetric
// correctly preserves all complexity source values from IterationLog.
// This ensures complexity attribution is fully captured for retro analysis.
func TestBuildIterationMetrics_AllComplexitySourceValues(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                     string
		complexity               string
		complexitySource         string
		complexityFallbackReason string
		estimatedFiles           int
	}{
		{
			name:                     "scope_estimate_source",
			complexity:               "high",
			complexitySource:         "scope_estimate",
			complexityFallbackReason: "none",
			estimatedFiles:           25,
		},
		{
			name:                     "label_source",
			complexity:               "low",
			complexitySource:         "label",
			complexityFallbackReason: "scope_unavailable",
			estimatedFiles:           2,
		},
		{
			name:                     "default_source",
			complexity:               "medium",
			complexitySource:         "default",
			complexityFallbackReason: "scope_and_label_unavailable",
			estimatedFiles:           12,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entries := []IterationLog{
				{
					BeadID:                   "bead-test",
					Iteration:                1,
					Success:                  true,
					DurationMs:               1000,
					CostUSD:                  1.0,
					InputTokens:              100,
					Complexity:               tc.complexity,
					ComplexitySource:         tc.complexitySource,
					ComplexityFallbackReason: tc.complexityFallbackReason,
					EstimatedFiles:           tc.estimatedFiles,
				},
			}
			metrics := buildIterationMetrics(entries, 10)
			if len(metrics) != 1 {
				t.Fatalf("expected 1 metric, got %d", len(metrics))
			}
			m := metrics[0]
			if m.Complexity != tc.complexity {
				t.Errorf("Complexity = %q, want %q", m.Complexity, tc.complexity)
			}
			if m.ComplexitySource != tc.complexitySource {
				t.Errorf("ComplexitySource = %q, want %q", m.ComplexitySource, tc.complexitySource)
			}
			if m.EstimatedFiles != tc.estimatedFiles {
				t.Errorf("EstimatedFiles = %d, want %d", m.EstimatedFiles, tc.estimatedFiles)
			}
		})
	}
}

func TestDetectPackageMaintenanceCosts_FlagsAfterThreeBreaches(t *testing.T) {
	metrics := []IterationMetric{
		{RollingAvgValidationMs: 10, TouchedPackages: []string{"internal/pkg"}, Timestamp: time.Unix(1, 0)},
		{RollingAvgValidationMs: 200, TouchedPackages: []string{"internal/pkg"}, Timestamp: time.Unix(2, 0)},
		{RollingAvgValidationMs: 210, TouchedPackages: []string{"internal/pkg"}, Timestamp: time.Unix(3, 0)},
		{RollingAvgValidationMs: 220, TouchedPackages: []string{"internal/pkg"}, Timestamp: time.Unix(4, 0)},
	}
	limit := TrendControlLimit{
		Metric: metricRollingAvgValidationMs,
		UCL:    150,
	}

	costs := detectPackageMaintenanceCosts(metrics, limit)
	if len(costs) != 1 {
		t.Fatalf("expected 1 flagged package, got %d", len(costs))
	}
	if costs[0].Package != "internal/pkg" {
		t.Fatalf("unexpected package %q", costs[0].Package)
	}
	if costs[0].ConsecutiveBreaches < 3 {
		t.Fatalf("expected at least 3 consecutive breaches, got %d", costs[0].ConsecutiveBreaches)
	}
	if costs[0].LatestValue != 220 {
		t.Fatalf("unexpected latest value %.0f", costs[0].LatestValue)
	}
	if costs[0].Severity != anomalySeverityHigh {
		t.Fatalf("expected high severity, got %q", costs[0].Severity)
	}
	if !costs[0].DetectedAt.Equal(time.Unix(4, 0)) {
		t.Fatalf("unexpected detected at %v", costs[0].DetectedAt)
	}
}

func TestBuildIterationMetrics_ProviderRollingStats(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                     string
		windowSize               int
		metricIndex              int
		entries                  []IterationLog
		wantInvocations          int
		wantSuccesses            int
		wantSuccessRate          float64
		wantTransportFailures    int
		wantTransportFailureRate float64
	}{
		{
			name:       "mixed provider window",
			windowSize: 3,
			metricIndex: 2,
			entries: []IterationLog{
				{
					Timestamp:       time.Now(),
					Iteration:       1,
					Provider:        "openai",
					Model:           "gpt-5",
					Success:         true,
				},
				{
					Timestamp:       time.Now().Add(time.Second),
					Iteration:       2,
					Provider:        "claude",
					Model:           "claude-v1",
					Success:         false,
				},
				{
					Timestamp:       time.Now().Add(2 * time.Second),
					Iteration:       3,
					Provider:        "openai",
					Model:           "gpt-5",
					Success:         false,
					FailureCategory: transportDisconnectFailure,
				},
			},
			wantInvocations:          2,
			wantSuccesses:            1,
			wantSuccessRate:          0.5,
			wantTransportFailures:    1,
			wantTransportFailureRate: 0.5,
		},
		{
			name:       "inference fallback",
			windowSize: 3,
			metricIndex: 1,
			entries: []IterationLog{
				{
					Timestamp:       time.Now(),
					Iteration:       1,
					Provider:        "",
					Model:           "gpt-5.3-codex",
					Success:         true,
				},
				{
					Timestamp:       time.Now().Add(time.Second),
					Iteration:       2,
					Provider:        "",
					Model:           "gpt-5.3-codex",
					Success:         false,
					FailureCategory: transportDisconnectFailure,
				},
				{
					Timestamp:       time.Now().Add(2 * time.Second),
					Iteration:       3,
					Provider:        "claude",
					Model:           "claude-v1",
					Success:         true,
				},
			},
			wantInvocations:          2,
			wantSuccesses:            1,
			wantSuccessRate:          0.5,
			wantTransportFailures:    1,
			wantTransportFailureRate: 0.5,
		},
		{
			name:       "transport-only failure",
			windowSize: 1,
			metricIndex: 0,
			entries: []IterationLog{
				{
					Timestamp:       time.Now(),
					Iteration:       1,
					Provider:        "openai",
					Model:           "gpt-5",
					Success:         false,
					FailureCategory: transportDisconnectFailure,
				},
			},
			wantInvocations:          1,
			wantSuccesses:            0,
			wantSuccessRate:          0,
			wantTransportFailures:    1,
			wantTransportFailureRate: 1,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			metrics := buildIterationMetrics(tc.entries, tc.windowSize)
			if len(metrics) <= tc.metricIndex {
				t.Fatalf("expected at least %d metrics, got %d", tc.metricIndex+1, len(metrics))
			}
			metric := metrics[tc.metricIndex]
			if metric.RollingProviderInvocations != tc.wantInvocations {
				t.Fatalf("RollingProviderInvocations = %d, want %d", metric.RollingProviderInvocations, tc.wantInvocations)
			}
			if metric.RollingProviderSuccesses != tc.wantSuccesses {
				t.Fatalf("RollingProviderSuccesses = %d, want %d", metric.RollingProviderSuccesses, tc.wantSuccesses)
			}
			if metric.RollingProviderSuccessRate != tc.wantSuccessRate {
				t.Fatalf("RollingProviderSuccessRate = %v, want %v", metric.RollingProviderSuccessRate, tc.wantSuccessRate)
			}
			if metric.RollingProviderTransportFailures != tc.wantTransportFailures {
				t.Fatalf("RollingProviderTransportFailures = %d, want %d", metric.RollingProviderTransportFailures, tc.wantTransportFailures)
			}
			if metric.RollingProviderTransportFailureRate != tc.wantTransportFailureRate {
				t.Fatalf("RollingProviderTransportFailureRate = %v, want %v", metric.RollingProviderTransportFailureRate, tc.wantTransportFailureRate)
			}
		})
	}
}
