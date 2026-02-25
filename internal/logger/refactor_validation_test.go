package logger

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

	_, currentFile, _, _ := runtime.Caller(0)
	loggerDir := filepath.Dir(currentFile)
	processTrendPath := filepath.Join(loggerDir, "process_trend.go")

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, processTrendPath, nil, 0)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", processTrendPath, err)
	}

	actualTypes := make([]string, 0, len(requiredTypes))
	actualFunctions := make([]string, 0, len(requiredFunctions))
	for _, decl := range parsed.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !typeSpec.Name.IsExported() {
					continue
				}
				actualTypes = append(actualTypes, typeSpec.Name.Name)
			}
		case *ast.FuncDecl:
			if d.Recv != nil || !d.Name.IsExported() {
				continue
			}
			actualFunctions = append(actualFunctions, d.Name.Name)
		}
	}

	slices.Sort(actualTypes)
	slices.Sort(actualFunctions)
	slices.Sort(requiredTypes)
	slices.Sort(requiredFunctions)

	if !slices.Equal(requiredTypes, actualTypes) {
		t.Errorf("exported types in process_trend.go mismatch: got %v want %v", actualTypes, requiredTypes)
	}
	if !slices.Equal(requiredFunctions, actualFunctions) {
		t.Errorf("exported functions in process_trend.go mismatch: got %v want %v", actualFunctions, requiredFunctions)
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
