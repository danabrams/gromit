package logger

import (
	"errors"
	"testing"
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
