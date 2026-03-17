package enrich

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/next/fact"
)

// categoryDescriptions provides per-category instructions for the LLM.
var categoryDescriptions = map[EnrichmentCategory]string{
	CategoryComponentBoundary: `Identify the major component boundaries in this codebase.
A component boundary is a clear separation between distinct functional areas,
typically represented by package directories, module boundaries, or service interfaces.
Focus on boundaries that are architecturally significant.`,

	CategoryComponentResponsibility: `Identify the primary responsibility of each major component.
A component responsibility is a concise description of what a package, module, or service
is responsible for. Focus on the single most important responsibility of each component.`,

	CategoryEntrypoint: `Identify the entrypoints of this codebase.
An entrypoint is a location where execution begins or where external input is received.
This includes main functions, HTTP handlers, CLI commands, event listeners, and similar.`,

	CategoryRiskyArea: `Identify areas of the codebase that are risky or fragile.
A risky area is code that is likely to cause bugs, is hard to test, has complex logic,
lacks test coverage, or handles critical operations like data persistence or authentication.`,

	CategoryIntegrationPoint: `Identify the integration points in this codebase.
An integration point is where this system communicates with external systems, services,
databases, APIs, file systems, or other processes.`,

	CategoryGlossaryTerm: `Identify important domain-specific terms used in this codebase.
A glossary term is a word or phrase that has a specific meaning within this project's domain.
Include terms that a new developer would need to understand to work on the code.`,

	CategoryValidationSurface: `Identify the likely validation surfaces in this codebase.
A validation surface is a location where input data is validated, sanitized, or checked
for correctness. This includes form validation, API request validation, schema validation,
and similar checks.`,

	CategoryOwnershipBoundary: `Identify the likely ownership boundaries in this codebase.
An ownership boundary is a division of the codebase that is likely maintained by different
teams or individuals. Look for patterns like directory structure, naming conventions,
or configuration that suggest distinct ownership.`,
}

// buildPrompt constructs a prompt for the LLM enrichment pass.
func buildPrompt(category EnrichmentCategory, observed []fact.Fact, input EnrichInput) string {
	var b strings.Builder

	// System instruction
	fmt.Fprintf(&b, "You are analyzing a software project to infer facts about its structure.\n\n")

	// Project context
	if input.ProjectName != "" {
		fmt.Fprintf(&b, "## Project: %s\n\n", input.ProjectName)
	}

	// Category-specific instructions
	desc, ok := categoryDescriptions[category]
	if !ok {
		desc = fmt.Sprintf("Identify facts related to the category: %s", category)
	}
	fmt.Fprintf(&b, "## Task: %s\n\n%s\n\n", category, desc)

	// File tree context — summarize by directory for large repos
	if len(input.FileTree) > 0 {
		fmt.Fprintf(&b, "## File Tree\n\n")
		if len(input.FileTree) <= 1000 {
			// Small enough to list every file
			for _, f := range input.FileTree {
				fmt.Fprintf(&b, "- %s\n", f)
			}
		} else {
			// Summarize: count files per directory
			dirCounts := make(map[string]int)
			for _, f := range input.FileTree {
				dir := filepath.Dir(f)
				if dir == "." {
					dir = "(root)"
				}
				dirCounts[dir]++
			}
			dirs := make([]string, 0, len(dirCounts))
			for d := range dirCounts {
				dirs = append(dirs, d)
			}
			sort.Strings(dirs)
			fmt.Fprintf(&b, "(%d files across %d directories)\n\n", len(input.FileTree), len(dirs))
			for _, d := range dirs {
				fmt.Fprintf(&b, "- %s/ (%d files)\n", d, dirCounts[d])
			}
		}
		fmt.Fprintln(&b)
	}

	// Architecture context
	if input.Architecture != "" {
		fmt.Fprintf(&b, "## Architecture\n\n%s\n\n", input.Architecture)
	}

	// Doctrine context
	if input.Doctrine != "" {
		fmt.Fprintf(&b, "## Doctrine\n\n%s\n\n", input.Doctrine)
	}

	// Source map context
	if input.SourceMap != "" {
		fmt.Fprintf(&b, "## Source Map\n\n%s\n\n", input.SourceMap)
	}

	// Glossary context
	if input.Glossary != "" {
		fmt.Fprintf(&b, "## Glossary\n\n%s\n\n", input.Glossary)
	}

	// Validation context
	if input.Validation != "" {
		fmt.Fprintf(&b, "## Validation Commands\n\n%s\n\n", input.Validation)
	}

	// Observed facts
	if len(observed) > 0 {
		fmt.Fprintf(&b, "## Observed Facts\n\n")
		for _, f := range observed {
			fmt.Fprintf(&b, "- [%s] %s (source: %s)\n", f.ID, f.Content, f.Source)
		}
		fmt.Fprintln(&b)
	}

	// Output format instructions
	fmt.Fprintf(&b, `## Output Format

Return ONLY a JSON array of objects. Each object must have these fields:
- "statement": a concise factual claim (string)
- "rationale": why you believe this is true (string)
- "evidence_refs": list of file paths or identifiers supporting this (array of strings)
- "confidence": one of "high", "medium", or "low" (string)
- "scope": the part of the codebase this applies to (string)

Example:
[
  {
    "statement": "The auth package handles all authentication",
    "rationale": "It contains JWT validation and OAuth2 flows",
    "evidence_refs": ["internal/auth/jwt.go", "internal/auth/oauth.go"],
    "confidence": "high",
    "scope": "internal/auth"
  }
]

Return ONLY the JSON array, no markdown fences, no commentary.
`)

	return b.String()
}
