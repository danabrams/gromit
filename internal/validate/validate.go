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

	// minSubBeads is the minimum number of sub-beads required from a decomposition.
	minSubBeads = 2

	// MaxSubBeads is the maximum number of sub-beads allowed from a decomposition.
	MaxSubBeads = 5
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

// CheckBatchContract validates batch-level constraints on the full set of decomposed beads.
// These are hard structural rules checked after the reprompt loop.
func CheckBatchContract(beads []BeadCandidate) []Violation {
	var violations []Violation

	if len(beads) < minSubBeads {
		violations = append(violations, Violation{
			BeadIndex: -1,
			Rule:      "batch_size_min",
			Message:   fmt.Sprintf("Decomposition produced %d sub-bead(s); minimum is %d", len(beads), minSubBeads),
		})
	}

	if len(beads) > MaxSubBeads {
		violations = append(violations, Violation{
			BeadIndex: -1,
			Rule:      "batch_size_max",
			Message:   fmt.Sprintf("Decomposition produced %d sub-beads; maximum is %d", len(beads), MaxSubBeads),
		})
	}

	return violations
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

		// Check for empty expected outputs
		for _, output := range bead.ExpectedOutputs {
			if strings.TrimSpace(output) == "" {
				violations = append(violations, Violation{
					BeadIndex: i,
					Rule:      "output_empty",
					Message:   "Bead has an empty expected output entry",
				})
				break
			}
		}

		// Check for duplicate expected outputs
		if hasDuplicateOutputs(bead.ExpectedOutputs) {
			violations = append(violations, Violation{
				BeadIndex: i,
				Rule:      "output_duplicate",
				Message:   "Bead has duplicate expected output entries",
			})
		}

		// Check for parent echo: expected output exactly matches bead title
		for _, output := range bead.ExpectedOutputs {
			if output == bead.Title {
				violations = append(violations, Violation{
					BeadIndex: i,
					Rule:      "parent_echo",
					Message:   "Bead has an expected output that exactly echoes the bead title",
				})
				break
			}
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

// hasDuplicateOutputs checks if a list of expected outputs contains duplicates (case-sensitive)
func hasDuplicateOutputs(outputs []string) bool {
	seen := make(map[string]bool, len(outputs))
	for _, output := range outputs {
		if seen[output] {
			return true
		}
		seen[output] = true
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
