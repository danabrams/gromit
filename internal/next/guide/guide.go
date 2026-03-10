// Package guide renders agent-guide.md from inspection artifacts and doctrine.
//
// The agent guide is a compiled markdown document that provides an LLM agent
// with everything it needs to work effectively in a project: architecture
// overview, key conventions, file inventory, and domain glossary.
package guide

import (
	"bytes"
	"fmt"

	"github.com/danabrams/gromit/internal/next/architecture"
	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/sourcemap"
	"github.com/danabrams/gromit/internal/next/validation"
)

// GlossaryEntry defines a domain term and its definition.
type GlossaryEntry struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// Risk describes a known risky area in the codebase.
type Risk struct {
	Area        string `json:"area"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// Invariant describes a rule that must always hold.
type Invariant struct {
	Rule        string `json:"rule"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
}

// RenderInput holds everything needed to render an agent guide.
type RenderInput struct {
	ProjectName  string
	Architecture architecture.Architecture
	SourceMap    sourcemap.SourceMap
	Validation   validation.CommandSet
	Risks        []Risk
	Invariants   []Invariant
	Glossary     []GlossaryEntry
	Doctrine     doctrine.Doctrine
}

// Renderer produces agent-guide markdown from a RenderInput.
type Renderer interface {
	Render(input RenderInput) ([]byte, error)
}

// MarkdownRenderer renders agent guides as markdown.
type MarkdownRenderer struct{}

// NewMarkdownRenderer returns a new MarkdownRenderer.
func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{}
}

// Render produces markdown bytes from the given RenderInput.
// Empty sections are omitted entirely.
func (r *MarkdownRenderer) Render(input RenderInput) ([]byte, error) {
	var buf bytes.Buffer

	// Always render project heading
	fmt.Fprintf(&buf, "# %s\n\n", input.ProjectName)

	// Architecture section - only if modules exist
	if len(input.Architecture.Modules) > 0 {
		fmt.Fprintf(&buf, "## Architecture\n\n")
		for _, m := range input.Architecture.Modules {
			fmt.Fprintf(&buf, "- **%s** (%s): %s\n", m.Name, m.Language, m.Description)
		}
		buf.WriteString("\n")
	}

	// Source Map section - only if entries exist
	if len(input.SourceMap.Entries) > 0 {
		fmt.Fprintf(&buf, "## Source Map\n\n")
		for _, e := range input.SourceMap.Entries {
			fmt.Fprintf(&buf, "- `%s` (%s, %d lines)\n", e.Path, e.Language, e.Lines)
		}
		buf.WriteString("\n")
	}

	// Validation section - only if commands exist
	if len(input.Validation.Commands) > 0 {
		fmt.Fprintf(&buf, "## Validation\n\n")
		for _, cmd := range input.Validation.Commands {
			fmt.Fprintf(&buf, "- **%s** [%s]: `%s` (from %s)\n", cmd.Name, cmd.Kind.String(), cmd.Run, cmd.Source)
		}
		buf.WriteString("\n")
	}

	// Risky Areas - only if risks exist
	if len(input.Risks) > 0 {
		fmt.Fprintf(&buf, "## Risky Areas\n\n")
		for _, ri := range input.Risks {
			fmt.Fprintf(&buf, "- **%s** [%s]: %s\n", ri.Area, ri.Severity, ri.Description)
		}
		buf.WriteString("\n")
	}

	// Invariants - only if they exist
	if len(input.Invariants) > 0 {
		fmt.Fprintf(&buf, "## Invariants\n\n")
		for _, inv := range input.Invariants {
			fmt.Fprintf(&buf, "- **%s** (%s): %s\n", inv.Rule, inv.Scope, inv.Description)
		}
		buf.WriteString("\n")
	}

	// Glossary - only if entries exist
	if len(input.Glossary) > 0 {
		fmt.Fprintf(&buf, "## Glossary\n\n")
		for _, g := range input.Glossary {
			fmt.Fprintf(&buf, "- **%s**: %s\n", g.Term, g.Definition)
		}
		buf.WriteString("\n")
	}

	// Doctrine - only if rules exist
	if len(input.Doctrine.Rules) > 0 {
		fmt.Fprintf(&buf, "## Doctrine\n\n")
		for _, rule := range input.Doctrine.Rules {
			fmt.Fprintf(&buf, "- [%s] %s (scope: %s)\n", rule.ID, rule.Summary, rule.Scope)
		}
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}
