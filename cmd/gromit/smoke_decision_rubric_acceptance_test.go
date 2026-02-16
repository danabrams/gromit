//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSmokeCoverageMatrix_RubricDefinesCriticalOutcomesOnly(t *testing.T) {
	// Expected failure: SmokeDecisionRubric section (and LoadSmokeDecisionRubric) does not exist yet.
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
	if !strings.Contains(content, "smoke decision rubric") && !strings.Contains(content, "keep vs move rubric") {
		t.Fatalf("expected rubric section header (e.g., 'Smoke Decision Rubric' or 'Keep vs Move Rubric')")
	}

	if !strings.Contains(content, "critical success") || !strings.Contains(content, "critical failure") {
		t.Fatalf("expected rubric to define critical success/failure outcomes")
	}

	if !strings.Contains(content, "only") {
		t.Fatalf("expected rubric to state that only critical success/failure outcomes are retained")
	}
}

func TestSmokeCoverageMatrix_MatrixTemplateHasRequiredFields(t *testing.T) {
	// Expected failure: MatrixTemplate section (and RenderSmokeMatrixTemplate) does not exist yet.
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

	re := regexp.MustCompile(`(?i)\|\s*source case\s*\|\s*keep/move\s*\|\s*rationale\s*\|\s*destination suite/file\s*\|`)
	if !re.MatchString(content) {
		t.Fatalf("expected matrix template header with required fields: source case, keep/move, rationale, destination suite/file")
	}
}

func TestSmokeCoverageMatrix_DestinationConventionsForCmdAndRunner(t *testing.T) {
	// Expected failure: DestinationConventions section (and ResolveDestinationConvention) does not exist yet.
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
	if !strings.Contains(content, "destination conventions") {
		t.Fatalf("expected a 'Destination Conventions' section")
	}

	if !strings.Contains(content, "cmd/gromit") {
		t.Fatalf("expected destination conventions to mention cmd/gromit suite paths")
	}

	if !strings.Contains(content, "internal/runner") {
		t.Fatalf("expected destination conventions to mention internal/runner suite paths")
	}

	if !strings.Contains(content, "file:testname") && !strings.Contains(content, "file:suite") {
		t.Fatalf("expected destination conventions to define file:testname or file:suite format")
	}
}
