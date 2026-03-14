package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestExtractSpecConstraints_BothSections(t *testing.T) {
	input := `## Overview
Some overview text.

## Out-of-Scope
- Do NOT modify any existing test files
- No changes to existing functions

## Architectural Constraints
- All code stays in the ` + "`calc`" + ` package
- Existing tests must not be modified

## Some Other Section
Irrelevant content.
`
	got := extractSpecConstraints(input)
	want := "## Out-of-Scope\n- Do NOT modify any existing test files\n- No changes to existing functions\n\n## Architectural Constraints\n- All code stays in the `calc` package\n- Existing tests must not be modified"
	if got != want {
		t.Fatalf("extractSpecConstraints mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestExtractSpecConstraints_OnlyOutOfScope(t *testing.T) {
	input := `## Out-of-Scope
- Do NOT modify any existing test files

## Some Other Section
Other content.
`
	got := extractSpecConstraints(input)
	want := "## Out-of-Scope\n- Do NOT modify any existing test files"
	if got != want {
		t.Fatalf("extractSpecConstraints mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestExtractSpecConstraints_OnlyArchitecturalConstraints(t *testing.T) {
	input := `## Overview
Overview text.

## Architectural Constraints
- All code stays in the calc package
- Existing tests must not be modified
`
	got := extractSpecConstraints(input)
	want := "## Architectural Constraints\n- All code stays in the calc package\n- Existing tests must not be modified"
	if got != want {
		t.Fatalf("extractSpecConstraints mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestExtractSpecConstraints_NeitherSection(t *testing.T) {
	input := `## Overview
Some overview text.

## Goals
- Goal one
- Goal two
`
	got := extractSpecConstraints(input)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestExtractSpecConstraints_StopsAtNextHeading(t *testing.T) {
	input := `## Out-of-Scope
- Only this line

## Unrelated
- Should not be included

## Architectural Constraints
- Also only this line

## Another Section
- Also excluded
`
	got := extractSpecConstraints(input)
	want := "## Out-of-Scope\n- Only this line\n\n## Architectural Constraints\n- Also only this line"
	if got != want {
		t.Fatalf("extractSpecConstraints mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

type fakeCompiler struct {
	content string
	err     error
}

func (f *fakeCompiler) Compile(ctx context.Context) (string, error) {
	return f.content, f.err
}

// Verify CompileStage satisfies the Stage interface.
var _ specloop.Stage = (*CompileStage)(nil)

func TestCompileStage_WritesSpecPacket(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")

	// Create run dir
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)

	compiler := &fakeCompiler{content: "compiled content"}
	stage := NewCompileStage(compiler, store, nil)

	if stage.Name() != "compile" {
		t.Fatalf("expected name 'compile', got %q", stage.Name())
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	packetPath := filepath.Join(runDir, "spec-packet.md")
	data, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatalf("spec-packet.md not written: %v", err)
	}
	if string(data) != "compiled content" {
		t.Fatalf("expected 'compiled content', got %q", string(data))
	}
}
