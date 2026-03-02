package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestVisionMetricsCommandRegistered verifies the vision-metrics command group is registered to root
func TestVisionMetricsCommandRegistered(t *testing.T) {
	t.Parallel()
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "vision-metrics" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("vision-metrics command not registered to root")
	}
}

// TestVisionMetricsValidateSubcommand verifies the validate subcommand exists
func TestVisionMetricsValidateSubcommand(t *testing.T) {
	t.Parallel()
	found := false
	visionMetricsCmd, _, err := rootCmd.Find([]string{"vision-metrics"})
	if err == nil && visionMetricsCmd != nil {
		for _, cmd := range visionMetricsCmd.Commands() {
			if cmd.Name() == "validate" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("validate subcommand not registered to vision-metrics")
	}
}

// TestVisionMetricsReportSubcommand verifies the report subcommand exists
func TestVisionMetricsReportSubcommand(t *testing.T) {
	t.Parallel()
	found := false
	visionMetricsCmd, _, err := rootCmd.Find([]string{"vision-metrics"})
	if err == nil && visionMetricsCmd != nil {
		for _, cmd := range visionMetricsCmd.Commands() {
			if cmd.Name() == "report" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("report subcommand not registered to vision-metrics")
	}
}

// TestVisionMetricsReportHasJSONFlag verifies the report subcommand has --json flag
func TestVisionMetricsReportHasJSONFlag(t *testing.T) {
	t.Parallel()
	reportCmd, _, err := rootCmd.Find([]string{"vision-metrics", "report"})
	if err != nil {
		t.Fatalf("failed to find report command: %v", err)
	}
	if reportCmd == nil {
		t.Fatal("report command not found")
	}

	jsonFlag := reportCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Fatal("--json flag not found on report subcommand")
	}
}

// TestVisionMetricsValidateLoadsAndValidatesRecords verifies validate loads records and reports errors
func TestVisionMetricsValidateLoadsAndValidatesRecords(t *testing.T) {
	tmpDir := t.TempDir()
	recordsPath := filepath.Join(tmpDir, "records.jsonl")

	// Create a test records file with an invalid record (missing spec_id)
	recordsContent := `{"spec_id":"","cycle_start_trigger_at":"2024-01-01T00:00:00Z","cycle_end_presented_at":"2024-01-02T00:00:00Z","review_outcome":"accepted","human_tactical_intervention":"no","human_debugging_intervention":"no","escaped_regression_within_7d":"no"}
{"spec_id":"valid-spec","cycle_start_trigger_at":"2024-01-01T00:00:00Z","cycle_end_presented_at":"2024-01-02T00:00:00Z","review_outcome":"accepted","human_tactical_intervention":"no","human_debugging_intervention":"no","escaped_regression_within_7d":"no"}`

	if err := os.WriteFile(recordsPath, []byte(recordsContent), 0644); err != nil {
		t.Fatalf("failed to write test records file: %v", err)
	}

	// Change to temp directory so validate can find the records file
	t.Chdir(tmpDir)

	// Create minimal gromit.yaml
	configContent := "paths:\n  gromit_dir: " + tmpDir + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "gromit.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Run validate with records path
	rootCmd.SetArgs([]string{"vision-metrics", "validate", recordsPath})
	output := captureStdout(t, func() {
		_ = rootCmd.Execute()
	})

	// Check that validation error is reported in output
	if output == "" {
		t.Error("validate command produced no output")
	}
}

func TestVisionMetricsValidateReturnsErrorOnInvalidRecords(t *testing.T) {
	tmpDir := t.TempDir()
	recordsPath := filepath.Join(tmpDir, "records.jsonl")

	recordsContent := `{"spec_id":"","cycle_start_trigger_at":"2024-01-01T00:00:00Z","cycle_end_presented_at":"2024-01-02T00:00:00Z","review_outcome":"accepted","human_tactical_intervention":"no","human_debugging_intervention":"no","escaped_regression_within_7d":"no"}`

	if err := os.WriteFile(recordsPath, []byte(recordsContent), 0644); err != nil {
		t.Fatalf("failed to write test records file: %v", err)
	}

	t.Chdir(tmpDir)

	configContent := "paths:\n  gromit_dir: " + tmpDir + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "gromit.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	rootCmd.SetArgs([]string{"vision-metrics", "validate", recordsPath})
	defer rootCmd.SetArgs(nil)

	var execErr error
	output := captureStdout(t, func() {
		execErr = rootCmd.Execute()
	})

	if execErr == nil {
		t.Fatal("expected validate command to fail when records are invalid")
	}

	if !contains(output, "Record 0") {
		t.Fatalf("expected output to mention invalid record, got: %q", output)
	}
}

// TestVisionMetricsReportOutputsKPIRollup verifies report outputs KPI metrics in text and JSON formats
func TestVisionMetricsReportOutputsKPIRollup(t *testing.T) {
	tmpDir := t.TempDir()
	recordsPath := filepath.Join(tmpDir, "records.jsonl")

	// Create a test records file with valid records
	recordsContent := `{"spec_id":"spec1","cycle_start_trigger_at":"2024-01-01T00:00:00Z","cycle_end_presented_at":"2024-01-02T00:00:00Z","review_outcome":"accepted","human_tactical_intervention":"no","human_debugging_intervention":"no","escaped_regression_within_7d":"no"}
{"spec_id":"spec2","cycle_start_trigger_at":"2024-01-03T00:00:00Z","cycle_end_presented_at":"2024-01-04T00:00:00Z","review_outcome":"accepted","human_tactical_intervention":"yes","human_debugging_intervention":"no","escaped_regression_within_7d":"no"}`

	if err := os.WriteFile(recordsPath, []byte(recordsContent), 0644); err != nil {
		t.Fatalf("failed to write test records file: %v", err)
	}

	// Change to temp directory
	t.Chdir(tmpDir)

	// Create minimal gromit.yaml
	configContent := "paths:\n  gromit_dir: " + tmpDir + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "gromit.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Run report in text format
	rootCmd.SetArgs([]string{"vision-metrics", "report", recordsPath})
	textOutput := captureStdout(t, func() {
		_ = rootCmd.Execute()
	})

	// Verify text output contains KPI header
	if !contains(textOutput, "Vision Metrics KPI Rollup") {
		t.Errorf("text output missing KPI header, got: %s", textOutput)
	}

	// Verify text output contains rate metrics
	if !contains(textOutput, "Pass Rate") && !contains(textOutput, "Integration") {
		t.Errorf("text output missing rate metrics, got: %s", textOutput)
	}

	// Run report in JSON format
	rootCmd.SetArgs([]string{"vision-metrics", "report", recordsPath, "--json"})
	jsonOutput := captureStdout(t, func() {
		_ = rootCmd.Execute()
	})

	// Verify JSON output contains expected fields
	if !contains(jsonOutput, "human_tactical_intervention_rate") {
		t.Errorf("JSON output missing human_tactical_intervention_rate, got: %s", jsonOutput)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i < len(haystack); i++ {
		if haystack[i] == needle[0] && i+len(needle) <= len(haystack) && haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
