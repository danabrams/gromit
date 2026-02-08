package retro

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExperiment_JSONSerialization(t *testing.T) {
	startTime := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)

	exp := Experiment{
		Name:        "Use haiku for test-only beads",
		Hypothesis:  "Beads that only modify test files can succeed with haiku, reducing cost by ~60% for those beads",
		Change:      "Add label `complexity:low` to beads whose title contains 'test'",
		Measurement: "Compare success rate and cost of test-only beads before vs after",
		Risk:        "Test-only beads may fail more on haiku, increasing retries",
		StartedAt:   startTime,
		BaselineMetrics: BaselineMetrics{
			AvgCostPerBead:  0.42,
			AvgDurationMs:   45000,
			AvgInputTokens:  12000.5,
			AvgOutputTokens: 3000.25,
			FailureRate:     0.08,
		},
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(exp, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify JSON contains expected fields
	jsonStr := string(data)
	expectedFields := []string{
		`"name"`,
		`"hypothesis"`,
		`"change"`,
		`"measurement"`,
		`"risk"`,
		`"started_at"`,
		`"baseline_metrics"`,
		`"avg_cost_per_bead"`,
		`"avg_duration_ms"`,
		`"avg_input_tokens"`,
		`"avg_output_tokens"`,
		`"failure_rate"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON missing field: %s", field)
		}
	}

	// Unmarshal back and verify
	var decoded Experiment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.Name != exp.Name {
		t.Errorf("Name mismatch: got %q, want %q", decoded.Name, exp.Name)
	}
	if decoded.Hypothesis != exp.Hypothesis {
		t.Errorf("Hypothesis mismatch: got %q, want %q", decoded.Hypothesis, exp.Hypothesis)
	}
	if decoded.Change != exp.Change {
		t.Errorf("Change mismatch: got %q, want %q", decoded.Change, exp.Change)
	}
	if decoded.Measurement != exp.Measurement {
		t.Errorf("Measurement mismatch: got %q, want %q", decoded.Measurement, exp.Measurement)
	}
	if decoded.Risk != exp.Risk {
		t.Errorf("Risk mismatch: got %q, want %q", decoded.Risk, exp.Risk)
	}
	if !decoded.StartedAt.Equal(exp.StartedAt) {
		t.Errorf("StartedAt mismatch: got %v, want %v", decoded.StartedAt, exp.StartedAt)
	}

	// Verify baseline metrics
	if decoded.BaselineMetrics.AvgCostPerBead != exp.BaselineMetrics.AvgCostPerBead {
		t.Errorf("AvgCostPerBead mismatch: got %v, want %v", decoded.BaselineMetrics.AvgCostPerBead, exp.BaselineMetrics.AvgCostPerBead)
	}
	if decoded.BaselineMetrics.AvgDurationMs != exp.BaselineMetrics.AvgDurationMs {
		t.Errorf("AvgDurationMs mismatch: got %v, want %v", decoded.BaselineMetrics.AvgDurationMs, exp.BaselineMetrics.AvgDurationMs)
	}
	if decoded.BaselineMetrics.AvgInputTokens != exp.BaselineMetrics.AvgInputTokens {
		t.Errorf("AvgInputTokens mismatch: got %v, want %v", decoded.BaselineMetrics.AvgInputTokens, exp.BaselineMetrics.AvgInputTokens)
	}
	if decoded.BaselineMetrics.AvgOutputTokens != exp.BaselineMetrics.AvgOutputTokens {
		t.Errorf("AvgOutputTokens mismatch: got %v, want %v", decoded.BaselineMetrics.AvgOutputTokens, exp.BaselineMetrics.AvgOutputTokens)
	}
	if decoded.BaselineMetrics.FailureRate != exp.BaselineMetrics.FailureRate {
		t.Errorf("FailureRate mismatch: got %v, want %v", decoded.BaselineMetrics.FailureRate, exp.BaselineMetrics.FailureRate)
	}
}

func TestBaselineMetrics_ZeroValues(t *testing.T) {
	metrics := BaselineMetrics{}

	// Marshal and unmarshal
	data, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BaselineMetrics
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify zero values are preserved
	if decoded.AvgCostPerBead != 0 {
		t.Errorf("AvgCostPerBead should be 0, got %v", decoded.AvgCostPerBead)
	}
	if decoded.AvgDurationMs != 0 {
		t.Errorf("AvgDurationMs should be 0, got %v", decoded.AvgDurationMs)
	}
	if decoded.AvgInputTokens != 0 {
		t.Errorf("AvgInputTokens should be 0, got %v", decoded.AvgInputTokens)
	}
	if decoded.AvgOutputTokens != 0 {
		t.Errorf("AvgOutputTokens should be 0, got %v", decoded.AvgOutputTokens)
	}
	if decoded.FailureRate != 0 {
		t.Errorf("FailureRate should be 0, got %v", decoded.FailureRate)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
