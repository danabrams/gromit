package prompt

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseClaudeSections_IsolatesArchitectureAndKeyPrinciples(t *testing.T) {
	content := `# Project

Intro text.

## Architecture

Old architecture detail line 1.
Old architecture detail line 2.

## Key Principles

1. Keep behavior stable.
2. Prefer clear interfaces.

## Extra

Ignored.`

	sections := parseClaudeSections(content)

	if !sections.HasArchitecture {
		t.Fatal("expected Architecture section to be found")
	}
	if !sections.HasKeyPrinciples {
		t.Fatal("expected Key Principles section to be found")
	}

	expectedArchitecture := "Old architecture detail line 1.\nOld architecture detail line 2."
	if sections.ArchitectureBody != expectedArchitecture {
		t.Fatalf("unexpected Architecture body:\nwant: %q\n got: %q", expectedArchitecture, sections.ArchitectureBody)
	}

	expectedKey := "## Key Principles\n\n1. Keep behavior stable.\n2. Prefer clear interfaces."
	if sections.KeyPrinciplesSection != expectedKey {
		t.Fatalf("unexpected Key Principles section:\nwant: %q\n got: %q", expectedKey, sections.KeyPrinciplesSection)
	}
}

func TestRenderScopedClaudeContent_ReplacesArchitectureAndPreservesKeyPrinciples(t *testing.T) {
	content := `# Project

Project intro.

## Architecture

Old text that should be replaced.

## Key Principles

1. Keep this exact text.
2. Preserve formatting verbatim.`

	entries := []ScopedArchitectureEntry{
		{Path: "internal/prompt", Description: "Prompt rendering and shaping."},
		{Path: "cmd/gromit", Description: "CLI command entrypoints."},
	}

	rendered := renderScopedClaudeContent(content, entries)

	if strings.Contains(rendered, "Old text that should be replaced") {
		t.Fatal("expected old architecture text to be replaced")
	}

	expectedArchitecture := "## Architecture\n\n- `cmd/gromit/` — CLI command entrypoints.\n- `internal/prompt/` — Prompt rendering and shaping."
	if !strings.Contains(rendered, expectedArchitecture) {
		t.Fatalf("rendered Architecture section mismatch:\n%s", rendered)
	}

	expectedKey := "## Key Principles\n\n1. Keep this exact text.\n2. Preserve formatting verbatim."
	if !strings.Contains(rendered, expectedKey) {
		t.Fatalf("expected Key Principles section to remain verbatim, got:\n%s", rendered)
	}
}

func TestRenderScopedClaudeContent_MissingSectionsFallsBack(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "missing architecture",
			content: `# Project

## Key Principles

1. Keep this text.`,
		},
		{
			name: "missing key principles",
			content: `# Project

## Architecture

Old architecture text.`,
		},
		{
			name:    "no sections",
			content: "just plain text",
		},
	}

	entries := []ScopedArchitectureEntry{{Path: "internal/prompt", Description: "Prompt package."}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered := renderScopedClaudeContent(tc.content, entries)
			if rendered != tc.content {
				t.Fatalf("expected fallback to original content;\nwant: %q\n got: %q", tc.content, rendered)
			}
		})
	}
}

func TestRenderScopedArchitectureBullets_DeterministicFormatting(t *testing.T) {
	entries := []ScopedArchitectureEntry{
		{Path: "./internal/prompt/", Description: "Prompt rendering."},
		{Path: "cmd/gromit", Description: ""},
		{Path: "internal/prompt", Description: "Overridden description should be ignored due to first-write wins."},
		{Path: "", Description: "skip"},
	}

	got := renderScopedArchitectureBullets(entries)
	want := []string{
		"- `cmd/gromit/` — Task-relevant package context.",
		"- `internal/prompt/` — Prompt rendering.",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected bullets:\nwant: %#v\n got: %#v", want, got)
	}
}

func TestResolveScopedPackageDescription_PriorityOrder(t *testing.T) {
	repoRoot := t.TempDir()
	architectureBody := "- `alpha/` — CLAUDE architecture description."
	if err := os.MkdirAll(filepath.Join(repoRoot, "internal", "alpha"), 0755); err != nil {
		t.Fatalf("creating alpha package directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repoRoot, "internal", "alpha", "doc.go"), []byte(`// Package alpha provides alpha behavior. Extra sentence.
package alpha
`), 0644); err != nil {
		t.Fatalf("writing alpha package doc.go: %v", err)
	}

	claudeDescriptions := parseArchitectureBulletDescriptions(architectureBody)
	got := resolveScopedPackageDescription("internal/alpha", claudeDescriptions, repoRoot)
	want := "CLAUDE architecture description."
	if got != want {
		t.Fatalf("expected CLAUDE description to win:\nwant: %q\n got: %q", want, got)
	}
}

func TestResolveScopedPackageDescription_UsesGoDocFallback(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "internal", "beta"), 0755); err != nil {
		t.Fatalf("creating beta package directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repoRoot, "internal", "beta", "doc.go"), []byte(`// Package beta resolves package-level fallback text. Additional details are omitted.
package beta
`), 0644); err != nil {
		t.Fatalf("writing beta package doc.go: %v", err)
	}

	got := resolveScopedPackageDescription("internal/beta", map[string]string{}, repoRoot)
	want := "Package beta resolves package-level fallback text."
	if got != want {
		t.Fatalf("unexpected Go doc fallback description:\nwant: %q\n got: %q", want, got)
	}
}

func TestResolveScopedPackageDescription_UsesGenericFallback(t *testing.T) {
	repoRoot := t.TempDir()
	got := resolveScopedPackageDescription("internal/missing", map[string]string{}, repoRoot)
	if got != scopedDescriptionFallback {
		t.Fatalf("unexpected generic fallback:\nwant: %q\n got: %q", scopedDescriptionFallback, got)
	}
}

func TestResolveScopedArchitectureEntries_AlwaysPopulatesDescriptions(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "internal", "beta"), 0755); err != nil {
		t.Fatalf("creating beta package directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "internal", "beta", "doc.go"), []byte(`// Package beta uses docs for scoped architecture entries.
package beta
`), 0644); err != nil {
		t.Fatalf("writing beta package doc.go: %v", err)
	}

	architectureBody := "- `alpha/` — CLAUDE architecture description."
	entries := resolveScopedArchitectureEntries(
		[]string{"internal/alpha", "internal/beta", "internal/missing"},
		architectureBody,
		repoRoot,
	)

	want := []ScopedArchitectureEntry{
		{Path: "internal/alpha/", Description: "CLAUDE architecture description."},
		{Path: "internal/beta/", Description: "Package beta uses docs for scoped architecture entries."},
		{Path: "internal/missing/", Description: scopedDescriptionFallback},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("unexpected resolved entries:\nwant: %#v\n got: %#v", want, entries)
	}
}
