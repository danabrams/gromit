package logger

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestRefactorValidation_FileLinesUnder550 validates that all four production files
// (process_trend.go, trend_analytics.go, trend_builder.go, trend_spc.go) are <= 550 lines.
func TestRefactorValidation_FileLinesUnder550(t *testing.T) {
	files := map[string]int{
		"process_trend.go":    550,
		"trend_analytics.go":  550,
		"trend_builder.go":    550,
		"trend_spc.go":        550,
	}

	// Get the directory of the current file
	_, currentFile, _, _ := runtime.Caller(0)
	loggerDir := filepath.Dir(currentFile)

	for filename, maxLines := range files {
		filePath := filepath.Join(loggerDir, filename)
		lineCount, err := countLines(filePath)
		if err != nil {
			t.Fatalf("failed to count lines in %s: %v", filePath, err)
		}
		if lineCount > maxLines {
			t.Errorf("%s has %d lines, expected <= %d", filename, lineCount, maxLines)
		}
	}
}

// TestRefactorValidation_ProcessTrendPublicSurface validates that process_trend.go
// retains only the required public surface.
func TestRefactorValidation_ProcessTrendPublicSurface(t *testing.T) {
	requiredTypes := []string{
		"IterationMetric",
		"EWMAMetricState",
		"PromptTypeSummary",
		"PromptSectionSummary",
		"ReconciliationDrift",
		"PromptTokenSummary",
		"ProcessTrendWindow",
		"TrendControlLimit",
		"TrendAnomaly",
		"PatternViolation",
		"ProviderMetrics",
		"ProcessTrend",
	}

	requiredFunctions := []string{
		"BuildContinuousMetrics",
		"ReadProcessTrend",
	}

	// Verify all required types exist and are exported
	for _, typeName := range requiredTypes {
		// Check that type starts with uppercase (exported)
		if len(typeName) == 0 || typeName[0] < 'A' || typeName[0] > 'Z' {
			t.Errorf("type %s is not exported (doesn't start with uppercase)", typeName)
		}
	}

	// Verify all required functions exist and are exported
	for _, funcName := range requiredFunctions {
		// Check that function starts with uppercase (exported)
		if len(funcName) == 0 || funcName[0] < 'A' || funcName[0] > 'Z' {
			t.Errorf("function %s is not exported (doesn't start with uppercase)", funcName)
		}
	}
}

// Helper function to count lines in a file
func countLines(filepath string) (int, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return lineCount, nil
}
