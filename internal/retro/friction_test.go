package retro

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/learnings"
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

func TestClusterByAreaComputesTimespanAndEvidence(t *testing.T) {
	base := time.Date(2026, time.February, 3, 10, 0, 0, 0, time.UTC)
	entries := []learnings.Learning{
		{
			Content:  "Notes reference internal/service/api.go and internal/service/handler.go",
			Date:     base,
			Category: learnings.CategoryGotchas,
		},
		{
			Content:  "See internal/service/pipeline.go for more context",
			Date:     base.Add(24 * time.Hour),
			Category: learnings.CategoryPatterns,
		},
	}
	clusters := clusterByArea(entries, 1)
	if len(clusters) != 1 {
		t.Fatalf("expected one cluster, got %d", len(clusters))
	}
	cluster := clusters[0]
	if cluster.Area != "internal/service" {
		t.Fatalf("unexpected area: %s", cluster.Area)
	}
	if cluster.LearningCount != 2 {
		t.Fatalf("unexpected count: %d", cluster.LearningCount)
	}
	if !cluster.EarliestDate.Equal(base) {
		t.Fatalf("earliest date mismatch")
	}
	if !cluster.LatestDate.Equal(base.Add(24 * time.Hour)) {
		t.Fatalf("latest date mismatch")
	}
	if cluster.Timespan != 24*time.Hour {
		t.Fatalf("expected 24h timespan, got %v", cluster.Timespan)
	}
	if got := cluster.Categories[learnings.CategoryGotchas]; got != 1 {
		t.Fatalf("expected 1 gotchas, got %d", got)
	}
	if got := cluster.Categories[learnings.CategoryPatterns]; got != 1 {
		t.Fatalf("expected 1 patterns, got %d", got)
	}
}

func TestClusterByAreaFiltersMinClusterSize(t *testing.T) {
	entries := []learnings.Learning{
		{
			Content: "pkg/solo/module.go",
			Date:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	clusters := clusterByArea(entries, 2)
	if clusters != nil {
		t.Fatalf("expected nil when no clusters meet min size, got %v", clusters)
	}
}

func TestExtractAreaHandlesWindowsPaths(t *testing.T) {
	content := "Breaking changes happened at internal\\retro\\friction.go and internal\\retro\\prompt.go"
	if got := extractArea(content); got != "internal/retro" {
		t.Fatalf("expected internal/retro, got %q", got)
	}
}
