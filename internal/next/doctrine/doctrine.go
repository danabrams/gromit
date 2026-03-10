// Package doctrine manages project-level coding standards, conventions,
// and rules that guide agent behavior.
//
// Doctrine is the successor to legacy RULES.md / LEARNINGS.md. It provides
// structured, queryable conventions rather than flat markdown.
//
// TODO: implement doctrine loading from project cell
// TODO: implement doctrine merging (project + workspace defaults)
// TODO: implement doctrine querying by topic/scope
package doctrine

// Doctrine represents the full set of rules and conventions for a project.
type Doctrine struct {
	// Rules is the list of coding rules and conventions.
	Rules []Rule `json:"rules"`
}

// Rule is a single convention or standard.
type Rule struct {
	// ID is a stable identifier for this rule.
	ID string `json:"id"`

	// Summary is a short human-readable description.
	Summary string `json:"summary"`

	// Scope limits where this rule applies (e.g. "tests", "api", "*").
	Scope string `json:"scope,omitempty"`
}
