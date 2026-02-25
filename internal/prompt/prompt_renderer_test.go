package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPromptRendererInterfaceImplementation verifies that PromptRenderer interface exists
// and that Renderer implements RenderCoverageValidation.
func TestPromptRendererInterfaceImplementation(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `Criterion: {{ .CriterionNumber }}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_coverage_validation.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	// This test verifies that Renderer can be assigned to a PromptRenderer variable
	var renderer PromptRenderer = r

	ctx := &CoverageValidationContext{
		CriterionNumber: 1,
		CriterionText:   "Test criterion",
		TestCode:        "test code",
	}

	result, err := renderer.RenderCoverageValidation(ctx)
	if err != nil {
		t.Fatalf("RenderCoverageValidation() error = %v", err)
	}

	if result == "" {
		t.Fatal("expected non-empty result from RenderCoverageValidation")
	}
}

// TestPromptRendererCoverageValidationNilContext verifies that RenderCoverageValidation
// through the PromptRenderer interface properly handles nil context.
func TestPromptRendererCoverageValidationNilContext(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `Criterion: {{ .CriterionNumber }}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_coverage_validation.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	// This test verifies that PromptRenderer properly returns an error for nil context
	var renderer PromptRenderer = r

	_, err := renderer.RenderCoverageValidation(nil)
	if err == nil {
		t.Fatal("expected error for nil context")
	}
}

// TestPromptRendererAcceptsImplementation verifies that a function accepting
// PromptRenderer can be called with a Renderer implementation.
func TestPromptRendererAcceptsImplementation(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `Criterion {{ .CriterionNumber }}: {{ .CriterionText }}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_coverage_validation.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	// Test that renderWithInterface can accept a *Renderer
	result, err := renderWithInterface(r, &CoverageValidationContext{
		CriterionNumber: 42,
		CriterionText:   "Sample test",
		TestCode:        "func Test() {}",
	})

	if err != nil {
		t.Fatalf("renderWithInterface() error = %v", err)
	}

	if result == "" {
		t.Fatal("expected non-empty result")
	}

	if !contains(result, "42") {
		t.Errorf("expected criterion number in result, got: %s", result)
	}
}

// renderWithInterface is a helper function that accepts a PromptRenderer interface.
// This demonstrates that Renderer can be used polymorphically.
func renderWithInterface(pr PromptRenderer, ctx *CoverageValidationContext) (string, error) {
	return pr.RenderCoverageValidation(ctx)
}

// contains is a simple string helper
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
