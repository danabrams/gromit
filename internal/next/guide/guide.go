// Package guide renders agent-guide.md from inspection artifacts and doctrine.
//
// The agent guide is a compiled markdown document that provides an LLM agent
// with everything it needs to work effectively in a project: architecture
// overview, key conventions, file inventory, and domain glossary.
package guide

import (
	"bytes"
	"fmt"
)

// Module describes a high-level architectural module.
type Module struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
}

// SourceMapEntry describes a single file in the source map.
type SourceMapEntry struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
}

// ValidationCommand describes a validation command to run.
type ValidationCommand struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Run    string `json:"run"`
	Source string `json:"source"`
}

// DoctrineRule describes a single doctrine rule.
type DoctrineRule struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Scope   string `json:"scope"`
}

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

// InferredObservation represents an inferred fact for guide rendering.
type InferredObservation struct {
	Category   string `json:"category"`
	Statement  string `json:"statement"`
	Confidence string `json:"confidence"`
}

// RenderInput holds everything needed to render an agent guide.
type RenderInput struct {
	ProjectName     string
	Modules         []Module
	SourceMap       []SourceMapEntry
	Validation      []ValidationCommand
	Risks           []Risk
	Invariants      []Invariant
	Glossary        []GlossaryEntry
	Doctrine        []DoctrineRule
	InferredFacts   []InferredObservation `json:"inferred_facts"`
	IncludeInferred bool                  `json:"include_inferred"`
}

// NormalizeNilFields maps nil slices to empty values.
func (r *RenderInput) NormalizeNilFields() {
	if r.Modules == nil {
		r.Modules = []Module{}
	}
	if r.SourceMap == nil {
		r.SourceMap = []SourceMapEntry{}
	}
	if r.Validation == nil {
		r.Validation = []ValidationCommand{}
	}
	if r.Risks == nil {
		r.Risks = []Risk{}
	}
	if r.Invariants == nil {
		r.Invariants = []Invariant{}
	}
	if r.Glossary == nil {
		r.Glossary = []GlossaryEntry{}
	}
	if r.Doctrine == nil {
		r.Doctrine = []DoctrineRule{}
	}
	if r.InferredFacts == nil {
		r.InferredFacts = []InferredObservation{}
	}
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

	// Always render project overview
	fmt.Fprintf(&buf, "## Project Overview\n\n")
	fmt.Fprintf(&buf, "**Project:** %s\n\n", input.ProjectName)

	// Architecture section - only if modules exist
	if len(input.Modules) > 0 {
		fmt.Fprintf(&buf, "## Architecture\n\n")
		for _, m := range input.Modules {
			fmt.Fprintf(&buf, "- **%s** (%s): %s\n", m.Name, m.Language, m.Description)
		}
		buf.WriteString("\n")
	}

	// Source Map section - only if entries exist
	if len(input.SourceMap) > 0 {
		fmt.Fprintf(&buf, "## Source Map\n\n")
		for _, e := range input.SourceMap {
			fmt.Fprintf(&buf, "- `%s` (%s, %d lines)\n", e.Path, e.Language, e.Lines)
		}
		buf.WriteString("\n")
	}

	// Validation section - only if commands exist
	if len(input.Validation) > 0 {
		fmt.Fprintf(&buf, "## Validation\n\n")
		for _, cmd := range input.Validation {
			fmt.Fprintf(&buf, "- **%s** [%s]: `%s` (from %s)\n", cmd.Name, cmd.Kind, cmd.Run, cmd.Source)
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
	if len(input.Doctrine) > 0 {
		fmt.Fprintf(&buf, "## Doctrine\n\n")
		for _, rule := range input.Doctrine {
			fmt.Fprintf(&buf, "- [%s] %s (scope: %s)\n", rule.ID, rule.Summary, rule.Scope)
		}
		buf.WriteString("\n")
	}

	// Inferred sections - only if IncludeInferred is true and facts exist
	if input.IncludeInferred && len(input.InferredFacts) > 0 {
		// Category-to-section mapping
		sectionMap := map[string]string{
			"component_boundary":       "Inferred Component Structure",
			"component_responsibility": "Inferred Component Structure",
			"entrypoint":               "Inferred Likely Entrypoints",
			"risky_area":               "Inferred Risky Areas",
			"integration_point":        "Inferred Integration Points",
			"glossary_term":            "Inferred Glossary",
			"likely_validation_surface": "Inferred Validation Surfaces",
			"likely_ownership_boundary": "Inferred Ownership Boundaries",
		}

		// Ordered section names to ensure deterministic output
		sectionOrder := []string{
			"Inferred Component Structure",
			"Inferred Likely Entrypoints",
			"Inferred Risky Areas",
			"Inferred Integration Points",
			"Inferred Glossary",
			"Inferred Validation Surfaces",
			"Inferred Ownership Boundaries",
		}

		// Group facts by section
		grouped := make(map[string][]InferredObservation)
		for _, fact := range input.InferredFacts {
			section, ok := sectionMap[fact.Category]
			if !ok {
				section = "Inferred Other"
			}
			grouped[section] = append(grouped[section], fact)
		}

		// Render each section in order
		for _, section := range sectionOrder {
			facts, ok := grouped[section]
			if !ok {
				continue
			}
			fmt.Fprintf(&buf, "## %s [INFERRED]\n\n", section)
			for _, fact := range facts {
				fmt.Fprintf(&buf, "- %s (confidence: %s)\n", fact.Statement, fact.Confidence)
			}
			buf.WriteString("\n")
		}

		// Render any uncategorized facts
		if facts, ok := grouped["Inferred Other"]; ok {
			fmt.Fprintf(&buf, "## Inferred Other [INFERRED]\n\n")
			for _, fact := range facts {
				fmt.Fprintf(&buf, "- %s (confidence: %s)\n", fact.Statement, fact.Confidence)
			}
			buf.WriteString("\n")
		}
	}

	return buf.Bytes(), nil
}
