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
	DependsOnIndex     []int
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
}

// DecomposeValidationMode selects mode-specific decompose validation behavior.
type DecomposeValidationMode string

const (
	DecomposeValidationModePipeline DecomposeValidationMode = "pipeline"
	DecomposeValidationModeRuntime  DecomposeValidationMode = "runtime"
)

// DecomposeOutputValidation returns per-bead and batch-level contract violations.
type DecomposeOutputValidation struct {
	Violations      []Violation
	BatchViolations []Violation
}

// ValidateDecomposeOutput is the shared decompose validation entry point used by
// decomposition workflows in pipeline and runtime/scope-gate modes.
func ValidateDecomposeOutput(beads []BeadCandidate, mode DecomposeValidationMode, parentTitle string) DecomposeOutputValidation {
	violations := CheckBeads(beads)
	if mode == DecomposeValidationModeRuntime && strings.TrimSpace(parentTitle) != "" {
		violations = CheckBeadsWithParentTitle(beads, parentTitle)
	}

	return DecomposeOutputValidation{
		Violations:      violations,
		BatchViolations: CheckBatchContract(beads),
	}
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
		classification := "low"
		reasons := []string{}
		if bead.EstimatedFiles > highComplexityFileThreshold {
			classification = "high"
			reasons = append(reasons, fmt.Sprintf("estimated_files=%d crosses the high-complexity threshold", bead.EstimatedFiles))
		}
		if containsScopeSignal(bead.Title) || containsScopeSignal(bead.Description) {
			classification = "high"
			reasons = append(reasons, "broad-scope language indicates oversized task")
		}
		if containsDependencySequencingSignal(bead.Title + " " + bead.Description) {
			classification = "high"
			reasons = append(reasons, "dependency-sequencing language indicates multi-phase task")
		}
		if classification == "high" {
			result.ComplexityOutcome.HighComplexity = append(result.ComplexityOutcome.HighComplexity, CandidateComplexityResult{
				Title:   bead.Title,
				Reasons: append([]string(nil), reasons...),
			})
			result.ComplexityOutcome.AggregateReasons = append(result.ComplexityOutcome.AggregateReasons, reasons...)
		}

		result.ComplexityResults = append(result.ComplexityResults, ComplexityResult{
			Title:          bead.Title,
			Classification: classification,
			Reasons:        reasons,
		})
	}

	result.ComplexityOutcome.HighCount = len(result.ComplexityOutcome.HighComplexity)
	return result
}

// CheckBeads validates a list of bead candidates and returns any violations
func CheckBeads(beads []BeadCandidate) []Violation {
	var violations []Violation

	for i, bead := range beads {
		if strings.TrimSpace(bead.Title) == "" {
			violations = append(violations, Violation{
				BeadIndex: i,
				Rule:      "title_empty",
				Message:   "Bead title is empty",
			})
		}

		if len(bead.ExpectedOutputs) == 0 {
			violations = append(violations, Violation{
				BeadIndex: i,
				Rule:      "output_missing",
				Message:   "Bead has no expected outputs",
			})
		}

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

func containsDependencySequencingSignal(text string) bool {
	lower := strings.ToLower(text)
	hasFirst := strings.Contains(lower, "first")
	hasThen := strings.Contains(lower, "then")
	hasFinally := strings.Contains(lower, "finally")

	return (hasFirst && hasThen) || (hasThen && hasFinally) || (hasFirst && hasFinally)
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
