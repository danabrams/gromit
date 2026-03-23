package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_AcceptedRunWithFindings_GeneratesRemediationSpec(t *testing.T) {
	// ── Seed ──────────────────────────────────────────────────────────────
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	specsDir := filepath.Join(tmp, "docs", "specs")

	store := runstore.NewStore(storeDir)

	run := &runstore.RunState{
		RunID:     "run-0004f-001",
		SpecID:    "0004f-contract-specificity-validation",
		ProjectID: "gromit",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 3, 23, 10, 30, 0, 0, time.UTC),
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Build review.json with 9 architecture_drift + 8 spec_alignment findings = 17 total
	archDriftFindings := make([]interface{}, 9)
	severities := []string{"warning", "info", "suggestion"}
	for i := 0; i < 9; i++ {
		archDriftFindings[i] = map[string]interface{}{
			"file":          "internal/next/specloop/stages/build.go",
			"line":          float64(10 + i*10),
			"severity":      severities[i%3],
			"facet":         archDriftFacet(i),
			"description":   archDriftDesc(i),
			"suggested_fix": archDriftFix(i),
		}
	}

	specAlignFindings := make([]interface{}, 8)
	for i := 0; i < 8; i++ {
		specAlignFindings[i] = map[string]interface{}{
			"file":          "internal/next/specloop/stages/validate.go",
			"line":          float64(20 + i*10),
			"severity":      severities[i%3],
			"facet":         specAlignFacet(i),
			"description":   specAlignDesc(i),
			"suggested_fix": specAlignFix(i),
		}
	}

	reviewJSON := map[string]interface{}{
		"architecture_drift": archDriftFindings,
		"spec_alignment":     specAlignFindings,
	}

	evidenceDir := store.RunEvidenceDir("run-0004f-001")
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
	specPath, err := maybeGenerateRemediationSpec("run-0004f-001", storeDir, specsDir)
	if err != nil {
		t.Fatalf("maybeGenerateRemediationSpec: %v", err)
	}

	// ── Assert ────────────────────────────────────────────────────────────

	// File path is returned and correct
	expectedPath := filepath.Join(specsDir, "0004f-contract-specificity-validation-remediation.md")
	if specPath != expectedPath {
		t.Errorf("path: want %q, got %q", expectedPath, specPath)
	}

	// File exists on disk
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("spec file missing: %v", err)
	}

	content, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := string(content)

	// spec_id is correct
	if !strings.Contains(spec, "## spec_id") {
		t.Error("spec_id header missing or wrong")
	}
	if !strings.Contains(spec, "0004f-contract-specificity-validation-remediation") {
		t.Error("spec_id value missing or wrong")
	}

	// Depends on points to parent spec
	if !strings.Contains(spec, "## Depends on") {
		t.Error("Depends on header missing or wrong")
	}
	if !strings.Contains(spec, "0004f-contract-specificity-validation") {
		t.Error("Depends on value missing or wrong")
	}

	// Summary mentions 17 findings and 2 categories
	if !strings.Contains(spec, "17 findings") {
		t.Errorf("summary should mention 17 findings, got:\n%s", spec)
	}
	if !strings.Contains(spec, "2 categories") {
		t.Errorf("summary should mention 2 categories, got:\n%s", spec)
	}

	// Acceptance Criteria section exists with exactly 17 numbered items
	if !strings.Contains(spec, "## Acceptance Criteria") {
		t.Error("Acceptance Criteria section missing")
	}
	for i := 1; i <= 17; i++ {
		prefix := strings.Replace("N. ", "N", strings.TrimSpace(strings.Repeat(" ", 0)+itoa(i)), 1)
		if !strings.Contains(spec, prefix) {
			t.Errorf("acceptance criterion %d missing", i)
		}
	}
	// No 18th criterion
	if strings.Contains(spec, "18. ") {
		t.Error("unexpected 18th acceptance criterion")
	}

	// Acceptance criteria should be file-path-prefixed in the format "N. [file] description"
	// Check that criteria are prefixed with file paths in brackets
	if !strings.Contains(spec, "[internal/next/specloop/stages/build.go]") {
		t.Error("acceptance criteria missing file path prefix for build.go")
	}
	if !strings.Contains(spec, "[internal/next/specloop/stages/validate.go]") {
		t.Error("acceptance criteria missing file path prefix for validate.go")
	}

	// All 9 architecture_drift findings present
	for i := 0; i < 9; i++ {
		if !strings.Contains(spec, archDriftFacet(i)) {
			t.Errorf("architecture_drift finding %d facet missing: %s", i, archDriftFacet(i))
		}
	}

	// All 8 spec_alignment findings present
	for i := 0; i < 8; i++ {
		if !strings.Contains(spec, specAlignFacet(i)) {
			t.Errorf("spec_alignment finding %d facet missing: %s", i, specAlignFacet(i))
		}
	}

	// Validation section should contain go test and go vet commands
	if !strings.Contains(spec, "## Validation") {
		t.Error("Validation section missing")
	}
	if !strings.Contains(spec, "go test ./... -count=1") {
		t.Error("Validation section missing 'go test ./... -count=1'")
	}
	if !strings.Contains(spec, "go vet ./...") {
		t.Error("Validation section missing 'go vet ./...'")
	}

	// stdout would contain the file path (tested via specPath return value above)
}

// ── Helpers ──────────────────────────────────────────────────────────────

func archDriftFacet(i int) string {
	facets := []string{
		"Package boundary violation",
		"Cyclic import risk",
		"Wrong abstraction layer",
		"Leaking internal type",
		"Missing interface boundary",
		"Hardcoded dependency",
		"Unused export",
		"Cross-layer call",
		"Config in logic package",
	}
	return facets[i]
}

func archDriftDesc(i int) string {
	descs := []string{
		"Package X directly imports package Y internal type",
		"Import cycle between runner and config detected",
		"Business logic mixed into transport layer",
		"Internal struct exposed via public API",
		"No interface at package boundary for testing",
		"Direct dependency on concrete type instead of interface",
		"Exported function not used outside package",
		"Handler calls store directly instead of service",
		"Config parsing embedded in business logic",
	}
	return descs[i]
}

func archDriftFix(i int) string {
	fixes := []string{
		"Move shared type to a common package",
		"Extract shared interface to break cycle",
		"Move logic to domain package",
		"Make struct unexported and add constructor",
		"Define interface at package boundary",
		"Accept interface parameter instead",
		"Make function unexported",
		"Route through service layer",
		"Extract config to dedicated config loader",
	}
	return fixes[i]
}

func specAlignFacet(i int) string {
	facets := []string{
		"Missing acceptance criterion",
		"Divergent naming convention",
		"Unimplemented constraint",
		"Extra behavior not in spec",
		"Wrong error code",
		"Missing validation rule",
		"Inconsistent return type",
		"Spec timeout not enforced",
	}
	return facets[i]
}

func specAlignDesc(i int) string {
	descs := []string{
		"Spec requires idempotency check but none implemented",
		"Spec uses snake_case but code uses camelCase for field names",
		"Spec constraint on max retries not enforced in code",
		"Code adds caching behavior not mentioned in spec",
		"Spec says 404 but code returns 400 on missing resource",
		"Spec requires email format validation but none present",
		"Spec expects []string but code returns string",
		"Spec 30s timeout not configured in HTTP client",
	}
	return descs[i]
}

func specAlignFix(i int) string {
	fixes := []string{
		"Add idempotency key check before processing",
		"Rename fields to match spec convention",
		"Add max retry guard in loop",
		"Remove caching or add to spec",
		"Return 404 for missing resource",
		"Add email format validation",
		"Change return type to []string",
		"Set HTTP client timeout to 30s",
	}
	return fixes[i]
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
