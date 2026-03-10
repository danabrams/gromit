// Package validation manages the validation.json artifact and validation
// result tracking for the new architecture.
//
// This is distinct from legacy internal/validate/ — it operates on
// workspace-stored artifacts rather than repo-local state.
//
// TODO: implement validation result capture
// TODO: implement validation history tracking across runs
// TODO: implement validation-driven remediation hints
package validation

// ValidationResult represents the validation.json artifact.
//
// Captures the outcome of test/lint/build validation passes.
//
// TODO: define full schema (pass/fail categories, error fingerprints)
type ValidationResult struct {
	// Passed indicates whether all validation checks passed.
	Passed bool `json:"passed"`

	// Failures lists individual validation failures.
	Failures []Failure `json:"failures,omitempty"`
}

// Failure represents a single validation failure.
type Failure struct {
	Category string `json:"category"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}
