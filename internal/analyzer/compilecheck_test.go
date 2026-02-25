package analyzer

import (
	"os"
	"strings"
	"testing"
)

func TestAnalyzerDefinesProviderRunnerCheck(t *testing.T) {
	data, err := os.ReadFile("analyzer.go")
	if err != nil {
		t.Fatalf("read analyzer.go: %v", err)
	}

	if !strings.Contains(string(data), "var _ ProviderRunner = (*claudeClientAdapter)(nil)") {
		t.Fatalf("analyzer.go must define compile-time check that claudeClientAdapter implements ProviderRunner")
	}
}
