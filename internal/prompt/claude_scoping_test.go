package prompt

import (
	"go/ast"
	"go/parser"
	"go/token"
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

func TestExtractScopedPackagePathsFromText_CollectsFromAllSources(t *testing.T) {
	spec := `
Research:
- Update internal/prompt/claude_scoping.go and internal/runner/run_iteration.go
- Touch cmd/gromit/main.go for wiring
`
	beadDescription := "Scope only internal/prompt/ and cmd/gromit/"
	parentDescription := "Parent context references internal/logger for follow-up."

	got := extractScopedPackagePathsFromText(spec, beadDescription, parentDescription)
	want := []string{
		"cmd/gromit/",
		"internal/logger/",
		"internal/prompt/",
		"internal/runner/",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected discovered package paths:\nwant: %#v\n got: %#v", want, got)
	}
}

func TestExtractScopedPackagePathsFromText_DeduplicatesAndNormalizes(t *testing.T) {
	spec := "internal/prompt internal/prompt/ ./internal/prompt/ cmd/gromit/... cmd/gromit/main.go"

	got := extractScopedPackagePathsFromText(spec, "", "")
	want := []string{
		"cmd/gromit/",
		"internal/prompt/",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized paths:\nwant: %#v\n got: %#v", want, got)
	}
}

func TestExtractScopedPackagePathsFromText_IgnoresInvalidOrIncompleteCandidates(t *testing.T) {
	spec := "ignore internal/ and cmd/ and random/path and internal/.hidden"

	got := extractScopedPackagePathsFromText(spec, "", "")
	if len(got) != 0 {
		t.Fatalf("expected no extracted paths, got %#v", got)
	}
}

func TestReadPackageSynopsis_PrioritizesDocGoEvenWhenOtherFilesAreLexicographicallyFirst(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file that comes before doc.go alphabetically
	abPath := filepath.Join(tmpDir, "abc.go")
	if err := os.WriteFile(abPath, []byte(`// This is a misleading comment from abc.go that should be ignored.
package test
`), 0644); err != nil {
		t.Fatalf("writing abc.go: %v", err)
	}

	// Create doc.go with the correct synopsis
	docGoPath := filepath.Join(tmpDir, "doc.go")
	if err := os.WriteFile(docGoPath, []byte(`// Package test provides core functionality with extra details.
package test
`), 0644); err != nil {
		t.Fatalf("writing doc.go: %v", err)
	}

	parsed, err := parser.ParseDir(token.NewFileSet(), tmpDir, func(info os.FileInfo) bool {
		return strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing directory: %v", err)
	}
	if len(parsed) == 0 {
		t.Fatal("expected parsed packages")
	}

	var pkg *ast.Package
	for _, p := range parsed {
		pkg = p
		break
	}

	synopsis := readPackageSynopsis(pkg)
	want := "Package test provides core functionality with extra details."
	if synopsis != want {
		t.Fatalf("unexpected synopsis:\nwant: %q\n got: %q (doc.go should be prioritized over abc.go)", want, synopsis)
	}
}
