package prompt

import (
	"strings"
	"testing"
)

func TestPromptAssemblerAddsLayerMarkers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "project layer", "instance layer", "fragment layer")
	output := assembler.Assemble()

	expectedSequence := []string{
		"=== BASE ===",
		"base layer",
		"=== PROJECT ===",
		"project layer",
		"=== INSTANCE ===",
		"instance layer",
		"=== FRAGMENT ===",
		"fragment layer",
	}

	lastIndex := 0
	for _, fragment := range expectedSequence {
		idx := strings.Index(output, fragment)
		if idx == -1 {
			t.Fatalf("output missing %q", fragment)
		}
		if idx < lastIndex {
			t.Fatalf("%q appears out of order", fragment)
		}
		lastIndex = idx
	}
}

func TestPromptAssemblerSkipsEmptyLayers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "", "instance layer", "")
	output := assembler.Assemble()

	if !strings.Contains(output, "=== BASE ===") {
		t.Fatalf("base section missing")
	}
	if strings.Contains(output, "=== PROJECT ===") {
		t.Fatalf("project section should be omitted when empty")
	}
	if !strings.Contains(output, "=== INSTANCE ===") {
		t.Fatalf("instance section missing")
	}
	if strings.Contains(output, "=== FRAGMENT ===") {
		t.Fatalf("fragment section should be omitted when empty")
	}
}

func TestPromptAssemblerSkipsWhitespaceOnlyLayers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "  \n\t", "instance layer", "fragment layer")
	output := assembler.Assemble()

	if strings.Contains(output, "=== PROJECT ===") {
		t.Fatalf("project section should be omitted when it contains only whitespace")
	}
	if !strings.Contains(output, "=== BASE ===") {
		t.Fatalf("base section missing")
	}
	if !strings.Contains(output, "=== INSTANCE ===") {
		t.Fatalf("instance section missing")
	}
	if !strings.Contains(output, "=== FRAGMENT ===") {
		t.Fatalf("fragment section missing")
	}
}

func TestLoadProjectContextNoPackagePathsReturnsProjectUnchanged(t *testing.T) {
	project := "# Project\nSome instructions"
	assembler := NewPromptAssembler("base", project, "instance", "fragment")
	got := assembler.loadProjectContext(BeadInfo{Title: "add some feature"})
	if got != project {
		t.Errorf("expected project unchanged, got %q", got)
	}
}

func TestLoadBaseInstructionsFiltersPhase(t *testing.T) {
	base := `# Rules

## Code Style <!-- phases: red, build, green, refactor, review -->

- Use go fmt standard formatting

## Build Only <!-- phases: build -->

- Always run tests before committing

## Review Only <!-- phases: review -->

- Check for security issues`

	assembler := NewPromptAssembler(base, "", "", "")

	buildResult := assembler.loadBaseInstructions("build")
	if !strings.Contains(buildResult, "## Code Style") {
		t.Errorf("build result missing ## Code Style")
	}
	if !strings.Contains(buildResult, "## Build Only") {
		t.Errorf("build result missing ## Build Only")
	}
	if strings.Contains(buildResult, "## Review Only") {
		t.Errorf("build result should not contain ## Review Only")
	}

	reviewResult := assembler.loadBaseInstructions("review")
	if !strings.Contains(reviewResult, "## Code Style") {
		t.Errorf("review result missing ## Code Style")
	}
	if strings.Contains(reviewResult, "## Build Only") {
		t.Errorf("review result should not contain ## Build Only")
	}
	if !strings.Contains(reviewResult, "## Review Only") {
		t.Errorf("review result missing ## Review Only")
	}
}

func TestLoadProjectContextScopesArchitectureToMentionedPackage(t *testing.T) {
	project := `# Gromit

A Go CLI tool.

## Architecture

- ` + "`internal/v2/prompt/`" + ` — prompt assembly
- ` + "`internal/runner/`" + ` — core loop orchestration

## Key Principles

1. Fresh context each iteration
2. State in files, not memory`

	assembler := NewPromptAssembler("base", project, "instance", "fragment")
	got := assembler.loadProjectContext(BeadInfo{Title: "Modify internal/v2/prompt/assembler.go: implement loadProjectContext"})

	if !strings.Contains(got, "internal/v2/prompt/") {
		t.Errorf("expected scoped content to include internal/v2/prompt/, got: %q", got)
	}
	if strings.Contains(got, "internal/runner/") {
		t.Errorf("expected scoped content to exclude internal/runner/, got: %q", got)
	}
	if !strings.Contains(got, "Key Principles") {
		t.Errorf("expected scoped content to preserve Key Principles section, got: %q", got)
	}
}
