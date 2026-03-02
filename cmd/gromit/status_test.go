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
	output := captureStatusOutput(t, func() {
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

	output := captureStatusOutput(t, func() {
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
		"generated_at":     time.Now().Format(time.RFC3339),
		"total_iterations": 10,
		"window_size":      5,
		"latest_window":    map[string]interface{}{},
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

func TestStatusCmd_RegressionAssertion_OutputIsConsistent(t *testing.T) {
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

	// Create status.json
	status := runner.Status{
		Running:   false,
		Iteration: 5,
		BeadID:    "test-bead-123",
		BeadTitle: "Test Feature Implementation",
		Model:     "sonnet",
		StartedAt: time.Now().Add(-10 * time.Minute),
		ElapsedS:  600,
	}
	statusData, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "status.json"), statusData, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	t.Chdir(tmpDir)

	// Run status command twice - should produce identical output
	output1 := captureStatusOutput(t, func() {
		rootCmd.SetArgs([]string{"status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status command failed: %v", err)
		}
	})

	// Reset command state
	rootCmd.ResetFlags()

	output2 := captureStatusOutput(t, func() {
		rootCmd.SetArgs([]string{"status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status command failed: %v", err)
		}
	})

	// Both outputs should be identical (no state mutation)
	if output1 != output2 {
		t.Errorf("status command output is not consistent across invocations:\nFirst:\n%s\n\nSecond:\n%s", output1, output2)
	}

	// Output should contain expected sections
	if !strings.Contains(output1, "Pipeline:") {
		t.Errorf("status output missing 'Pipeline:' section, got:\n%s", output1)
	}
	if !strings.Contains(output1, "5 iterations completed") {
		t.Errorf("status output missing iteration count, got:\n%s", output1)
	}
}

func TestStatusCmd_RealCoordinatorQueueSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}
	writeRealCoordinatorQueueFixture(t, gromitDir)

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Chdir(tmpDir)

	output := captureStatusOutput(t, func() {
		rootCmd.SetArgs([]string{"status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status command failed: %v", err)
		}
	})

	if !strings.Contains(output, "Integration Queue:") {
		t.Fatalf("missing Integration Queue section:\n%s", output)
	}
	if !strings.Contains(output, "Queue length: 12") {
		t.Fatalf("unexpected queue length:\n%s", output)
	}
	if !strings.Contains(output, "Blocked: 7") {
		t.Fatalf("unexpected blocked count:\n%s", output)
	}
	if !strings.Contains(output, "gromit/conflict-late") {
		t.Fatalf("missing conflict branch entry:\n%s", output)
	}
}

// TestStatusCmd_RegressionAssertion_ModelPerformanceAlwaysVisible ensures the status
// command prints the Model Performance section even when no stats exist, guarding
// the TUI layout against missing sections.
func TestStatusCmd_RegressionAssertion_ModelPerformanceAlwaysVisible(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Create minimal config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Chdir(tmpDir)

	output := captureStatusOutput(t, func() {
		rootCmd.SetArgs([]string{"status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status command failed: %v", err)
		}
	})

	if !strings.Contains(output, "Model Performance:") {
		t.Fatalf("expected Model Performance section in status output, got:\n%s", output)
	}
}

func TestStatusCmd_RegressionAssertion_TUISectionsUnchanged(t *testing.T) {
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

	status := runner.Status{
		Running:   false,
		Iteration: 7,
		BeadID:    "gromit-xyz",
		BeadTitle: "Ensure regression test",
		Model:     "sonnet",
		StartedAt: time.Now().Add(-20 * time.Minute),
		ElapsedS:  1200,
	}
	statusData, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "status.json"), statusData, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	t.Chdir(tmpDir)

	output := captureStatusOutput(t, func() {
		rootCmd.SetArgs([]string{"status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status command failed: %v", err)
		}
	})

	assertStatusTUISections(t, output)
}

func assertStatusTUISections(t *testing.T, output string) {
	requiredSections := []string{"Run:", "Pipeline:", "Health:", "Model Performance:"}
	for _, section := range requiredSections {
		if !strings.Contains(output, section) {
			t.Fatalf("status output missing %q section, got:\n%s", section, output)
		}
	}
}

// getKeys is a helper to extract keys from a map for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestStatusCmd_JSONFlagOutputsValidJSON verifies that --json flag produces
// valid JSON with the expected StatusJSON structure including Run, Pipeline,
// and IntegrationQueue sections.
func TestStatusCmd_JSONFlagOutputsValidJSON(t *testing.T) {
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

	// Create status.json with test data
	status := runner.Status{
		Running:   false,
		Iteration: 2,
		BeadID:    "test-bead-123",
		BeadTitle: "Test Feature",
		Model:     "haiku",
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

	// Execute status command with --json flag and capture output
	output := captureStatusOutput(t, func() {
		rootCmd.SetArgs([]string{"status", "--json", "--spc=false"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status --json command failed: %v", err)
		}
	})

	// Parse and validate JSON structure
	var statusJSON map[string]interface{}
	if err := json.Unmarshal([]byte(output), &statusJSON); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, output)
	}

	// Verify expected fields exist
	if _, hasRun := statusJSON["run"]; !hasRun {
		t.Errorf("JSON missing 'run' field, got: %v", statusJSON)
	}
	if _, hasPipeline := statusJSON["pipeline"]; !hasPipeline {
		t.Errorf("JSON missing 'pipeline' field, got: %v", statusJSON)
	}
	if _, hasQueue := statusJSON["integration_queue"]; !hasQueue {
		t.Errorf("JSON missing 'integration_queue' field, got: %v", statusJSON)
	}
}

// TestStatusCmd_JSONAndSPCFlagsAreMutuallyExclusive verifies that using both
// --json and --spc flags together produces an error.
func TestStatusCmd_JSONAndSPCFlagsAreMutuallyExclusive(t *testing.T) {
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

	t.Chdir(tmpDir)

	// Execute status command with both --json and --spc flags
	stdout, stderr, exitCode := runGromitCobra(t, "status", "--json", "--spc")

	// Should fail with non-zero exit code
	if exitCode == 0 {
		t.Fatalf("status --json --spc should fail, but succeeded with exit code 0")
	}

	// Error message (in either stdout or stderr) should mention mutually exclusive flags
	combinedOutput := stdout + stderr
	if !strings.Contains(combinedOutput, "mutually exclusive") {
		t.Errorf("expected error mentioning 'mutually exclusive', got stdout: %s\nstderr: %s", stdout, stderr)
	}
}

// TestStatusCmd_JSONIncludesIntegrationQueueData verifies that the JSON output
// includes integration queue data when queue entries exist.
func TestStatusCmd_JSONIncludesIntegrationQueueData(t *testing.T) {
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

	// Create integration queue with test entries
	entries := []map[string]interface{}{
		map[string]interface{}{
			"branch":     "feature/test-1",
			"state":      "ready",
			"lane":       "code_lane",
			"fifo_seq":   1,
			"created_at": "2026-02-28T00:00:00Z",
			"updated_at": "2026-02-28T00:01:00Z",
		},
		map[string]interface{}{
			"branch":     "feature/test-2",
			"state":      "integrating",
			"lane":       "code_lane",
			"fifo_seq":   2,
			"created_at": "2026-02-28T00:02:00Z",
			"updated_at": "2026-02-28T00:03:00Z",
		},
	}
	queueData := map[string]interface{}{
		"schema_version": 1,
		"updated_at":     "2026-02-28T00:00:00Z",
		"entries":        entries,
	}
	queueBytes, err := json.Marshal(queueData)
	if err != nil {
		t.Fatalf("failed to marshal queue data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "integration-queue.json"), queueBytes, 0644); err != nil {
		t.Fatalf("failed to write integration-queue.json: %v", err)
	}

	t.Chdir(tmpDir)

	// Execute status command with --json flag
	output := captureStatusOutput(t, func() {
		rootCmd.SetArgs([]string{"status", "--json"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status --json command failed: %v", err)
		}
	})

	// Parse JSON output
	var statusJSON map[string]interface{}
	if err := json.Unmarshal([]byte(output), &statusJSON); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, output)
	}

	// Verify integration_queue field exists and has queue_length
	queueInterface, hasQueue := statusJSON["integration_queue"]
	if !hasQueue {
		t.Fatalf("JSON missing 'integration_queue' field")
	}

	queueObj, ok := queueInterface.(map[string]interface{})
	if !ok {
		t.Fatalf("integration_queue is not an object, got type: %T", queueInterface)
	}

	// Verify queue_length field
	queueLength, hasLength := queueObj["queue_length"]
	if !hasLength {
		t.Errorf("integration_queue missing 'queue_length' field, got keys: %v", getKeys(queueObj))
	}
	if queueLengthVal, ok := queueLength.(float64); ok {
		if queueLengthVal != 2.0 {
			t.Errorf("expected queue_length=2, got %v", queueLengthVal)
		}
	} else {
		t.Errorf("queue_length is not a number, got type: %T", queueLength)
	}

	// Verify Entries field
	_, hasEntries := queueObj["entries"]
	if !hasEntries {
		t.Errorf("integration_queue missing 'entries' field")
	}
}

func TestStatusCmd_JSONUsesSnakeCaseQueueFields(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	status := runner.Status{
		Running:   false,
		Iteration: 1,
		BeadID:    "snake-case-bead",
		BeadTitle: "JSON queue key test",
		Model:     "haiku",
		StartedAt: time.Now().Add(-time.Minute),
		ElapsedS:  60,
	}
	statusData, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "status.json"), statusData, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	entries := []map[string]interface{}{
		{
			"branch":         "feature/snake-case",
			"state":          "ready",
			"lane":           "code_lane",
			"fifo_seq":       1,
			"created_at":     "2026-02-28T00:00:00Z",
			"updated_at":     "2026-02-28T00:01:00Z",
			"session_id":     "session-snake",
			"origin_command": "gromit run",
			"base_ref":       "main",
			"head_sha":       "abc123",
		},
	}
	queueData := map[string]interface{}{
		"schema_version": 1,
		"updated_at":     "2026-02-28T00:00:00Z",
		"entries":        entries,
	}
	queueBytes, err := json.Marshal(queueData)
	if err != nil {
		t.Fatalf("failed to marshal queue data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "integration-queue.json"), queueBytes, 0644); err != nil {
		t.Fatalf("failed to write integration-queue.json: %v", err)
	}

	t.Chdir(tmpDir)

	output := captureStatusOutput(t, func() {
		rootCmd.SetArgs([]string{"status", "--json"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status --json command failed: %v", err)
		}
	})

	var statusJSON map[string]interface{}
	if err := json.Unmarshal([]byte(output), &statusJSON); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, output)
	}

	queueInterface, hasQueue := statusJSON["integration_queue"]
	if !hasQueue {
		t.Fatalf("JSON missing 'integration_queue' field")
	}
	queueObj, ok := queueInterface.(map[string]interface{})
	if !ok {
		t.Fatalf("integration_queue is not an object, got type: %T", queueInterface)
	}

	if _, hasLength := queueObj["queue_length"]; !hasLength {
		t.Fatalf("integration_queue missing 'queue_length' field, got keys: %v", getKeys(queueObj))
	}

	entriesInterface, hasEntries := queueObj["entries"]
	if !hasEntries {
		t.Fatalf("integration_queue missing 'entries' field, got keys: %v", getKeys(queueObj))
	}
	entriesSlice, ok := entriesInterface.([]interface{})
	if !ok {
		t.Fatalf("entries field is not an array, got type: %T", entriesInterface)
	}
	if len(entriesSlice) == 0 {
		t.Fatalf("entries array is empty, got output: %s", output)
	}
	firstEntry, ok := entriesSlice[0].(map[string]interface{})
	if !ok {
		t.Fatalf("entry is not an object, got type: %T", entriesSlice[0])
	}
	if _, hasBranch := firstEntry["branch"]; !hasBranch {
		t.Fatalf("entry missing 'branch' field, got keys: %v", getKeys(firstEntry))
	}
}

// captureStatusOutput is a helper for status command tests that captures stdout
// and properly resets statusCmd flags before execution.
func captureStatusOutput(t *testing.T, fn func()) string {
	t.Helper()

	// Reset statusCmd flags before execution
	if err := statusCmd.Flags().Set("spc", "false"); err != nil {
		t.Fatalf("failed to reset --spc flag: %v", err)
	}
	if err := statusCmd.Flags().Set("json", "false"); err != nil {
		t.Fatalf("failed to reset --json flag: %v", err)
	}

	// Use captureStdout from stats_test.go to handle stdout redirection
	return captureStdout(t, fn)
}

// TestStatusCmd_OutputIncludesIntegrationQueueSection verifies that the status
// command text output includes the Integration Queue section with queue length,
// per-state counts, and entry details.
func TestStatusCmd_OutputIncludesIntegrationQueueSection(t *testing.T) {
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

	// Create status.json with a stopped run
	status := runner.Status{
		Running:   false,
		Iteration: 2,
		BeadID:    "cmd-status-test",
		BeadTitle: "Integration queue section test",
		Model:     "haiku",
		StartedAt: time.Now().Add(-10 * time.Minute),
		ElapsedS:  600,
	}
	statusData, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "status.json"), statusData, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	// Create integration queue with multiple entries in different states
	entries := []map[string]interface{}{
		{
			"branch":                 "gromit/ready-feature",
			"state":                  "ready",
			"lane":                   "code_lane",
			"fifo_seq":               1,
			"created_at":             "2026-02-28T00:00:00Z",
			"updated_at":             "2026-02-28T00:01:00Z",
			"session_id":             "session-1",
			"origin_command":         "review",
			"base_ref":               "main",
			"head_sha":               "deadbeef",
			"changed_files_hash":     "sha256:abc",
			"last_error_code":        "",
			"last_error_message":     "",
			"last_transition_reason": "session_committed",
		},
		{
			"branch":                 "gromit/integrating-feature",
			"state":                  "integrating",
			"lane":                   "code_lane",
			"fifo_seq":               2,
			"created_at":             "2026-02-28T00:00:00Z",
			"updated_at":             "2026-02-28T00:02:00Z",
			"session_id":             "session-2",
			"origin_command":         "review",
			"base_ref":               "main",
			"head_sha":               "deadbeef2",
			"changed_files_hash":     "sha256:def",
			"last_error_code":        "",
			"last_error_message":     "",
			"last_transition_reason": "integration_started",
		},
		{
			"branch":                 "gromit/conflict-feature",
			"state":                  "conflict",
			"lane":                   "safe_lane",
			"fifo_seq":               3,
			"created_at":             "2026-02-28T00:00:00Z",
			"updated_at":             "2026-02-28T00:03:00Z",
			"session_id":             "session-3",
			"origin_command":         "review",
			"base_ref":               "main",
			"head_sha":               "deadbeef3",
			"changed_files_hash":     "sha256:ghi",
			"last_error_code":        "merge_conflict",
			"last_error_message":     "Conflict in main.go",
			"last_transition_reason": "merge_conflict_detected",
		},
	}
	queueData := map[string]interface{}{
		"schema_version": 1,
		"updated_at":     "2026-02-28T00:03:00Z",
		"entries":        entries,
	}
	queueBytes, err := json.Marshal(queueData)
	if err != nil {
		t.Fatalf("failed to marshal queue data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "integration-queue.json"), queueBytes, 0644); err != nil {
		t.Fatalf("failed to write integration-queue.json: %v", err)
	}

	t.Chdir(tmpDir)

	// Run status command without --json flag
	output := captureStatusOutput(t, func() {
		rootCmd.SetArgs([]string{"status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status command failed: %v", err)
		}
	})

	if len(strings.TrimSpace(output)) == 0 {
		t.Fatalf("status command produced empty output")
	}

	// Verify Integration Queue section exists
	requiredStrings := []string{
		"Integration Queue:",
		"Queue length: 3",
		"Ready: 1",
		"Integrating: 1",
		"Blocked: 1",
		"gromit/ready-feature",
		"gromit/integrating-feature",
		"gromit/conflict-feature",
		"Error: merge_conflict",
		"Conflict in main.go",
	}

	for _, required := range requiredStrings {
		if !strings.Contains(output, required) {
			t.Errorf("status command output missing %q; got:\n%s", required, output)
		}
	}
}
