package retro

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestLoadExperiment(t *testing.T) {
	tests := []struct {
		name        string
		setupFile   bool
		fileContent string
		wantNil     bool
		wantErr     bool
	}{
		{
			name:      "file does not exist",
			setupFile: false,
			wantNil:   true,
			wantErr:   false,
		},
		{
			name:      "valid experiment file",
			setupFile: true,
			fileContent: `{
  "name": "Test Experiment",
  "hypothesis": "This will work",
  "change": "Do something",
  "measurement": "Check results",
  "risk": "Might fail",
  "started_at": "2026-02-07T12:00:00Z",
  "baseline_metrics": {
    "avg_cost_per_bead": 0.5,
    "avg_duration_ms": 30000,
    "avg_input_tokens": 10000,
    "avg_output_tokens": 2000,
    "failure_rate": 0.1
  }
}`,
			wantNil: false,
			wantErr: false,
		},
		{
			name:        "invalid JSON",
			setupFile:   true,
			fileContent: `{invalid json`,
			wantNil:     false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "experiment.json")

			if tt.setupFile {
				if err := os.WriteFile(path, []byte(tt.fileContent), 0644); err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			}

			exp, err := LoadExperiment(path)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantNil && exp != nil {
				t.Error("expected nil experiment, got non-nil")
			}

			if !tt.wantNil && exp == nil {
				t.Error("expected non-nil experiment, got nil")
			}

			// Verify loaded data if we expect a valid experiment
			if !tt.wantNil && exp != nil {
				if exp.Name != "Test Experiment" {
					t.Errorf("Name mismatch: got %q, want %q", exp.Name, "Test Experiment")
				}
				if exp.BaselineMetrics.AvgCostPerBead != 0.5 {
					t.Errorf("AvgCostPerBead mismatch: got %v, want %v", exp.BaselineMetrics.AvgCostPerBead, 0.5)
				}
			}
		})
	}
}

func TestSaveExperiment(t *testing.T) {
	tests := []struct {
		name    string
		exp     *Experiment
		wantErr bool
	}{
		{
			name:    "nil experiment",
			exp:     nil,
			wantErr: true,
		},
		{
			name: "valid experiment",
			exp: &Experiment{
				Name:        "Test Experiment",
				Hypothesis:  "This will work",
				Change:      "Do something",
				Measurement: "Check results",
				Risk:        "Might fail",
				StartedAt:   time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC),
				BaselineMetrics: BaselineMetrics{
					AvgCostPerBead:  0.5,
					AvgDurationMs:   30000,
					AvgInputTokens:  10000,
					AvgOutputTokens: 2000,
					FailureRate:     0.1,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "experiment.json")

			err := SaveExperiment(path, tt.exp)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify file was created
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Error("experiment file was not created")
				return
			}

			// Verify we can read it back
			loaded, err := LoadExperiment(path)
			if err != nil {
				t.Errorf("failed to load saved experiment: %v", err)
				return
			}

			if loaded == nil {
				t.Error("loaded experiment is nil")
				return
			}

			if loaded.Name != tt.exp.Name {
				t.Errorf("Name mismatch: got %q, want %q", loaded.Name, tt.exp.Name)
			}
			if loaded.Hypothesis != tt.exp.Hypothesis {
				t.Errorf("Hypothesis mismatch: got %q, want %q", loaded.Hypothesis, tt.exp.Hypothesis)
			}
		})
	}
}

func TestDeleteExperiment(t *testing.T) {
	tests := []struct {
		name      string
		setupFile bool
		wantErr   bool
	}{
		{
			name:      "file does not exist",
			setupFile: false,
			wantErr:   false,
		},
		{
			name:      "file exists",
			setupFile: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "experiment.json")

			if tt.setupFile {
				exp := &Experiment{
					Name:       "Test",
					Hypothesis: "Test",
					Change:     "Test",
					StartedAt:  time.Now(),
				}
				if err := SaveExperiment(path, exp); err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			}

			err := DeleteExperiment(path)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify file is gone
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Error("experiment file still exists after delete")
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

// mockStatsData implements StatsData interface for testing
type mockStatsData struct {
	failureRate float64
}

func (m mockStatsData) FailureRate() float64 {
	return m.failureRate
}

// TestComputeBaselineMetrics_Success tests successful baseline computation
func TestComputeBaselineMetrics_Success(t *testing.T) {
	tests := []struct {
		name           string
		stats          mockStatsData
		efficiency     EfficiencyData
		wantCost       float64
		wantDurationMs int64
		wantInput      float64
		wantOutput     float64
		wantFailure    float64
	}{
		{
			name: "single model with multiple iterations",
			stats: mockStatsData{
				failureRate: 0.1,
			},
			efficiency: EfficiencyData{
				Models: map[string]ModelEfficiencyData{
					"sonnet": {
						TotalCostUSD:      4.20,
						TotalDuration:     450 * time.Second,
						TotalInputTokens:  120000,
						TotalOutputTokens: 30000,
						IterationCount:    10,
					},
				},
			},
			wantCost:       0.42,
			wantDurationMs: 45000,
			wantInput:      12000,
			wantOutput:     3000,
			wantFailure:    0.1,
		},
		{
			name: "multiple models with weighted averages",
			stats: mockStatsData{
				failureRate: 0.133,
			},
			efficiency: EfficiencyData{
				Models: map[string]ModelEfficiencyData{
					"opus": {
						TotalCostUSD:      6.0,
						TotalDuration:     300 * time.Second,
						TotalInputTokens:  150000,
						TotalOutputTokens: 40000,
						IterationCount:    5,
					},
					"sonnet": {
						TotalCostUSD:      4.0,
						TotalDuration:     400 * time.Second,
						TotalInputTokens:  100000,
						TotalOutputTokens: 20000,
						IterationCount:    10,
					},
				},
			},
			wantCost:       (6.0 + 4.0) / 15,
			wantDurationMs: ((300 + 400) * 1000) / 15,
			wantInput:      (150000.0 + 100000.0) / 15,
			wantOutput:     (40000.0 + 20000.0) / 15,
			wantFailure:    0.133,
		},
		{
			name: "zero iterations returns zero metrics",
			stats: mockStatsData{
				failureRate: 0,
			},
			efficiency: EfficiencyData{
				Models: map[string]ModelEfficiencyData{},
			},
			wantCost:       0,
			wantDurationMs: 0,
			wantInput:      0,
			wantOutput:     0,
			wantFailure:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readStats := func(dir string) (StatsData, error) {
				return tt.stats, nil
			}
			readEfficiency := func(dir, runID string) (EfficiencyData, error) {
				if runID != "" {
					t.Errorf("expected empty runID, got %q", runID)
				}
				return tt.efficiency, nil
			}

			got, err := ComputeBaselineMetrics("/fake/logs", readStats, readEfficiency)
			if err != nil {
				t.Fatalf("ComputeBaselineMetrics() error = %v", err)
			}

			const epsilon = 0.001
			if abs(got.AvgCostPerBead-tt.wantCost) > epsilon {
				t.Errorf("AvgCostPerBead = %v, want %v", got.AvgCostPerBead, tt.wantCost)
			}
			if got.AvgDurationMs != tt.wantDurationMs {
				t.Errorf("AvgDurationMs = %v, want %v", got.AvgDurationMs, tt.wantDurationMs)
			}
			if abs(got.AvgInputTokens-tt.wantInput) > epsilon {
				t.Errorf("AvgInputTokens = %v, want %v", got.AvgInputTokens, tt.wantInput)
			}
			if abs(got.AvgOutputTokens-tt.wantOutput) > epsilon {
				t.Errorf("AvgOutputTokens = %v, want %v", got.AvgOutputTokens, tt.wantOutput)
			}
			if abs(got.FailureRate-tt.wantFailure) > epsilon {
				t.Errorf("FailureRate = %v, want %v", got.FailureRate, tt.wantFailure)
			}
		})
	}
}

// TestComputeBaselineMetrics_ErrorHandling tests error propagation
func TestComputeBaselineMetrics_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		readStatsErr  bool
		readEffErr    bool
		wantErrSubstr string
	}{
		{
			name:          "stats read error",
			readStatsErr:  true,
			wantErrSubstr: "reading run stats",
		},
		{
			name:          "efficiency read error",
			readEffErr:    true,
			wantErrSubstr: "reading efficiency data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readStats := func(dir string) (StatsData, error) {
				if tt.readStatsErr {
					return nil, &testError{"stats error"}
				}
				return mockStatsData{}, nil
			}
			readEfficiency := func(dir, runID string) (EfficiencyData, error) {
				if tt.readEffErr {
					return EfficiencyData{}, &testError{"efficiency error"}
				}
				return EfficiencyData{Models: map[string]ModelEfficiencyData{}}, nil
			}

			_, err := ComputeBaselineMetrics("/fake/logs", readStats, readEfficiency)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !contains(err.Error(), tt.wantErrSubstr) {
				t.Errorf("error = %v, want substring %q", err, tt.wantErrSubstr)
			}
		})
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
