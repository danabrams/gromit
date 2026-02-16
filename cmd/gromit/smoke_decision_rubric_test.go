package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSmokeDecisionRubric_DefinesCriticalOutcomesOnly(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	matrixPath := filepath.Join(projectRoot, "docs", "smoke_coverage_matrix.md")
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("read smoke coverage matrix: %v", err)
	}

	content := strings.ToLower(string(data))
	if !strings.Contains(content, "smoke decision rubric") {
		t.Fatalf("expected 'Smoke Decision Rubric' section header")
	}
	if !strings.Contains(content, "critical success") || !strings.Contains(content, "critical failure") {
		t.Fatalf("expected rubric to define critical success/failure outcomes")
	}
	if !strings.Contains(content, "only") {
		t.Fatalf("expected rubric to state that only critical success/failure outcomes are retained")
	}
}

func TestSmokeDecisionRubric_MatrixTemplateHasRequiredFields(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	matrixPath := filepath.Join(projectRoot, "docs", "smoke_coverage_matrix.md")
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("read smoke coverage matrix: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Matrix Template") {
		t.Fatalf("expected a 'Matrix Template' section")
	}

	lower := strings.ToLower(content)
	requiredFields := []string{"source case", "keep/move", "rationale", "destination suite/file"}
	for _, field := range requiredFields {
		if !strings.Contains(lower, field) {
			t.Fatalf("expected matrix template to include required field %q", field)
		}
	}
}
