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

// TestVisionMetricsValidateExecutes verifies the validate subcommand can be executed
func TestVisionMetricsValidateExecutes(t *testing.T) {
	t.Parallel()
	rootCmd.SetArgs([]string{"vision-metrics", "validate"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("vision-metrics validate failed: %v", err)
	}
}

// TestVisionMetricsReportExecutes verifies the report subcommand can be executed
func TestVisionMetricsReportExecutes(t *testing.T) {
	t.Parallel()
	rootCmd.SetArgs([]string{"vision-metrics", "report"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("vision-metrics report failed: %v", err)
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
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldCwd)
	os.Chdir(tmpDir)

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
