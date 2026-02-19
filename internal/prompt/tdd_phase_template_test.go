package prompt

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderTDDRedTemplateIncludesRequiredSections(t *testing.T) {
	r := &Renderer{templatesDir: filepath.Join("..", "..", ".gromit", "templates")}

	ctx := &TDDRedContext{
		BeadID:      "gromit-60m8d",
		BeadTitle:   "Create TDD red and green phase templates",
		SpecExcerpt: "Acceptance: render spec excerpt and cycle summary.",
		TestFileContents: map[string]string{
			"internal/prompt/prompt_test.go": "func TestSomething(t *testing.T) {}",
		},
		APISurface:        "RenderTDDRed(ctx *TDDRedContext) (string, error)",
		CycleSummary:      "Cycle 1 complete, next requirement pending.",
		Rules:             "- Write one failing test.\n- Keep changes minimal.",
		ScopedTestCommand: "go test ./internal/prompt/...",
	}

	result, err := r.RenderTDDRed(ctx)
	if err != nil {
		t.Fatalf("RenderTDDRed() error = %v", err)
	}

	required := []string{
		"## Role",
		"## Rules",
		"## Context",
		"## Task",
		ctx.SpecExcerpt,
		ctx.CycleSummary,
		ctx.APISurface,
		"```",
		"`internal/prompt/prompt_test.go`",
		ctx.ScopedTestCommand,
	}
	for _, fragment := range required {
		if !strings.Contains(result, fragment) {
			t.Errorf("expected rendered red prompt to include %q", fragment)
		}
	}

	notExpected := []string{
		"## Learnings",
		"## Project Context",
		"## Specification",
	}
	for _, fragment := range notExpected {
		if strings.Contains(result, fragment) {
			t.Errorf("expected rendered red prompt to omit %q", fragment)
		}
	}
}
