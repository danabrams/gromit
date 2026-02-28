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
