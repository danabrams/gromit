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
