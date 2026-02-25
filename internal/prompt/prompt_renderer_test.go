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
