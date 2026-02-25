package logger

import (
	"os"
	"strings"
	"testing"
)

func TestTrendSPCDefinitionsFileExists(t *testing.T) {
	data, err := os.ReadFile("internal/logger/trend_spc.go")
	if err != nil {
		t.Fatalf("expected internal/logger/trend_spc.go to exist: %v", err)
	}
	if !strings.Contains(string(data), "metricSeriesDefinition") {
		t.Fatalf("internal/logger/trend_spc.go does not declare metricSeriesDefinition")
	}
}
