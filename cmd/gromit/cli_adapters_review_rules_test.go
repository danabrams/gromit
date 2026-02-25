package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
)

func TestCliPromptRenderer_RenderThoroughReview_UsesThoroughReviewPhaseRules(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll templates: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll specs: %v", err)
	}

	const templateContent = "Rules:\n{{.Rules}}\n"
	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_thorough_review.md"), []byte(templateContent), 0o644); err != nil {
		t.Fatalf("WriteFile template: %v", err)
	}
	const rulesContent = `# Rules

## Review Rule <!-- phases: thorough_review -->
review-only

## Build Rule <!-- phases: build -->
build-only
`
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rulesContent), 0o644); err != nil {
		t.Fatalf("WriteFile rules: %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	adapter := &cliPromptRenderer{renderer: renderer}

	got, err := adapter.RenderThoroughReview(&pipeline.ThoroughReviewPromptInput{Diff: "diff"})
	if err != nil {
		t.Fatalf("RenderThoroughReview: %v", err)
	}
	if !strings.Contains(got, "review-only") {
		t.Fatalf("expected thorough_review rules in output, got: %q", got)
	}
	if strings.Contains(got, "build-only") {
		t.Fatalf("expected build-only rules to be filtered out, got: %q", got)
	}
}
