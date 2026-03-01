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
