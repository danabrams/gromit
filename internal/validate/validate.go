package validate

import (
	"fmt"
	"strings"
)

const (
	// minOverlapLength is the minimum number of characters required for a
	// substantial overlap between acceptance criteria to be considered a violation.
	// Set to 25 to reduce false positives from short common phrases while still
	// catching genuine duplication between sibling beads.
	minOverlapLength = 25

	// maxExpectedOutputs is the maximum number of expected outputs allowed per bead.
	maxExpectedOutputs = 5
)

// scopeSignals contains keywords that may indicate over-scoping
var scopeSignals = []string{
	"refactor entire",
	"update all",
	"across all packages",
	"and also",
}

// BeadCandidate represents a bead definition to be validated
type BeadCandidate struct {
	Title              string
	Description        string
	AcceptanceCriteria []string
	ExpectedOutputs    []string
}

// Violation represents a validation rule violation
type Violation struct {
	BeadIndex int
	Rule      string
	Message   string
}

// CheckBeads validates a list of bead candidates and returns any violations
func CheckBeads(beads []BeadCandidate) []Violation {
	var violations []Violation

	for i, bead := range beads {
		// Check expected outputs count
		if len(bead.ExpectedOutputs) > maxExpectedOutputs {
			violations = append(violations, Violation{
				BeadIndex: i,
				Rule:      "output_count",
				Message:   fmt.Sprintf("Bead has more than %d expected outputs", maxExpectedOutputs),
			})
		}

		// Check criteria count
		if len(bead.AcceptanceCriteria) > 3 {
			violations = append(violations, Violation{
				BeadIndex: i,
				Rule:      "criteria_count",
				Message:   "Bead has more than 3 acceptance criteria",
			})
		}

		// Check for scope signals in title or description
		if containsScopeSignal(bead.Title) || containsScopeSignal(bead.Description) {
			violations = append(violations, Violation{
				BeadIndex: i,
				Rule:      "scope_signals",
				Message:   "Bead contains scope signal keywords that may indicate over-scoping",
			})
		}

		// Check for sibling overlap
		for j, other := range beads {
			if i >= j {
				continue // Only check each pair once, skip self
			}
			if hasOverlap(bead, other) {
				violations = append(violations, Violation{
					BeadIndex: i,
					Rule:      "sibling_overlap",
					Message:   "Bead has acceptance criteria that overlap with sibling beads",
				})
				break // Only report once per bead
			}
		}
	}

	return violations
}

// containsScopeSignal checks if text contains scope signal keywords
func containsScopeSignal(text string) bool {
	lower := strings.ToLower(text)
	for _, signal := range scopeSignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

// hasOverlap checks if two beads have overlapping acceptance criteria
func hasOverlap(a, b BeadCandidate) bool {
	for _, critA := range a.AcceptanceCriteria {
		for _, critB := range b.AcceptanceCriteria {
			if isSubstringMatch(critA, critB) {
				return true
			}
		}
	}
	return false
}

// isSubstringMatch checks if one criterion is a substring of another (case-insensitive)
func isSubstringMatch(a, b string) bool {
	lowerA := strings.ToLower(a)
	lowerB := strings.ToLower(b)

	if len(lowerA) >= minOverlapLength && strings.Contains(lowerB, lowerA) {
		return true
	}
	if len(lowerB) >= minOverlapLength && strings.Contains(lowerA, lowerB) {
		return true
	}

	return false
}
