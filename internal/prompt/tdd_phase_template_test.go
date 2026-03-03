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

func TestRenderTDDGreenTemplateIncludesRequiredSections(t *testing.T) {
    r := &Renderer{templatesDir: filepath.Join("..", "..", ".gromit", "templates")}

	ctx := &TDDGreenContext{
		BeadID:            "gromit-60m8d",
		BeadTitle:         "Create TDD red and green phase templates",
		FailingTest:       "func TestRenderTDDGreen(t *testing.T) { t.Fatal(\"expected\") }",
		TestFailureOutput: "--- FAIL: TestRenderTDDGreen (0.00s)\nexpected",
		ImplFileContents: map[string]string{
			"internal/prompt/prompt.go": "func (r *Renderer) RenderTDDGreen(ctx *TDDGreenContext) (string, error) { return \"\", nil }",
		},
		Rules:             "- Make one test pass.\n- Minimal implementation only.",
		ScopedTestCommand: "go test ./internal/prompt/...",
	}

	result, err := r.RenderTDDGreen(ctx)
	if err != nil {
		t.Fatalf("RenderTDDGreen() error = %v", err)
	}

	required := []string{
		"## Role",
		"## Rules",
		"## Context",
		"## Task",
		ctx.FailingTest,
		ctx.TestFailureOutput,
		"```",
		"`internal/prompt/prompt.go`",
		ctx.ScopedTestCommand,
	}
	for _, fragment := range required {
		if !strings.Contains(result, fragment) {
			t.Errorf("expected rendered green prompt to include %q", fragment)
		}
	}

	notExpected := []string{
		"## Learnings",
		"## Project Context",
		"## Specification",
		"Spec Excerpt",
		"TDD Cycle Summary",
	}
	for _, fragment := range notExpected {
		if strings.Contains(result, fragment) {
			t.Errorf("expected rendered green prompt to omit %q", fragment)
		}
	}
}

func TestRenderTDDPhaseTemplatesFallbackToBuiltinContent(t *testing.T) {
    // Use an empty templates directory so file lookups fail.
    r := &Renderer{templatesDir: t.TempDir()}

    redCtx := &TDDRedContext{
        BeadID:      "fallback-red",
        BeadTitle:   "Fallback Red",
        SpecExcerpt: "Fallback spec",
        TestFileContents: map[string]string{
            "file_test.go": "func TestFallback(t *testing.T) {}",
        },
        APISurface:    "RenderTDDRed(ctx *TDDRedContext)",
        CycleSummary:  "Cycle summary",
        Rules:         "- Follow the fallback template",
        ScopedTestCommand: "go test ./...",
    }
    redResult, redErr := r.RenderTDDRed(redCtx)
    if redErr != nil {
        t.Fatalf("RenderTDDRed failed with builtin fallback: %v", redErr)
    }
    if !strings.Contains(redResult, "# TDD Red Phase") {
        t.Fatalf("expected builtin red template header, got:\n%s", redResult)
    }

    greenCtx := &TDDGreenContext{
        BeadID:            "fallback-green",
        BeadTitle:         "Fallback Green",
        FailingTest:       "func TestFallbackGreen(t *testing.T) { t.Fatal(\"fail\") }",
        TestFailureOutput: "--- FAIL: TestFallbackGreen (0.00s)\nfail",
        ImplFileContents: map[string]string{
            "file.go": "func fallback() {}",
        },
        Rules:             "- Follow the fallback template",
        ScopedTestCommand: "go test ./...",
    }
    greenResult, greenErr := r.RenderTDDGreen(greenCtx)
    if greenErr != nil {
        t.Fatalf("RenderTDDGreen failed with builtin fallback: %v", greenErr)
    }
    if !strings.Contains(greenResult, "# TDD Green Phase") {
        t.Fatalf("expected builtin green template header, got:\n%s", greenResult)
    }
}
