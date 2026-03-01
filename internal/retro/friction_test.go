package retro

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

func TestRunPopulatesFrictionContext(t *testing.T) {
    tmpDir := t.TempDir()
    templateDir := filepath.Join(tmpDir, "templates")
    if err := os.MkdirAll(templateDir, 0o755); err != nil {
        t.Fatalf("setup template dir: %v", err)
    }
    templatePath := filepath.Join(templateDir, "PROMPT_retro.md")
    templateContent := `{{.Rules}}
{{.Learnings}}
{{if .FrictionClusters}}
Friction areas detected.
{{else}}
No friction.
{{end}}`
    if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
        t.Fatalf("writing template: %v", err)
    }

    logsDir := filepath.Join(tmpDir, "logs")
    if err := os.MkdirAll(logsDir, 0o755); err != nil {
        t.Fatalf("creating logs dir: %v", err)
    }

    learningsContent := `# Learnings

Accumulated operational knowledge from Gromit iterations.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-25 | gromit-001 | gotchas

Repeated failures in internal/retro/retro.go due to missing context propagation.

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-25 | gromit-002 | gotchas

The same crash pipeline points back to internal/retro/retro.go when logging fails.
`
    learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
    if err := os.WriteFile(learningsPath, []byte(learningsContent), 0o644); err != nil {
        t.Fatalf("writing learnings: %v", err)
    }

    provider := &mockProvider{}
    retro, err := NewRetroWithProvider(provider, tmpDir)
    if err != nil {
        t.Fatalf("NewRetroWithProvider: %v", err)
    }

    if _, err := retro.Run(context.Background(), nil); err != nil {
        t.Fatalf("Retro.Run: %v", err)
    }

    ctx := retro.lastTemplateContext
    if len(ctx.FrictionClusters) == 0 {
        t.Fatalf("expected friction clusters, got none")
    }
    if len(ctx.FrictionResolutions) == 0 {
        t.Fatalf("expected friction resolutions, got none")
    }
}
