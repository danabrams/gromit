package prompt

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/danabrams/gromit/internal/bead"
)

func TestRenderBuild_UsesTemplateOverride(t *testing.T) {
    tmp := t.TempDir()
    defaultName := "PROMPT_build.md"
    overrideName := "PROMPT_override.md"

    if err := os.WriteFile(filepath.Join(tmp, defaultName), []byte("default"), 0o644); err != nil {
        t.Fatalf("write default template: %v", err)
    }
    if err := os.WriteFile(filepath.Join(tmp, overrideName), []byte("override"), 0o644); err != nil {
        t.Fatalf("write override template: %v", err)
    }

    r := &Renderer{
        templatesDir: tmp,
        specCache:    make(map[string]string),
    }

    ctx := &Context{
        Bead:             &bead.Bead{ID: "b1", Title: "B"},
        TemplateOverride: overrideName,
    }
    ctx.normalizeNilFields()

    result, err := r.RenderBuild(ctx)
    if err != nil {
        t.Fatalf("RenderBuild error: %v", err)
    }
    if result != "override" {
        t.Fatalf("got %q, want override template result", result)
    }
}
