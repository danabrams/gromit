package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestRenderBuildPrompt_PopulatesStaticPreambleCacheMetadataPerPromptClass(t *testing.T) {
	templatesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_build.md"), []byte("{{ .Rules }}\n{{ .Spec }}"), 0644); err != nil {
		t.Fatalf("WriteFile(PROMPT_build.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_atdd_build.md"), []byte("{{ .Rules }}\n{{ .Spec }}"), 0644); err != nil {
		t.Fatalf("WriteFile(PROMPT_atdd_build.md) error = %v", err)
	}

	r := &Renderer{templatesDir: templatesDir}
	ctx := &Context{
		Bead:  &bead.Bead{ID: "b1", Title: "Task", Description: "desc"},
		Rules: "rules",
		Spec:  "spec",
	}

	if _, err := r.RenderBuild(ctx); err != nil {
		t.Fatalf("RenderBuild() error = %v", err)
	}
	buildClass := ctx.StaticPreambleCacheClass
	buildKey := ctx.StaticPreambleCacheKey
	if buildClass == "" || buildKey == "" {
		t.Fatalf("expected RenderBuild to populate cache metadata, got class=%q key=%q", buildClass, buildKey)
	}

	if _, err := r.RenderATDDBuild(ctx); err != nil {
		t.Fatalf("RenderATDDBuild() error = %v", err)
	}
	atddClass := ctx.StaticPreambleCacheClass
	atddKey := ctx.StaticPreambleCacheKey
	if atddClass == "" || atddKey == "" {
		t.Fatalf("expected RenderATDDBuild to populate cache metadata, got class=%q key=%q", atddClass, atddKey)
	}
	if atddClass == buildClass && atddKey == buildKey {
		t.Fatalf("expected cache metadata to differ across prompt classes, got build=(%q,%q) atdd=(%q,%q)", buildClass, buildKey, atddClass, atddKey)
	}
}
