package main

import (
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
