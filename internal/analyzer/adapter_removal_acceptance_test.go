//go:build acceptance

package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClaudeClientAdapterRemoved verifies that the claudeClientAdapter type
// and NewClaudeClientAdapter constructor have been removed from analyzer.go
// after all callers migrated to using provider.Provider directly.
// Expected failure: claudeClientAdapter type still exists in analyzer.go
func TestClaudeClientAdapterRemoved(t *testing.T) {
	analyzerPath := filepath.Join(".", "analyzer.go")
	source, err := os.ReadFile(analyzerPath)
	if err != nil {
		t.Fatalf("could not read analyzer.go: %v", err)
	}

	sourceStr := string(source)

	// Should NOT contain claudeClientAdapter type
	if strings.Contains(sourceStr, "type claudeClientAdapter struct") {
		t.Error("analyzer.go should not define claudeClientAdapter type - all callers should use provider.Provider directly")
	}

	// Should NOT contain NewClaudeClientAdapter constructor
	if strings.Contains(sourceStr, "func NewClaudeClientAdapter") {
		t.Error("analyzer.go should not define NewClaudeClientAdapter - all callers should use provider.Provider directly")
	}

	// Analyzer should still accept ProviderRunner interface
	if !strings.Contains(sourceStr, "type ProviderRunner interface") {
		t.Error("analyzer.go should still define ProviderRunner interface")
	}

	// NewAnalyzer should still exist and accept ProviderRunner
	if !strings.Contains(sourceStr, "func NewAnalyzer(p ProviderRunner") {
		t.Error("analyzer.go should still have NewAnalyzer function accepting ProviderRunner")
	}
}

// TestAnalyzerTestsUpdated verifies that analyzer_test.go no longer uses
// NewClaudeClientAdapter in its test setup.
// Expected failure: analyzer_test.go still calls NewClaudeClientAdapter in tests
func TestAnalyzerTestsUpdated(t *testing.T) {
	testPath := filepath.Join(".", "analyzer_test.go")
	source, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("could not read analyzer_test.go: %v", err)
	}

	sourceStr := string(source)

	// Should NOT call NewClaudeClientAdapter in tests
	if strings.Contains(sourceStr, "NewClaudeClientAdapter") {
		t.Error("analyzer_test.go should not call NewClaudeClientAdapter - tests should use mock ProviderRunner instead")
	}

	// Tests should use mock ProviderRunner implementations or real provider.Provider instances
	// We don't enforce what mock pattern is used, just that NewClaudeClientAdapter is not used
}
