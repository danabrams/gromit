//go:build acceptance

package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderReviewAcceptanceTests_TemplateExists verifies that the template
// file exists at the expected location.
func TestRenderReviewAcceptanceTests_TemplateExists(t *testing.T) {
	// Expected failure: PROMPT_review_acceptance_tests.md does not exist yet

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	// Copy the template from the actual location
	templatePath := filepath.Join(".gromit", "templates", "PROMPT_review_acceptance_tests.md")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("template file does not exist at %s: %v", templatePath, err)
	}

	// Template should contain key sections
	contentStr := string(content)
	requiredSections := []string{
		"## Bead",
		"## Acceptance Criteria",
		"## Tests Written",
		"## Review Criteria",
		"## Output",
		"VERDICT: PASS",
		"VERDICT: FAIL",
	}

	for _, section := range requiredSections {
		if !strings.Contains(contentStr, section) {
			t.Errorf("template missing required section: %s", section)
		}
	}
}

// TestRenderReviewAcceptanceTests_RendersSuccessfully verifies that
// RenderReviewAcceptanceTests can be called and produces output.
func TestRenderReviewAcceptanceTests_RendersSuccessfully(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method does not exist yet
	// Expected failure: ReviewAcceptanceTestsContext type does not exist yet

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	gromitDir := filepath.Join(tmpDir, ".gromit")

	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	// Create the template
	templateContent := `You are reviewing acceptance tests.

## Bead
**Title:** {{.BeadTitle}}
**Description:** {{.BeadDescription}}

## Acceptance Criteria
{{.AcceptanceCriteria}}

## Tests Written (git diff)
{{.TestDiff}}

## Review Criteria
Review each test.

## Output
VERDICT: PASS or VERDICT: FAIL
`
	templatePath := filepath.Join(templatesDir, "PROMPT_review_acceptance_tests.md")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	r := &Renderer{
		templatesDir: templatesDir,
		specsDir:     specsDir,
		claudeMDPath: claudeMDPath,
		gromitDir:    gromitDir,
	}

	ctx := &ReviewAcceptanceTestsContext{
		BeadTitle:          "Add user authentication",
		BeadDescription:    "Implement login and logout",
		AcceptanceCriteria: "Users can log in\nUsers can log out",
		TestDiff:           "diff --git a/auth_test.go b/auth_test.go\n+func TestLogin",
	}

	result, err := r.RenderReviewAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("RenderReviewAcceptanceTests failed: %v", err)
	}

	if result == "" {
		t.Error("RenderReviewAcceptanceTests returned empty string")
	}

	// Verify template variables were substituted
	if !strings.Contains(result, "Add user authentication") {
		t.Error("result does not contain BeadTitle")
	}
	if !strings.Contains(result, "Implement login and logout") {
		t.Error("result does not contain BeadDescription")
	}
	if !strings.Contains(result, "Users can log in") {
		t.Error("result does not contain AcceptanceCriteria")
	}
	if !strings.Contains(result, "diff --git a/auth_test.go") {
		t.Error("result does not contain TestDiff")
	}
}

// TestRenderReviewAcceptanceTests_HandlesEmptyFields verifies that
// the render method handles empty fields gracefully.
func TestRenderReviewAcceptanceTests_HandlesEmptyFields(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method does not exist yet
	// Expected failure: ReviewAcceptanceTestsContext type does not exist yet

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	gromitDir := filepath.Join(tmpDir, ".gromit")

	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	templateContent := `Title: {{.BeadTitle}}
Description: {{.BeadDescription}}
Criteria: {{.AcceptanceCriteria}}
Diff: {{.TestDiff}}`

	templatePath := filepath.Join(templatesDir, "PROMPT_review_acceptance_tests.md")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	r := &Renderer{
		templatesDir: templatesDir,
		specsDir:     specsDir,
		claudeMDPath: claudeMDPath,
		gromitDir:    gromitDir,
	}

	// Empty context should still render without errors
	ctx := &ReviewAcceptanceTestsContext{}

	result, err := r.RenderReviewAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("RenderReviewAcceptanceTests with empty context failed: %v", err)
	}

	if result == "" {
		t.Error("RenderReviewAcceptanceTests returned empty string")
	}

	// Should contain the template structure but with empty values
	expected := "Title: \nDescription: \nCriteria: \nDiff: "
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// TestRenderReviewAcceptanceTests_HandlesMultilineContent verifies that
// the render method correctly handles multiline content in all fields.
func TestRenderReviewAcceptanceTests_HandlesMultilineContent(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method does not exist yet
	// Expected failure: ReviewAcceptanceTestsContext type does not exist yet

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	gromitDir := filepath.Join(tmpDir, ".gromit")

	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	templateContent := `## Criteria
{{.AcceptanceCriteria}}

## Diff
{{.TestDiff}}`

	templatePath := filepath.Join(templatesDir, "PROMPT_review_acceptance_tests.md")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	r := &Renderer{
		templatesDir: templatesDir,
		specsDir:     specsDir,
		claudeMDPath: claudeMDPath,
		gromitDir:    gromitDir,
	}

	multilineCriteria := "1. Users can log in\n2. Users can log out\n3. Sessions are secure"
	multilineDiff := `diff --git a/auth_test.go b/auth_test.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/auth_test.go
@@ -0,0 +1,20 @@
+package auth
+
+func TestLogin(t *testing.T) {
+	// test implementation
+}`

	ctx := &ReviewAcceptanceTestsContext{
		BeadTitle:          "Auth",
		BeadDescription:    "Add auth",
		AcceptanceCriteria: multilineCriteria,
		TestDiff:           multilineDiff,
	}

	result, err := r.RenderReviewAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("RenderReviewAcceptanceTests failed: %v", err)
	}

	// Multiline content should be preserved
	if !strings.Contains(result, "1. Users can log in") {
		t.Error("multiline criteria line 1 not found")
	}
	if !strings.Contains(result, "2. Users can log out") {
		t.Error("multiline criteria line 2 not found")
	}
	if !strings.Contains(result, "3. Sessions are secure") {
		t.Error("multiline criteria line 3 not found")
	}
	if !strings.Contains(result, "diff --git a/auth_test.go") {
		t.Error("multiline diff header not found")
	}
	if !strings.Contains(result, "+func TestLogin(t *testing.T)") {
		t.Error("multiline diff content not found")
	}
}

// TestRenderReviewAcceptanceTests_NilRenderer verifies behavior with nil receiver.
func TestRenderReviewAcceptanceTests_NilRenderer(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method does not exist yet
	// Expected failure: ReviewAcceptanceTestsContext type does not exist yet

	var r *Renderer
	ctx := &ReviewAcceptanceTestsContext{
		BeadTitle: "Test",
	}

	_, err := r.RenderReviewAcceptanceTests(ctx)
	if err == nil {
		t.Error("expected error for nil renderer, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected error message to mention 'nil', got: %v", err)
	}
}

// TestRenderReviewAcceptanceTests_TemplateNotFound verifies error handling
// when the template file does not exist.
func TestRenderReviewAcceptanceTests_TemplateNotFound(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method does not exist yet
	// Expected failure: ReviewAcceptanceTestsContext type does not exist yet

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	gromitDir := filepath.Join(tmpDir, ".gromit")

	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	// Do NOT create the template file

	r := &Renderer{
		templatesDir: templatesDir,
		specsDir:     specsDir,
		claudeMDPath: claudeMDPath,
		gromitDir:    gromitDir,
	}

	ctx := &ReviewAcceptanceTestsContext{
		BeadTitle: "Test",
	}

	_, err := r.RenderReviewAcceptanceTests(ctx)
	if err == nil {
		t.Error("expected error when template not found, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "PROMPT_review_acceptance_tests.md") {
		t.Errorf("expected error to reference template name, got: %v", err)
	}
}

// TestRenderReviewAcceptanceTests_ReturnTypeIsString verifies that
// the method returns a string as its first return value.
func TestRenderReviewAcceptanceTests_ReturnTypeIsString(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method does not exist yet
	// Expected failure: ReviewAcceptanceTestsContext type does not exist yet

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	gromitDir := filepath.Join(tmpDir, ".gromit")

	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	templateContent := "Test template"
	templatePath := filepath.Join(templatesDir, "PROMPT_review_acceptance_tests.md")
	os.WriteFile(templatePath, []byte(templateContent), 0644)

	r := &Renderer{
		templatesDir: templatesDir,
		specsDir:     specsDir,
		claudeMDPath: claudeMDPath,
		gromitDir:    gromitDir,
	}

	ctx := &ReviewAcceptanceTestsContext{}

	result, err := r.RenderReviewAcceptanceTests(ctx)

	// Type assertions to verify return types
	var _ string = result
	var _ error = err
}

// TestRenderReviewAcceptanceTests_OutputContainsVerdictFormat verifies that
// the rendered output includes the verdict format per spec.
func TestRenderReviewAcceptanceTests_OutputContainsVerdictFormat(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method does not exist yet
	// Expected failure: ReviewAcceptanceTestsContext type does not exist yet

	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	gromitDir := filepath.Join(tmpDir, ".gromit")

	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	// Use a realistic template that matches the spec
	templateContent := `# Acceptance Test Review

You are reviewing acceptance tests written for an ATDD workflow.

## Bead

**Title:** {{.BeadTitle}}

**Description:** {{.BeadDescription}}

## Acceptance Criteria

{{.AcceptanceCriteria}}

## Tests Written (git diff)

` + "```diff" + `
{{.TestDiff}}
` + "```" + `

## Review Criteria

For each test, evaluate:

1. Does it assert on behavior that requires implementation changes to pass?
2. Does it reference functions, methods, types, or constants that do not currently exist?
3. Would it pass against the current codebase without any changes?

A test that would pass against the current codebase is testing existing behavior, not new behavior. It must be rewritten.

## Output

Respond with exactly one of:
- **VERDICT: PASS** — if all tests require new behavior
- **VERDICT: FAIL** — followed by a description of which tests are weak and what they should test instead
`

	templatePath := filepath.Join(templatesDir, "PROMPT_review_acceptance_tests.md")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	r := &Renderer{
		templatesDir: templatesDir,
		specsDir:     specsDir,
		claudeMDPath: claudeMDPath,
		gromitDir:    gromitDir,
	}

	ctx := &ReviewAcceptanceTestsContext{
		BeadTitle:          "Add authentication",
		BeadDescription:    "Implement login and logout functionality",
		AcceptanceCriteria: "Users can log in with username and password",
		TestDiff:           "diff --git a/auth_test.go b/auth_test.go\n+func TestLogin",
	}

	result, err := r.RenderReviewAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("RenderReviewAcceptanceTests failed: %v", err)
	}

	// Verify the output contains the verdict format strings
	if !strings.Contains(result, "VERDICT: PASS") {
		t.Error("rendered output does not contain 'VERDICT: PASS'")
	}
	if !strings.Contains(result, "VERDICT: FAIL") {
		t.Error("rendered output does not contain 'VERDICT: FAIL'")
	}

	// Verify instructions are present
	if !strings.Contains(result, "Respond with exactly one of:") {
		t.Error("rendered output does not contain verdict instructions")
	}

	// Verify review criteria are present
	if !strings.Contains(result, "Review Criteria") {
		t.Error("rendered output does not contain Review Criteria section")
	}
	if !strings.Contains(result, "Does it assert on behavior that requires implementation changes to pass?") {
		t.Error("rendered output does not contain first review criterion")
	}
}
