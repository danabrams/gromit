package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderMidBuildReview_IncludesTemplateData(t *testing.T) {
	templatesDir := t.TempDir()
	content := "Title: {{ .BeadTitle }}\nDiff: {{ .Diff }}\nCriteria: {{ .AcceptanceCriteria }}"
	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_midreview.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	r := &Renderer{templatesDir: templatesDir, specCache: make(map[string]string)}
	ctx := &MidBuildReviewContext{
		BeadTitle:          "Fix bug",
		BeadDescription:    "Fix the thing",
		Diff:               "diff",
		Spec:               "spec",
		AcceptanceCriteria: "keep things green",
	}

	got, err := r.RenderMidBuildReview(ctx)
	if err != nil {
		t.Fatalf("RenderMidBuildReview error: %v", err)
	}

	want := "Title: Fix bug\nDiff: diff\nCriteria: keep things green"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
