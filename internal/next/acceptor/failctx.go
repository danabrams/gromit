package acceptor

import "fmt"

// AcceptanceFailure represents a non-passing criterion with context for replanning.
type AcceptanceFailure struct {
	Criterion    string   `json:"criterion"`
	Status       string   `json:"status"`
	Rationale    string   `json:"rationale"`
	EvidenceRefs []string `json:"evidence_refs"`
	Cycle        int      `json:"cycle"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for consistent JSON serialization.
func (af *AcceptanceFailure) NormalizeNilFields() {
	if af.EvidenceRefs == nil {
		af.EvidenceRefs = []string{}
	}
}

// BuildFailureContext extracts non-pass results from an AcceptanceResult for planner consumption.
func BuildFailureContext(result AcceptanceResult, cycle int) []AcceptanceFailure {
	failures := []AcceptanceFailure{}
	for _, cr := range result.Results {
		if cr.Status == StatusPass {
			continue
		}
		f := AcceptanceFailure{
			Criterion:    cr.Criterion,
			Status:       cr.Status,
			Rationale:    cr.Rationale,
			EvidenceRefs: cr.EvidenceRefs,
			Cycle:        cycle,
		}
		f.NormalizeNilFields()
		failures = append(failures, f)
	}
	return failures
}

// AcceptanceFailuresToStrings converts non-pass criterion results to human-readable strings
// suitable for FailureContext.Failures.
func AcceptanceFailuresToStrings(results []CriterionResult) []string {
	strs := []string{}
	for _, cr := range results {
		switch cr.Status {
		case StatusFail:
			strs = append(strs, fmt.Sprintf("acceptance:fail: %s — implement missing behavior", cr.Criterion))
		case StatusUnclear:
			strs = append(strs, fmt.Sprintf("acceptance:unclear: %s — add tests or evidence to prove/disprove", cr.Criterion))
		}
	}
	return strs
}
