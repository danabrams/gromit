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

	// highComplexityFileThreshold marks candidates above this estimate as high complexity.
	highComplexityFileThreshold = 5
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
	EstimatedFiles     int
	AcceptanceCriteria []string
	ExpectedOutputs    []string
}

// Violation represents a validation rule violation
type Violation struct {
	BeadIndex int
	Rule      string
	Message   string
}

// ComplexityResult describes the complexity classification for one candidate.
type ComplexityResult struct {
	Title          string
	Classification string
	Reasons        []string
}

// ComplexityOutcome summarizes high-complexity candidates across a batch.
type ComplexityOutcome struct {
	HighCount      int
	HighComplexity []CandidateComplexityResult
	AggregateReasons []string
}

// ValidationResult aggregates validation and complexity outputs for decompose candidates.
type ValidationResult struct {
	Violations        []Violation
	ComplexityResults []ComplexityResult
	ComplexityOutcome ComplexityOutcome

	// Legacy fields kept for backward compatibility with older call sites.
	HighComplexityCount int
	HighComplexity      []CandidateComplexityResult
	ComplexityReasons   []string
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

// ValidateDecomposeCandidates validates decompose candidates and returns complexity metadata.
func ValidateDecomposeCandidates(beads []BeadCandidate) ValidationResult {
	result := ValidationResult{
		Violations:        CheckBeads(beads),
		ComplexityResults: make([]ComplexityResult, 0, len(beads)),
	}

	for _, bead := range beads {
		score := ScoreCandidate(bead)
		if score.Classification == "high" {
			result.ComplexityOutcome.HighComplexity = append(result.ComplexityOutcome.HighComplexity, CandidateComplexityResult{
				Title:   score.Title,
				Reasons: append([]string(nil), score.Reasons...),
			})
			result.ComplexityOutcome.AggregateReasons = append(result.ComplexityOutcome.AggregateReasons, score.Reasons...)
		}

		result.ComplexityResults = append(result.ComplexityResults, score)
	}

	result.ComplexityOutcome.HighCount = len(result.ComplexityOutcome.HighComplexity)
	result.HighComplexityCount = result.ComplexityOutcome.HighCount
	result.HighComplexity = result.ComplexityOutcome.HighComplexity
	result.ComplexityReasons = result.ComplexityOutcome.AggregateReasons
	return result
}

// ScoreCandidate classifies one candidate's complexity and records scoring reasons.
func ScoreCandidate(bead BeadCandidate) ComplexityResult {
	result := ComplexityResult{
		Title:          bead.Title,
		Classification: "low",
	}

	if bead.EstimatedFiles > highComplexityFileThreshold {
		result.Reasons = append(result.Reasons, fmt.Sprintf("estimated_files=%d crosses the high-complexity threshold", bead.EstimatedFiles))
	}
	if len(bead.AcceptanceCriteria) >= 3 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("acceptance_criteria=%d indicates broad implementation surface", len(bead.AcceptanceCriteria)))
	}
	if len(bead.ExpectedOutputs) >= 4 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected_outputs=%d indicates multiple coupled deliverables", len(bead.ExpectedOutputs)))
	}
	titleHasScopeSignal := containsScopeSignal(bead.Title)
	descriptionHasScopeSignal := containsScopeSignal(bead.Description)
	if titleHasScopeSignal {
		result.Reasons = append(result.Reasons, "contains broad-scope language in title")
	}
	if descriptionHasScopeSignal {
		result.Reasons = append(result.Reasons, "contains broad-scope language in description")
	}
	if len(result.Reasons) > 0 {
		result.Classification = "high"
	} else {
		result.Reasons = append(result.Reasons, "no high-complexity signals detected")
	}

	return result
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

// CheckBeadsWithParentTitle validates bead candidates with parent context.
// It applies CheckBeads rules and also flags expected outputs that echo the parent title.
func CheckBeadsWithParentTitle(beads []BeadCandidate, parentTitle string) []Violation {
	violations := CheckBeads(beads)
	if strings.TrimSpace(parentTitle) == "" {
		return violations
	}

	for i, bead := range beads {
		for _, output := range bead.ExpectedOutputs {
			if output == parentTitle {
				violations = append(violations, Violation{
					BeadIndex: i,
					Rule:      "parent_echo",
					Message:   "Bead has an expected output that exactly echoes the parent title",
				})
				break
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
