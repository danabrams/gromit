package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestGenerateRemediationSpec_PureFunction(t *testing.T) {
	// Create a RemediationInput with multiple categories and findings
	input := RemediationInput{
		SpecID:    "spec-0001",
		Summary:   "Fix security and style issues",
		Goals:     []string{"Eliminate security vulnerabilities", "Improve code style"},
		NonGoals:  []string{"No behavioral changes"},
		DependsOn: "spec-0000",
		Findings: map[string][]RemediationFinding{
			"security": {
				{
					File:         "main.go",
					Line:         42,
					Category:     "security",
					Type:         "SQL Injection",
					Description:  "User input is not properly escaped in database query",
					SuggestedFix: "Use parameterized queries instead of string concatenation",
				},
				{
					File:         "utils.go",
					Line:         15,
					Category:     "security",
					Type:         "Missing Validation",
					Description:  "Function does not validate input bounds",
					SuggestedFix: "Add bounds check before accessing array",
				},
			},
			"style": {
				{
					File:         "main.go",
					Line:         87,
					Category:     "style",
					Type:         "Naming Convention",
					Description:  "Variable name should be camelCase",
					SuggestedFix: "Rename variable from snake_case to camelCase",
				},
				{
					File:         "utils.go",
					Category:     "style",
					Type:         "Comment Quality",
					Description:  "Function comment is unclear",
					SuggestedFix: "", // No suggested fix; should fall back to description
				},
			},
		},
	}

	output := generateRemediationSpec(input)

	// Verify spec_id header is present
	if !strings.Contains(output, "## spec_id") {
		t.Errorf("spec_id header missing: expected '## spec_id'")
	}
	if !strings.Contains(output, "spec-0001") {
		t.Errorf("spec ID missing: expected 'spec-0001' in output")
	}

	// Verify Depends on section
	if !strings.Contains(output, "## Depends on") {
		t.Errorf("Depends on section missing: expected '## Depends on'")
	}
	if !strings.Contains(output, "spec-0000") {
		t.Errorf("Depends on value missing: expected 'spec-0000' in output")
	}

	// Verify Summary section with counts
	if !strings.Contains(output, "## Summary") {
		t.Errorf("Summary section header missing")
	}
	if !strings.Contains(output, "Fix security and style issues") {
		t.Errorf("Summary content missing")
	}

	// Verify Goals section
	if !strings.Contains(output, "## Goals") {
		t.Errorf("Goals section header missing")
	}
	if !strings.Contains(output, "- Eliminate security vulnerabilities") {
		t.Errorf("Goals content missing")
	}

	// Verify Non-goals section
	if !strings.Contains(output, "## Non-goals") {
		t.Errorf("Non-goals section header missing")
	}
	if !strings.Contains(output, "- No behavioral changes") {
		t.Errorf("Non-goals content missing")
	}

	// Verify Architecture section is grouped by file
	if !strings.Contains(output, "## Architecture") {
		t.Errorf("Architecture section header missing")
	}

	// Verify main.go findings are grouped together
	mainGoIndex := strings.Index(output, "### main.go")
	if mainGoIndex == -1 {
		t.Errorf("main.go subsection missing in Architecture")
	}

	// Verify utils.go findings are grouped together
	utilsGoIndex := strings.Index(output, "### utils.go")
	if utilsGoIndex == -1 {
		t.Errorf("utils.go subsection missing in Architecture")
	}

	// Verify files are sorted (main.go should come before utils.go)
	if mainGoIndex > utilsGoIndex {
		t.Errorf("Files are not sorted: main.go should come before utils.go")
	}

	// Verify individual findings in Architecture section
	if !strings.Contains(output, "**Line 42:** `SQL Injection` (security)") {
		t.Errorf("Finding at main.go line 42 not found correctly")
	}
	if !strings.Contains(output, "**Line 87:** `Naming Convention` (style)") {
		t.Errorf("Finding at main.go line 87 not found correctly")
	}
	if !strings.Contains(output, "**Line 15:** `Missing Validation` (security)") {
		t.Errorf("Finding at utils.go line 15 not found correctly")
	}

	// Verify finding without line number
	if !strings.Contains(output, "**Comment Quality** (style)") {
		t.Errorf("Finding without line number not found correctly")
	}

	// Verify Acceptance Criteria section (one per finding)
	if !strings.Contains(output, "## Acceptance Criteria") {
		t.Errorf("Acceptance Criteria section header missing")
	}

	// Verify one criterion per finding (4 findings = 4 criteria)
	// Criteria are ordered by category (security, then style) due to map iteration
	criteria := []string{
		"1. [main.go] Use parameterized queries instead of string concatenation",
		"2. [utils.go] Add bounds check before accessing array",
		"3. [main.go] Rename variable from snake_case to camelCase",
		"4. [utils.go] Function comment is unclear", // Falls back to description
	}

	for _, criterion := range criteria {
		if !strings.Contains(output, criterion) {
			t.Errorf("Acceptance criterion missing: %s", criterion)
		}
	}

	// Verify Validation section
	if !strings.Contains(output, "## Validation") {
		t.Errorf("Validation section header missing")
	}
	if !strings.Contains(output, "go test ./... -count=1") {
		t.Errorf("Validation item 1 missing")
	}
	if !strings.Contains(output, "go vet ./...") {
		t.Errorf("Validation item 2 missing")
	}
}

func TestGenerateRemediationSpec_UsesDescriptionWhenSuggestedFixEmpty(t *testing.T) {
	input := RemediationInput{
		SpecID:  "spec-0003",
		Summary: "Fix documentation issues",
		Goals:   []string{"Improve code comments"},
		Findings: map[string][]RemediationFinding{
			"docs": {
				{
					File:         "doc.go",
					Line:         10,
					Category:     "docs",
					Type:         "Missing Docstring",
					Description:  "Function needs proper documentation",
					SuggestedFix: "", // Empty suggested fix; should use description
				},
			},
		},
	}

	output := generateRemediationSpec(input)

	// Verify acceptance criterion uses description when suggested_fix is empty
	if !strings.Contains(output, "1. [doc.go] Function needs proper documentation") {
		t.Errorf("Acceptance criterion should use description when suggested_fix is empty, got:\n%s", output)
	}
}

func TestGenerateRemediationSpec_EmptyFindings(t *testing.T) {
	input := RemediationInput{
		SpecID:    "spec-0011",
		Summary:   "Clean up items",
		Goals:     []string{"Fix issues"},
		NonGoals:  []string{"No breaking changes"},
		DependsOn: "spec-0010",
		Findings:  map[string][]RemediationFinding{}, // Empty findings
	}

	output := generateRemediationSpec(input)

	// Verify spec_id header is present
	if !strings.Contains(output, "## spec_id") || !strings.Contains(output, "spec-0011") {
		t.Errorf("spec_id header missing in output with empty findings")
	}

	// Verify Depends on section is present
	if !strings.Contains(output, "## Depends on") || !strings.Contains(output, "spec-0010") {
		t.Errorf("Depends on section missing")
	}

	// Verify Summary section is present
	if !strings.Contains(output, "## Summary") {
		t.Errorf("Summary section missing")
	}
	if !strings.Contains(output, "Clean up items") {
		t.Errorf("Summary content missing")
	}

	// Verify Goals section is present
	if !strings.Contains(output, "## Goals") {
		t.Errorf("Goals section missing")
	}

	// Verify Non-goals section is present
	if !strings.Contains(output, "## Non-goals") {
		t.Errorf("Non-goals section missing")
	}

	// Verify Architecture section is NOT present (no findings)
	if strings.Contains(output, "## Architecture") {
		t.Errorf("Architecture section should not be present with empty findings")
	}

	// Verify Acceptance Criteria section is NOT present (no findings)
	if strings.Contains(output, "## Acceptance Criteria") {
		t.Errorf("Acceptance Criteria section should not be present with empty findings")
	}

	// Verify Validation section IS present (always included)
	if !strings.Contains(output, "## Validation") {
		t.Errorf("Validation section should always be present")
	}
	if !strings.Contains(output, "go test ./... -count=1") {
		t.Errorf("Validation content missing")
	}
	if !strings.Contains(output, "go vet ./...") {
		t.Errorf("Validation content missing")
	}
}

func TestMaybeGenerateRemediationSpec_AcceptedWithFindings(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")

	// Create a RunState with terminal status
	run := &runstore.RunState{
		RunID:     "run-test-001",
		SpecID:    "spec-0004",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	// Save run to store
	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Create review.json with non-blocking findings across multiple categories
	reviewJSON := map[string]interface{}{
		"security": []interface{}{
			map[string]interface{}{
				"file":          "auth.go",
				"line":          25,
				"severity":      "warning",
				"facet":         "Missing Input Validation",
				"description":   "User input not validated before database query",
				"suggested_fix": "Add input validation before query execution",
			},
			map[string]interface{}{
				"file":          "auth.go",
				"line":          42,
				"severity":      "info",
				"facet":         "Weak Hash Algorithm",
				"description":   "Uses MD5 instead of bcrypt",
				"suggested_fix": "Replace MD5 with bcrypt",
			},
		},
		"style": []interface{}{
			map[string]interface{}{
				"file":          "main.go",
				"line":          10,
				"severity":      "suggestion",
				"facet":         "Naming Convention",
				"description":   "Variable uses snake_case instead of camelCase",
				"suggested_fix": "Rename to camelCase",
			},
		},
		"performance": []interface{}{
			map[string]interface{}{
				"file":          "db.go",
				"severity":      "warning",
				"facet":         "N+1 Query Problem",
				"description":   "Loop contains individual database queries",
				"suggested_fix": "",
			},
		},
	}

	// Write review.json to run evidence directory
	evidenceDir := store.RunEvidenceDir("run-test-001")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("failed to create evidence directory: %v", err)
	}

	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal review.json: %v", err)
	}

	reviewPath := filepath.Join(evidenceDir, "review.json")
	if err := os.WriteFile(reviewPath, reviewData, 0o644); err != nil {
		t.Fatalf("failed to write review.json: %v", err)
	}

	// Call maybeGenerateRemediationSpec
	filePath, err := maybeGenerateRemediationSpec("run-test-001", storeDir, specsDir)
	if err != nil {
		t.Fatalf("maybeGenerateRemediationSpec failed: %v", err)
	}

	// Verify file path is returned
	if filePath == "" {
		t.Fatal("expected file path to be returned, got empty string")
	}

	// Verify file path is correct
	expectedPath := filepath.Join(specsDir, "spec-0004-remediation.md")
	if filePath != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, filePath)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("spec file does not exist at %q: %v", filePath, err)
	}

	// Read and verify file contents
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read spec file: %v", err)
	}

	specContent := string(content)

	// Verify correct spec_id
	if !strings.Contains(specContent, "## spec_id") || !strings.Contains(specContent, "spec-0004-remediation") {
		t.Error("spec_id header not found or incorrect")
	}

	// Verify Depends on section
	if !strings.Contains(specContent, "## Depends on") || !strings.Contains(specContent, "spec-0004") {
		t.Error("DependsOn section not found")
	}

	// Verify one acceptance criterion per finding (4 total findings)
	criteriaCount := strings.Count(specContent, "\n1. ")
	criteriaCount += strings.Count(specContent, "\n2. ")
	criteriaCount += strings.Count(specContent, "\n3. ")
	criteriaCount += strings.Count(specContent, "\n4. ")

	if criteriaCount < 4 {
		t.Errorf("expected at least 4 acceptance criteria, spec content:\n%s", specContent)
	}

	// Verify findings from security category
	if !strings.Contains(specContent, "Missing Input Validation") {
		t.Error("Missing Input Validation finding not found")
	}
	if !strings.Contains(specContent, "Weak Hash Algorithm") {
		t.Error("Weak Hash Algorithm finding not found")
	}

	// Verify finding from style category
	if !strings.Contains(specContent, "Naming Convention") {
		t.Error("Naming Convention finding not found")
	}

	// Verify finding from performance category
	if !strings.Contains(specContent, "N+1 Query Problem") {
		t.Error("N+1 Query Problem finding not found")
	}

	// Verify finding with empty suggested_fix uses description
	if !strings.Contains(specContent, "Loop contains individual database queries") {
		t.Error("N+1 Query description not found in criteria")
	}

	// Verify summary mentions correct counts
	if !strings.Contains(specContent, "4 findings") {
		t.Error("summary should mention 4 findings")
	}
	if !strings.Contains(specContent, "3 categories") {
		t.Errorf("summary should mention 3 categories (security, style, performance), got:\n%s", specContent)
	}
}

func TestMaybeGenerateRemediationSpec_NoFindings(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")

	// Create a RunState with terminal status
	run := &runstore.RunState{
		RunID:     "run-test-002",
		SpecID:    "spec-0005",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	// Save run to store
	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Create review.json with only metadata (no finding arrays)
	reviewJSON := map[string]interface{}{
		"diff_unavailable": false,
	}

	// Write review.json to run evidence directory
	evidenceDir := store.RunEvidenceDir("run-test-002")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("failed to create evidence directory: %v", err)
	}

	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal review.json: %v", err)
	}

	reviewPath := filepath.Join(evidenceDir, "review.json")
	if err := os.WriteFile(reviewPath, reviewData, 0o644); err != nil {
		t.Fatalf("failed to write review.json: %v", err)
	}

	// Call maybeGenerateRemediationSpec
	filePath, err := maybeGenerateRemediationSpec("run-test-002", storeDir, specsDir)

	// Verify no error is returned
	if err != nil {
		t.Fatalf("maybeGenerateRemediationSpec failed: %v", err)
	}

	// Verify empty string is returned
	if filePath != "" {
		t.Errorf("expected empty file path, got %q", filePath)
	}

	// Verify no file is created in specsDir
	entries, err := os.ReadDir(specsDir)
	if err == nil && len(entries) > 0 {
		t.Errorf("expected no files in specsDir, but found %d entries", len(entries))
	}
}

func TestMaybeGenerateRemediationSpec_MissingReviewJSON(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")

	// Create a RunState with terminal status
	run := &runstore.RunState{
		RunID:     "run-test-003",
		SpecID:    "spec-0006",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	// Save run to store
	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Create evidence directory but do NOT write review.json
	evidenceDir := store.RunEvidenceDir("run-test-003")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("failed to create evidence directory: %v", err)
	}

	// Call maybeGenerateRemediationSpec
	filePath, err := maybeGenerateRemediationSpec("run-test-003", storeDir, specsDir)

	// Verify an error is returned
	if err == nil {
		t.Fatal("expected maybeGenerateRemediationSpec to return an error, got nil")
	}

	// Verify error message mentions review.json
	if !strings.Contains(err.Error(), "review.json") {
		t.Errorf("expected error to mention review.json, got: %v", err)
	}

	// Verify empty string is returned for file path
	if filePath != "" {
		t.Errorf("expected empty file path on error, got %q", filePath)
	}

	// Verify no spec file is created
	entries, err := os.ReadDir(specsDir)
	if err == nil && len(entries) > 0 {
		t.Errorf("expected no files in specsDir, but found %d entries", len(entries))
	}
}

func TestMaybeGenerateRemediationSpec_MalformedJSON(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")

	// Create a RunState with terminal status
	run := &runstore.RunState{
		RunID:     "run-test-004",
		SpecID:    "spec-0007",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	// Save run to store
	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Create evidence directory
	evidenceDir := store.RunEvidenceDir("run-test-004")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("failed to create evidence directory: %v", err)
	}

	// Write malformed JSON to review.json
	malformedJSON := `{invalid json content here` // Missing closing brace and proper formatting
	reviewPath := filepath.Join(evidenceDir, "review.json")
	if err := os.WriteFile(reviewPath, []byte(malformedJSON), 0o644); err != nil {
		t.Fatalf("failed to write review.json: %v", err)
	}

	// Call maybeGenerateRemediationSpec
	filePath, err := maybeGenerateRemediationSpec("run-test-004", storeDir, specsDir)

	// Verify an error is returned
	if err == nil {
		t.Fatal("expected maybeGenerateRemediationSpec to return an error, got nil")
	}

	// Verify error message mentions review.json or parse
	errMsg := err.Error()
	if !strings.Contains(errMsg, "review.json") && !strings.Contains(errMsg, "parse") {
		t.Errorf("expected error to mention review.json or parse, got: %v", err)
	}

	// Verify empty string is returned for file path
	if filePath != "" {
		t.Errorf("expected empty file path on error, got %q", filePath)
	}

	// Verify no spec file is created
	entries, err := os.ReadDir(specsDir)
	if err == nil && len(entries) > 0 {
		t.Errorf("expected no files in specsDir, but found %d entries", len(entries))
	}
}

func TestMaybeGenerateRemediationSpec_OverwritesExisting(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")

	// Create a RunState with terminal status
	run := &runstore.RunState{
		RunID:     "run-test-005",
		SpecID:    "spec-0008",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	// Save run to store
	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Create specs directory and write pre-existing remediation spec file
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs directory: %v", err)
	}

	oldContent := "# Old Remediation Spec\n\nThis is the old content that should be overwritten."
	specPath := filepath.Join(specsDir, "spec-0008-remediation.md")
	if err := os.WriteFile(specPath, []byte(oldContent), 0o644); err != nil {
		t.Fatalf("failed to write pre-existing spec file: %v", err)
	}

	// Verify old file exists and has expected content
	existingContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed to read pre-existing spec file: %v", err)
	}
	if string(existingContent) != oldContent {
		t.Error("pre-existing spec file does not have expected old content")
	}

	// Create review.json with different findings
	reviewJSON := map[string]interface{}{
		"security": []interface{}{
			map[string]interface{}{
				"file":          "api.go",
				"line":          100,
				"severity":      "warning",
				"facet":         "CORS Misconfiguration",
				"description":   "CORS headers not properly restricted",
				"suggested_fix": "Add whitelist of allowed origins",
			},
		},
	}

	// Write review.json to run evidence directory
	evidenceDir := store.RunEvidenceDir("run-test-005")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("failed to create evidence directory: %v", err)
	}

	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal review.json: %v", err)
	}

	reviewPath := filepath.Join(evidenceDir, "review.json")
	if err := os.WriteFile(reviewPath, reviewData, 0o644); err != nil {
		t.Fatalf("failed to write review.json: %v", err)
	}

	// Call maybeGenerateRemediationSpec
	filePath, err := maybeGenerateRemediationSpec("run-test-005", storeDir, specsDir)
	if err != nil {
		t.Fatalf("maybeGenerateRemediationSpec failed: %v", err)
	}

	// Verify file path is returned
	if filePath == "" {
		t.Fatal("expected file path to be returned, got empty string")
	}

	// Verify file path is correct
	expectedPath := filepath.Join(specsDir, "spec-0008-remediation.md")
	if filePath != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, filePath)
	}

	// Read the new file content
	newContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed to read spec file after generation: %v", err)
	}

	newContentStr := string(newContent)

	// Verify old content is NOT present
	if strings.Contains(newContentStr, "This is the old content that should be overwritten") {
		t.Error("old content was not overwritten")
	}

	// Verify new content is present
	if !strings.Contains(newContentStr, "## spec_id") || !strings.Contains(newContentStr, "spec-0008-remediation") {
		t.Error("new spec_id header not found")
	}
	if !strings.Contains(newContentStr, "CORS Misconfiguration") {
		t.Error("new finding (CORS Misconfiguration) not found in overwritten file")
	}
	if !strings.Contains(newContentStr, "Add whitelist of allowed origins") {
		t.Error("new suggested fix not found in overwritten file")
	}

	// Verify summary counts match new findings
	if !strings.Contains(newContentStr, "1 findings") {
		t.Error("summary should mention 1 finding")
	}
}

func TestMaybeGenerateRemediationSpec_ExcludesErrorSeverity(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")

	// Create a RunState with terminal status
	run := &runstore.RunState{
		RunID:     "run-test-filters-severity",
		SpecID:    "spec-0009",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	// Save run to store
	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Create review.json with mixed severity findings
	reviewJSON := map[string]interface{}{
		"security": []interface{}{
			// Error severity - should be excluded
			map[string]interface{}{
				"file":          "secure.go",
				"line":          10,
				"severity":      "error",
				"facet":         "Critical SQL Injection",
				"description":   "SQL Injection vulnerability in database layer",
				"suggested_fix": "Use parameterized queries",
			},
			// Warning severity - should be included
			map[string]interface{}{
				"file":          "secure.go",
				"line":          20,
				"severity":      "warning",
				"facet":         "Missing Input Validation",
				"description":   "User input not validated before processing",
				"suggested_fix": "Add input validation checks",
			},
			// Info severity - should be included
			map[string]interface{}{
				"file":          "secure.go",
				"line":          30,
				"severity":      "info",
				"facet":         "Weak Hash Algorithm",
				"description":   "Uses MD5 instead of SHA256",
				"suggested_fix": "Replace with SHA256",
			},
		},
		"style": []interface{}{
			// Suggestion severity - should be included
			map[string]interface{}{
				"file":          "main.go",
				"line":          5,
				"severity":      "suggestion",
				"facet":         "Naming Convention",
				"description":   "Function name should be camelCase",
				"suggested_fix": "Rename function",
			},
		},
	}

	// Write review.json to run evidence directory
	evidenceDir := store.RunEvidenceDir("run-test-filters-severity")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("failed to create evidence directory: %v", err)
	}

	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal review.json: %v", err)
	}

	reviewPath := filepath.Join(evidenceDir, "review.json")
	if err := os.WriteFile(reviewPath, reviewData, 0o644); err != nil {
		t.Fatalf("failed to write review.json: %v", err)
	}

	// Call maybeGenerateRemediationSpec
	filePath, err := maybeGenerateRemediationSpec("run-test-filters-severity", storeDir, specsDir)
	if err != nil {
		t.Fatalf("maybeGenerateRemediationSpec failed: %v", err)
	}

	// Verify file path is returned
	if filePath == "" {
		t.Fatal("expected file path to be returned, got empty string")
	}

	// Read and verify file contents
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read spec file: %v", err)
	}

	specContent := string(content)

	// Verify warning severity finding is included
	if !strings.Contains(specContent, "Missing Input Validation") {
		t.Error("warning severity finding should be included")
	}

	// Verify info severity finding is included
	if !strings.Contains(specContent, "Weak Hash Algorithm") {
		t.Error("info severity finding should be included")
	}

	// Verify suggestion severity finding is included
	if !strings.Contains(specContent, "Naming Convention") {
		t.Error("suggestion severity finding should be included")
	}

	// Verify error severity finding is excluded
	if strings.Contains(specContent, "Critical SQL Injection") {
		t.Error("error severity finding should be excluded from remediation spec")
	}
	if strings.Contains(specContent, "SQL Injection vulnerability") {
		t.Error("error severity finding description should not appear in spec")
	}

	// Verify correct count: 3 findings (warning, info, suggestion), 1 error excluded
	if !strings.Contains(specContent, "3 findings") {
		t.Errorf("summary should mention 3 findings (error excluded), got:\n%s", specContent)
	}

	// Verify two categories are mentioned (security, style)
	if !strings.Contains(specContent, "2 categories") {
		t.Errorf("summary should mention 2 categories, got:\n%s", specContent)
	}
}

func TestMaybeGenerateRemediationSpec_SkipsEmptyDescription(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")

	// Create a RunState with terminal status
	run := &runstore.RunState{
		RunID:     "run-test-empty-desc",
		SpecID:    "spec-0010",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	// Save run to store
	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Create review.json with some findings having empty descriptions
	reviewJSON := map[string]interface{}{
		"security": []interface{}{
			// Valid finding with description
			map[string]interface{}{
				"file":          "auth.go",
				"line":          25,
				"severity":      "warning",
				"facet":         "Missing Input Validation",
				"description":   "User input not validated before database query",
				"suggested_fix": "Add input validation before query execution",
			},
			// Invalid finding - empty description (should be skipped)
			map[string]interface{}{
				"file":          "auth.go",
				"line":          50,
				"severity":      "warning",
				"facet":         "Missing Authentication Check",
				"description":   "", // Empty description - should be skipped
				"suggested_fix": "Add authentication check",
			},
			// Invalid finding - no description field (should be skipped)
			map[string]interface{}{
				"file":          "auth.go",
				"line":          75,
				"severity":      "warning",
				"facet":         "Weak Password Policy",
				"suggested_fix": "Enforce strong password requirements",
				// Missing description field entirely
			},
		},
		"style": []interface{}{
			// Valid finding with description
			map[string]interface{}{
				"file":          "main.go",
				"line":          10,
				"severity":      "suggestion",
				"facet":         "Naming Convention",
				"description":   "Variable uses snake_case instead of camelCase",
				"suggested_fix": "Rename to camelCase",
			},
		},
	}

	// Write review.json to run evidence directory
	evidenceDir := store.RunEvidenceDir("run-test-empty-desc")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("failed to create evidence directory: %v", err)
	}

	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal review.json: %v", err)
	}

	reviewPath := filepath.Join(evidenceDir, "review.json")
	if err := os.WriteFile(reviewPath, reviewData, 0o644); err != nil {
		t.Fatalf("failed to write review.json: %v", err)
	}

	// Call maybeGenerateRemediationSpec
	filePath, err := maybeGenerateRemediationSpec("run-test-empty-desc", storeDir, specsDir)
	if err != nil {
		t.Fatalf("maybeGenerateRemediationSpec failed: %v", err)
	}

	// Verify file path is returned
	if filePath == "" {
		t.Fatal("expected file path to be returned, got empty string")
	}

	// Read and verify file contents
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read spec file: %v", err)
	}

	specContent := string(content)

	// Verify valid findings ARE included
	if !strings.Contains(specContent, "Missing Input Validation") {
		t.Error("finding with valid description should be included")
	}
	if !strings.Contains(specContent, "Naming Convention") {
		t.Error("finding with valid description should be included")
	}
	if !strings.Contains(specContent, "User input not validated before database query") {
		t.Error("description of valid finding should appear in spec")
	}
	if !strings.Contains(specContent, "Variable uses snake_case instead of camelCase") {
		t.Error("description of valid finding should appear in spec")
	}

	// Verify findings with empty descriptions are EXCLUDED
	if strings.Contains(specContent, "Missing Authentication Check") {
		t.Error("finding with empty description should be excluded")
	}
	if strings.Contains(specContent, "Weak Password Policy") {
		t.Error("finding without description field should be excluded")
	}
	if strings.Contains(specContent, "Add authentication check") {
		t.Error("suggested fix from finding with empty description should not appear in spec")
	}
	if strings.Contains(specContent, "Enforce strong password requirements") {
		t.Error("suggested fix from finding without description should not appear in spec")
	}

	// Verify only 2 findings are included (the ones with valid descriptions)
	if !strings.Contains(specContent, "2 findings") {
		t.Errorf("summary should mention 2 findings (others excluded), got:\n%s", specContent)
	}

	// Verify exactly 2 acceptance criteria (one for each valid finding)
	if !strings.Contains(specContent, "## Acceptance Criteria") {
		t.Error("Acceptance Criteria section should exist")
	}

	// Count numbered criteria
	if !strings.Contains(specContent, "1. [auth.go]") {
		t.Error("first acceptance criterion missing")
	}
	if !strings.Contains(specContent, "2. [main.go]") {
		t.Error("second acceptance criterion missing")
	}
	if strings.Contains(specContent, "3. ") {
		t.Error("should not have more than 2 acceptance criteria")
	}
}

// ── Integration Tests ─────────────────────────────────────────────────────

func TestReviewRecordCmd_AcceptedCallsRemediation(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	specsDir := filepath.Join(tmp, "specs")

	// Create run store and run
	store := runstore.NewStore(storeDir)
	run := &runstore.RunState{
		RunID:     "run-remediation-test-001",
		SpecID:    "spec-0005",
		ProjectID: "test-project",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 3, 23, 10, 30, 0, 0, time.UTC),
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create review.json with non-blocking findings
	evidenceDir := store.RunEvidenceDir("run-remediation-test-001")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Create review.json with findings that should be included (non-blocking severities)
	reviewJSON := map[string]interface{}{
		"code_quality": []interface{}{
			map[string]interface{}{
				"file":          "main.go",
				"line":          42.0,
				"severity":      "warning", // Non-blocking
				"facet":         "Unused variable",
				"description":   "Variable declared but never used",
				"suggested_fix": "Remove unused variable",
			},
		},
	}

	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("marshal review.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create required review packet files (product-review, process-review, manual-checklist)
	emptyReview := map[string]interface{}{}
	for _, fname := range []string{"product-review.json", "process-review.json", "manual-checklist.json"} {
		data, _ := json.MarshalIndent(emptyReview, "", "  ")
		if err := os.WriteFile(filepath.Join(evidenceDir, fname), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", fname, err)
		}
	}

	// === Invoke ===
	cmd := newReviewRecordCmd()
	cmd.SetArgs([]string{
		"run-remediation-test-001",
		"--outcome", "accepted",
		"--summary", "Test acceptance",
		"--store-dir", storeDir,
		"--specs-dir", specsDir,
	})

	// Capture stdout and stderr
	var stdout, stderr strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = cmd.Execute()

	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// === Assert ===

	// Verify remediation spec file was created
	expectedSpecPath := filepath.Join(specsDir, "spec-0005-remediation.md")
	if _, err := os.Stat(expectedSpecPath); err != nil {
		t.Fatalf("remediation spec file not created: %v (stdout: %s, stderr: %s)", err, stdoutStr, stderrStr)
	}

	// Verify the spec file path is printed to stdout
	if !strings.Contains(stdoutStr, "spec-0005-remediation.md") {
		t.Errorf("expected spec path in stdout, got: %s", stdoutStr)
	}

	// Verify the spec file contains expected content
	specContent, err := os.ReadFile(expectedSpecPath)
	if err != nil {
		t.Fatalf("read spec file: %v", err)
	}
	specStr := string(specContent)

	if !strings.Contains(specStr, "## spec_id") || !strings.Contains(specStr, "spec-0005-remediation") {
		t.Error("remediation spec header missing")
	}

	if !strings.Contains(specStr, "Unused variable") {
		t.Error("remediation spec should contain the finding")
	}
}

func TestReviewRecordCmd_ReworkSkipsRemediation(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	specsDir := filepath.Join(tmp, "specs")

	// Create run store and run
	store := runstore.NewStore(storeDir)
	run := &runstore.RunState{
		RunID:     "run-remediation-test-002",
		SpecID:    "spec-0006",
		ProjectID: "test-project",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 23, 11, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 3, 23, 11, 30, 0, 0, time.UTC),
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create review.json with non-blocking findings
	evidenceDir := store.RunEvidenceDir("run-remediation-test-002")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Create review.json with findings
	reviewJSON := map[string]interface{}{
		"code_quality": []interface{}{
			map[string]interface{}{
				"file":          "main.go",
				"line":          42.0,
				"severity":      "warning",
				"facet":         "Unused variable",
				"description":   "Variable declared but never used",
				"suggested_fix": "Remove unused variable",
			},
		},
	}

	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("marshal review.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create required review packet files
	emptyReview := map[string]interface{}{}
	for _, fname := range []string{"product-review.json", "process-review.json", "manual-checklist.json"} {
		data, _ := json.MarshalIndent(emptyReview, "", "  ")
		if err := os.WriteFile(filepath.Join(evidenceDir, fname), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", fname, err)
		}
	}

	// === Invoke ===
	cmd := newReviewRecordCmd()
	cmd.SetArgs([]string{
		"run-remediation-test-002",
		"--outcome", "rework_implementation_gap",
		"--summary", "Test rework",
		"--store-dir", storeDir,
		"--specs-dir", specsDir,
	})

	// Capture stdout and stderr
	var stdout, stderr strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = cmd.Execute()

	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	stdoutStr := stdout.String()

	// === Assert ===

	// Verify remediation spec file was NOT created
	expectedSpecPath := filepath.Join(specsDir, "spec-0006-remediation.md")
	if _, err := os.Stat(expectedSpecPath); err == nil {
		t.Fatal("remediation spec file should not be created for rework outcome")
	}

	// Verify spec path is NOT in stdout
	if strings.Contains(stdoutStr, "spec-0006-remediation.md") {
		t.Errorf("spec path should not be in stdout for rework outcome, got: %s", stdoutStr)
	}
}

func TestReviewRecordCmd_NoSpecsDirWarns(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")

	// Create run store and run
	store := runstore.NewStore(storeDir)
	run := &runstore.RunState{
		RunID:     "run-no-specsdir",
		SpecID:    "spec-test",
		ProjectID: "test-project",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 23, 11, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 3, 23, 11, 30, 0, 0, time.UTC),
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory with review packet files
	evidenceDir := store.RunEvidenceDir("run-no-specsdir")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Create review.json with non-blocking findings (to exercise remediation path)
	reviewJSON := map[string]interface{}{
		"style": []interface{}{
			map[string]interface{}{
				"file":          "main.go",
				"line":          10.0,
				"severity":      "warning",
				"facet":         "Naming",
				"description":   "Use camelCase",
				"suggested_fix": "Rename to camelCase",
			},
		},
	}
	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("marshal review.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create required review packet files
	productReview := reviewpacket.ProductReview{
		RunID:         "run-no-specsdir",
		SpecTitle:     "spec-test",
		TerminalState: "completed",
		Summary:       "All good",
		BehaviorCards: []reviewpacket.BehaviorCard{},
		Surprises:     []string{},
	}
	processReview := reviewpacket.ProcessReview{
		TrustLevel:         "high",
		AutomaticProof:     "pass",
		MachineReview:      "pass",
		Acceptance:         "pass",
		DegradedFlags:      []string{},
		RecommendedPosture: "accept",
	}
	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{},
	}

	for name, v := range map[string]interface{}{
		"product-review.json":   productReview,
		"process-review.json":   processReview,
		"manual-checklist.json": manualChecklist,
	} {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// === Invoke ===
	cmd := newReviewRecordCmd()
	cmd.SetArgs([]string{
		"--run", "run-no-specsdir",
		"--outcome", "accepted",
		"--summary", "Looks good",
		"--store-dir", storeDir,
		// No --specs-dir, no --project
	})

	// Capture stderr to verify warning is printed
	var stderr strings.Builder
	cmd.SetErr(&stderr)

	err = cmd.Execute()

	stderrStr := stderr.String()

	// === Assert ===

	// Verify command succeeds (accept should not fail due to missing specs-dir)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify review-outcome.json is written (accept succeeds)
	outcomePath := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomePath); err != nil {
		t.Errorf("review-outcome.json not found: %v", err)
	}

	// Verify stderr contains warning about skipping remediation spec generation
	if !strings.Contains(stderrStr, "skipping remediation spec generation") {
		t.Errorf("expected stderr warning about skipping remediation spec generation, got:\n%s", stderrStr)
	}
	if !strings.Contains(stderrStr, "specs-dir not configured") {
		t.Errorf("expected stderr to mention specs-dir not configured, got:\n%s", stderrStr)
	}
}
