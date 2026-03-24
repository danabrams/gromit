package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ExistingRemediationSpecIsOverwritten(t *testing.T) {
	// ── Seed ──────────────────────────────────────────────────────────────
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	specsDir := filepath.Join(tmp, "docs", "specs")

	store := runstore.NewStore(storeDir)

	run := &runstore.RunState{
		RunID:     "run-overwrite-001",
		SpecID:    "my-spec",
		ProjectID: "gromit",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 23, 14, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 3, 23, 14, 30, 0, 0, time.UTC),
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Pre-create the specs directory and an existing remediation spec with 3 acceptance criteria
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}

	oldSpec := `## spec_id

my-spec-remediation

## Depends on

my-spec

## Summary

3 findings across 1 categories from prior run.

## Goals

- Fix old issues

## Acceptance Criteria

1. Old Finding A: Fix the first old thing
2. Old Finding B: Fix the second old thing
3. Old Finding C: Fix the third old thing

## Validation

- All identified issues in this remediation spec have been addressed in the code
- Automated tests pass with no new failures
- Code review confirms all suggested fixes are applied
`
	specPath := filepath.Join(specsDir, "my-spec-remediation.md")
	if err := os.WriteFile(specPath, []byte(oldSpec), 0o644); err != nil {
		t.Fatalf("write old spec: %v", err)
	}

	// Verify old file exists with 3 criteria
	oldContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read old spec: %v", err)
	}
	if !strings.Contains(string(oldContent), "Old Finding C") {
		t.Fatal("pre-existing spec missing expected old content")
	}

	// Create review.json with 5 new findings
	reviewJSON := map[string]interface{}{
		"security": []interface{}{
			map[string]interface{}{
				"file":          "handler.go",
				"line":          10,
				"severity":      "warning",
				"facet":         "SQL Injection",
				"description":   "Unparameterized query in handler",
				"suggested_fix": "Use parameterized queries",
			},
			map[string]interface{}{
				"file":          "handler.go",
				"line":          35,
				"severity":      "warning",
				"facet":         "XSS Vulnerability",
				"description":   "User input rendered without escaping",
				"suggested_fix": "Escape HTML output",
			},
		},
		"style": []interface{}{
			map[string]interface{}{
				"file":          "config.go",
				"line":          5,
				"severity":      "suggestion",
				"facet":         "Naming Convention",
				"description":   "Variable uses snake_case",
				"suggested_fix": "Rename to camelCase",
			},
			map[string]interface{}{
				"file":          "config.go",
				"line":          20,
				"severity":      "info",
				"facet":         "Dead Code",
				"description":   "Unused function left in codebase",
				"suggested_fix": "Remove unused function",
			},
		},
		"performance": []interface{}{
			map[string]interface{}{
				"file":          "db.go",
				"line":          50,
				"severity":      "warning",
				"facet":         "N+1 Query",
				"description":   "Loop issues individual queries",
				"suggested_fix": "Batch into single query",
			},
		},
	}

	evidenceDir := store.RunEvidenceDir("run-overwrite-001")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}
	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("marshal review.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// ── Invoke ────────────────────────────────────────────────────────────
	resultPath, err := maybeGenerateRemediationSpec("run-overwrite-001", storeDir, specsDir)
	if err != nil {
		t.Fatalf("maybeGenerateRemediationSpec: %v", err)
	}

	// ── Assert ────────────────────────────────────────────────────────────

	// File path is returned and correct
	expectedPath := filepath.Join(specsDir, "my-spec-remediation.md")
	if resultPath != expectedPath {
		t.Errorf("path: want %q, got %q", expectedPath, resultPath)
	}

	// File exists on disk
	if _, err := os.Stat(resultPath); err != nil {
		t.Fatalf("spec file missing: %v", err)
	}

	newContent, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := string(newContent)

	// Old content is gone
	if strings.Contains(spec, "Old Finding A") {
		t.Error("old acceptance criterion A should be overwritten")
	}
	if strings.Contains(spec, "Old Finding B") {
		t.Error("old acceptance criterion B should be overwritten")
	}
	if strings.Contains(spec, "Old Finding C") {
		t.Error("old acceptance criterion C should be overwritten")
	}
	if strings.Contains(spec, "Fix the first old thing") {
		t.Error("old suggested fix text should be overwritten")
	}

	// New spec_id header is correct
	if !strings.Contains(spec, "## spec_id") {
		t.Error("new spec_id header missing or wrong")
	}
	if !strings.Contains(spec, "my-spec-remediation") {
		t.Error("new spec_id value missing or wrong")
	}

	// Depends on points to parent spec
	if !strings.Contains(spec, "## Depends on") {
		t.Error("Depends on header missing or wrong")
	}
	if !strings.Contains(spec, "my-spec") {
		t.Error("Depends on value missing or wrong")
	}

	// Summary mentions 5 findings and 3 categories
	if !strings.Contains(spec, "5 findings") {
		t.Errorf("summary should mention 5 findings, got:\n%s", spec)
	}
	if !strings.Contains(spec, "3 categories") {
		t.Errorf("summary should mention 3 categories, got:\n%s", spec)
	}

	// All 5 new findings are present
	for _, facet := range []string{"SQL Injection", "XSS Vulnerability", "Naming Convention", "Dead Code", "N+1 Query"} {
		if !strings.Contains(spec, facet) {
			t.Errorf("new finding %q missing from overwritten spec", facet)
		}
	}

	// Exactly 5 acceptance criteria
	if !strings.Contains(spec, "## Acceptance Criteria") {
		t.Error("Acceptance Criteria section missing")
	}
	for i := 1; i <= 5; i++ {
		prefix := strings.Replace("N. ", "N", itoa(i), 1)
		if !strings.Contains(spec, prefix) {
			t.Errorf("acceptance criterion %d missing", i)
		}
	}
	if strings.Contains(spec, "6. ") {
		t.Error("unexpected 6th acceptance criterion")
	}

	// Suggested fixes for new findings appear in criteria
	for _, fix := range []string{
		"Use parameterized queries",
		"Escape HTML output",
		"Rename to camelCase",
		"Remove unused function",
		"Batch into single query",
	} {
		if !strings.Contains(spec, fix) {
			t.Errorf("suggested fix %q missing from acceptance criteria", fix)
		}
	}
}
