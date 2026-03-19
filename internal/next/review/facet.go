package review

import (
	"fmt"
	"sort"
)

// FacetDef defines a built-in review facet.
type FacetDef struct {
	Name           string
	Description    string
	DefaultTier    string
	PromptTemplate string
}

// basePromptBody is the shared template body used by all facets.
// Each facet prepends its own role preamble to this body.
const basePromptBody = `

## Facet: {{.FacetDef.Name}}

## Spec Content
{{.SpecContent}}

## Code Diff
{{.DiffSummary}}

{{if .PriorFindings}}
## Prior Findings (from previous cycles)
Label each current finding as "new" or "pre-existing" by comparing against these:
{{range .PriorFindings}}- [{{.Severity}}] {{.File}}: {{.Description}}
{{end}}
{{end}}

CRITICAL OUTPUT FORMAT RULE: Your entire response must be a valid JSON array and nothing else. Do not include any text before or after the JSON. Do not wrap the JSON in markdown code fences. Do not add any explanation.

Each finding object must have these fields: "severity" (error/warning/suggestion/info), "file", "line" (integer), "description", "suggested_fix"{{if .PriorFindings}}, "disposition" (new/pre-existing){{end}}.

If no issues found, your entire response must be exactly: []

Your response (JSON array only):`

// Registry holds the set of built-in review facets.
type Registry struct {
	facets map[string]FacetDef
}

// NewRegistry returns a registry populated with all 5 built-in facets.
func NewRegistry() *Registry {
	r := &Registry{facets: map[string]FacetDef{
		"spec_alignment": {
			Name:           "spec_alignment",
			Description:    "Does the diff implement what the spec asked for?",
			DefaultTier:    "high",
			PromptTemplate: `You are a spec-alignment reviewer. Your task is to compare the code diff against the spec and identify any gaps, missing requirements, or deviations.` + basePromptBody,
		},
		"code_quality": {
			Name:           "code_quality",
			Description:    "Naming, structure, duplication, readability",
			DefaultTier:    "medium",
			PromptTemplate: `You are a code quality reviewer. Your task is to identify code smells, dead code, overly complex functions, naming issues, and duplication.` + basePromptBody,
		},
		"logic_gaps": {
			Name:           "logic_gaps",
			Description:    "Off-by-one, nil handling, missing error paths",
			DefaultTier:    "high",
			PromptTemplate: `You are a logic reviewer. Your task is to identify off-by-one errors, nil pointer risks, missing error handling, race conditions, and other logical issues.` + basePromptBody,
		},
		"test_coverage": {
			Name:           "test_coverage",
			Description:    "Are there untested code paths, missing edge cases, or inadequate assertions?",
			DefaultTier:    "medium",
			PromptTemplate: `You are a test coverage reviewer. Your task is to identify untested code paths, missing edge case tests, and inadequate assertions.` + basePromptBody,
		},
		"architecture_drift": {
			Name:           "architecture_drift",
			Description:    "Does the change respect boundaries from the project cell?",
			DefaultTier:    "medium",
			PromptTemplate: `You are an architecture reviewer. Your task is to identify violations of architectural boundaries, improper dependencies, and structural drift from the intended design.` + basePromptBody,
		},
	}}
	return r
}

// Get returns the facet definition by name.
func (r *Registry) Get(name string) (FacetDef, bool) {
	f, ok := r.facets[name]
	return f, ok
}

// ListNames returns all facet names in sorted order.
func (r *Registry) ListNames() []string {
	names := make([]string, 0, len(r.facets))
	for name := range r.facets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Select returns facet definitions for the given names. Returns an error if any
// name is unknown or if the list is empty.
func (r *Registry) Select(names []string) ([]FacetDef, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("at least one facet must be selected")
	}
	defs := make([]FacetDef, 0, len(names))
	for _, name := range names {
		f, ok := r.facets[name]
		if !ok {
			return nil, fmt.Errorf("unknown facet: %q", name)
		}
		defs = append(defs, f)
	}
	return defs, nil
}
