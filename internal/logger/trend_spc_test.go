package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrendSPCDefinitionsFileExists(t *testing.T) {
	target := filepath.Clean(filepath.Join("..", "..", "internal", "logger", "trend_spc.go"))
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", target, err)
	}
	if !strings.Contains(string(data), "metricSeriesDefinition") {
		t.Fatalf("%s does not declare metricSeriesDefinition", target)
	}
}

func TestQuarantineAttribution_BothBlank(t *testing.T) {
	m := IterationMetric{Model: "", Provider: ""}
	if !isQuarantinedAttribution(m) {
		t.Fatal("expected quarantine when both model and provider are blank")
	}
}

func TestQuarantineAttribution_BothUnknown(t *testing.T) {
	m := IterationMetric{Model: "unknown", Provider: "unknown"}
	if !isQuarantinedAttribution(m) {
		t.Fatal("expected quarantine when both model and provider are 'unknown'")
	}
}

func TestQuarantineAttribution_ModelSetProviderBlank(t *testing.T) {
	m := IterationMetric{Model: "claude-sonnet-4-20250514", Provider: ""}
	if isQuarantinedAttribution(m) {
		t.Fatal("should NOT quarantine when model is set")
	}
}

func TestQuarantineAttribution_ProviderSetModelBlank(t *testing.T) {
	m := IterationMetric{Model: "", Provider: "anthropic"}
	if isQuarantinedAttribution(m) {
		t.Fatal("should NOT quarantine when provider is set")
	}
}

func TestQuarantineAttribution_UnknownModelRealProvider(t *testing.T) {
	m := IterationMetric{Model: "unknown", Provider: "openai"}
	if isQuarantinedAttribution(m) {
		t.Fatal("should NOT quarantine when provider has a real value")
	}
}

func TestPartitionMetricsByStratum_ExcludesQuarantined(t *testing.T) {
	metrics := []IterationMetric{
		{Model: "claude-sonnet-4-20250514", Provider: "anthropic", CostUSD: 0.05, InputTokens: 100},
		{Model: "", Provider: "", CostUSD: 0, InputTokens: 0},        // quarantined
		{Model: "unknown", Provider: "", CostUSD: 0, InputTokens: 0}, // quarantined (resolves to unknown/unknown)
	}
	byStratum := partitionMetricsByStratum(metrics)

	// Only the first metric should appear in any stratum
	totalMetrics := 0
	for _, sm := range byStratum {
		totalMetrics += len(sm)
	}
	// One metric goes into provider:anthropic and model:claude-sonnet-4-20250514 = 2 entries total
	if totalMetrics != 2 {
		t.Fatalf("expected 2 stratum entries (one real metric in two strata), got %d", totalMetrics)
	}

	// Verify no "unknown" stratum exists
	for key := range byStratum {
		if strings.Contains(key, "unknown") {
			t.Fatalf("quarantined metric created stratum %q", key)
		}
	}
}

func TestBuildStratifiedControlLimits_ExcludesDegenerateUnknownStrata(t *testing.T) {
	// Create metrics where some have real attribution and some are degenerate unknown
	// with zero cost and zero input tokens
	metrics := []IterationMetric{
		{Model: "claude-sonnet-4-20250514", Provider: "anthropic", CostUSD: 0.05, InputTokens: 500, DurationMs: 1000, Success: true},
		{Model: "claude-sonnet-4-20250514", Provider: "anthropic", CostUSD: 0.03, InputTokens: 300, DurationMs: 800, Success: true},
	}
	limits, anomalies := buildStratifiedControlLimits(metrics)

	// Should have strata for the real model/provider, none with "unknown"
	for key := range limits {
		if strings.Contains(key, "unknown") {
			t.Fatalf("degenerate unknown stratum %q should have been excluded from limits", key)
		}
	}
	for key := range anomalies {
		if strings.Contains(key, "unknown") {
			t.Fatalf("degenerate unknown stratum %q should have been excluded from anomalies", key)
		}
	}
}

func TestDetectAnomalySeverityClassification(t *testing.T) {
	limit := TrendControlLimit{
		Metric: "rolling_avg_validation_ms",
		Latest: 2.1,
		Mean:   2,
		StdDev: 1,
		UCL:    2,
		LCL:    0,
	}
	if anomaly, ok := detectAnomaly(limit); !ok || anomaly.Severity != anomalySeverityHigh {
		t.Fatalf("severity = %q, want %q when latest exceeds UCL", anomaly.Severity, anomalySeverityHigh)
	}
}
