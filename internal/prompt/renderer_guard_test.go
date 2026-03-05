package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRendererMissingKeyError(t *testing.T) {
	templatesDir := t.TempDir()
	templatePath := filepath.Join(templatesDir, "missing.md")
	if err := os.WriteFile(templatePath, []byte("Hello {{.Missing}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(missing template) error = %v", err)
	}

	r := &Renderer{templatesDir: templatesDir}
	_, err := r.render("missing.md", struct{}{})
	if err == nil {
		t.Fatalf("render() succeeded, want missing key error")
	}
	if !strings.Contains(err.Error(), "can't evaluate field Missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderBuildHandlesNilBead(t *testing.T) {
	templatesDir := filepath.Join("..", "..", ".gromit", "templates")
	templatePath := filepath.Join(templatesDir, "PROMPT_build.md")
	if _, err := os.Stat(templatePath); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("real templates not found at %s", templatesDir)
		}
		t.Fatalf("stat templates dir: %v", err)
	}

	r := &Renderer{templatesDir: templatesDir}
	ctx := &Context{
		Bead:    nil,
		Rules:   "Use explicit defaults",
		Spec:    "Acceptance criteria",
		Model:   "sonnet",
		WorkDir: "src",
	}

	if _, err := r.RenderBuild(ctx); err != nil {
		t.Fatalf("RenderBuild() error = %v", err)
	}
}
