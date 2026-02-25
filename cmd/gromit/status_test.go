package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/runner"
)

func TestStatusCmd_OutputIncludesPipelineSection(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Create gromit.yaml config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create status.json with a stopped run (PID 0 avoids process-alive checks)
	status := runner.Status{
		Running:   false,
		Iteration: 3,
		BeadID:    "gromit-abc",
		BeadTitle: "Fix login",
		Model:     "sonnet",
		StartedAt: time.Now().Add(-5 * time.Minute),
		ElapsedS:  300,
	}
	statusData, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "status.json"), statusData, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	t.Chdir(tmpDir)

	// Execute status command and capture output
	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status command failed: %v", err)
		}
	})

	// When showStatus delegates to runner.PrintStatus, the output includes
	// a Pipeline section. The current manual formatting does not.
	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("expected output to contain 'Pipeline:' section (from runner.PrintStatus), got:\n%s", output)
	}
}

func TestStatusCmd_SPCFlagDisplaysNoData(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Chdir(tmpDir)

	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"status", "--spc"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status --spc command failed: %v", err)
		}
	})

	const noData = "SPC: (no data)"
	if !strings.Contains(output, noData) {
		t.Fatalf("expected %q in output when no process trend data exists, got:\n%s", noData, output)
	}
}

func TestStatusCmd_SPCFlagSkipsDefaultSections(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Chdir(tmpDir)

	stdout, stderr, exitCode := runGromitCobra(t, "status", "--spc")
	if exitCode != 0 {
		t.Fatalf("status --spc exit %d, stderr: %s", exitCode, stderr)
	}

	// With --spc flag, should show SPC data (or "no data" message)
	if !strings.Contains(stdout, "SPC:") {
		t.Fatalf("expected SPC output when --spc flag is used, got:\n%s", stdout)
	}

	for _, section := range []string{"Run:", "Pipeline:", "Health:"} {
		if strings.Contains(stdout, section) {
			t.Fatalf("expected SPC guard path to skip %q section, got:\n%s", section, stdout)
		}
	}
}

func TestStatusCmd_SPCFlagWithStableOutput(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	metricsDir := filepath.Join(gromitDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		t.Fatalf("failed to create metrics dir: %v", err)
	}

	// Create gromit.yaml config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create process_trend.json with test data
	processTrend := map[string]interface{}{
		"generated_at":      time.Now().Format(time.RFC3339),
		"total_iterations":  10,
		"window_size":       5,
		"latest_window":     map[string]interface{}{},
		"prompt_token_summary": map[string]interface{}{
			"by_prompt_type": []interface{}{},
		},
		"provider_metrics": []interface{}{},
		"control_limits": []map[string]interface{}{
			{
				"metric": "rolling_success_rate",
				"latest": 0.9,
				"lcl":    0.7,
				"ucl":    1.0,
			},
			{
				"metric": "rolling_escalation_rate",
				"latest": 0.1,
				"lcl":    0.0,
				"ucl":    0.3,
			},
			{
				"metric": "rolling_quality_score",
				"latest": 0.85,
				"lcl":    0.6,
				"ucl":    1.0,
			},
			{
				"metric": "rolling_avg_duration_ms",
				"latest": 5000.0,
				"lcl":    2000.0,
				"ucl":    8000.0,
			},
		},
		"stratified_control_limits": map[string]interface{}{},
		"anomalies":                 []interface{}{},
		"stratified_anomalies":      map[string]interface{}{},
		"ewma_anomalies":            []interface{}{},
		"pattern_violations":        []interface{}{},
	}
	trendData, err := json.Marshal(processTrend)
	if err != nil {
		t.Fatalf("failed to marshal process trend: %v", err)
	}
	if err := os.WriteFile(filepath.Join(metricsDir, "process_trend.json"), trendData, 0644); err != nil {
		t.Fatalf("failed to write process_trend.json: %v", err)
	}

	t.Chdir(tmpDir)

	stdout, stderr, exitCode := runGromitCobra(t, "status", "--spc")
	if exitCode != 0 {
		t.Fatalf("status --spc exit %d, stderr: %s", exitCode, stderr)
	}

	// Should contain SPC data, not the placeholder
	if strings.Contains(stdout, "SPC dashboard is not yet implemented") {
		t.Errorf("expected SPC formatter to replace placeholder, got:\n%s", stdout)
	}

	// Should contain SPC section with metrics
	if !strings.Contains(stdout, "SPC:") {
		t.Errorf("expected 'SPC:' section in output, got:\n%s", stdout)
	}

	// Should contain at least one control limit metric
	expectedMetrics := []string{"Success:", "Escalate:", "Quality:", "Duration:"}
	foundAny := false
	for _, metric := range expectedMetrics {
		if strings.Contains(stdout, metric) {
			foundAny = true
			break
		}
	}
	if !foundAny {
		t.Errorf("expected at least one SPC metric label in output, got:\n%s", stdout)
	}

	// Should NOT contain default sections
	for _, section := range []string{"Run:", "Pipeline:", "Health:"} {
		if strings.Contains(stdout, section) {
			t.Errorf("expected SPC guard path to skip %q section, got:\n%s", section, stdout)
		}
	}
}
