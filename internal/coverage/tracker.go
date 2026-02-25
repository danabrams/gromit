package coverage

import (
	"fmt"
	"strings"
)

// State represents the lifecycle state of the coverage tracker.
type State int

const (
	StatePending State = iota
	StateCollecting
	StateValidating
	StateComplete
	StateError
)

// Status represents the coverage state of a single criterion.
type Status int

const (
	Unchecked Status = iota
	Covered
	Untestable
)

// CriterionState tracks a criterion and its coverage status.
type CriterionState struct {
	Criterion
	Status         Status
	RejectionCount int
}

// CoverageTracker manages a checklist of criteria states.
type CoverageTracker struct {
	state         State
	criteria      []CriterionState
	maxRejections int
}

// NewTracker creates a CoverageTracker from a list of criteria.
func NewTracker(criteria []Criterion, maxRejections int) *CoverageTracker {
	states := make([]CriterionState, len(criteria))
	for i, c := range criteria {
		states[i] = CriterionState{Criterion: c, Status: Unchecked}
	}
	return &CoverageTracker{state: StatePending, criteria: states, maxRejections: maxRejections}
}

// State returns the current lifecycle state of the tracker.
func (t *CoverageTracker) State() State {
	return t.state
}

// RecordRejection increments rejection count; transitions to Untestable at threshold.
func (t *CoverageTracker) RecordRejection(n int) {
	cs := t.findCriterionState(n)
	if cs == nil || cs.Status != Unchecked {
		return
	}

	cs.RejectionCount++
	if cs.RejectionCount >= t.maxRejections {
		cs.Status = Untestable
	}
}

// MarkCovered transitions a criterion to Covered by its number.
func (t *CoverageTracker) MarkCovered(n int) {
	cs := t.findCriterionState(n)
	if cs == nil || cs.Status != Unchecked {
		return
	}

	cs.Status = Covered
}

// FormatCoverageState renders the coverage state block for prompt injection.
func (t *CoverageTracker) FormatCoverageState(targeting int) string {
	var targetText string
	for _, cs := range t.criteria {
		if cs.Number == targeting {
			targetText = cs.Text
			break
		}
	}

	uncovered := t.UncoveredCriteria()
	return fmt.Sprintf("## Coverage State\nTargeting criterion #%d: %q\nRemaining uncovered: %s",
		targeting, targetText, formatCriteriaNumbers(uncovered))
}

// Summary renders a human-readable coverage summary for bead comments.
func (t *CoverageTracker) Summary() string {
	total := len(t.criteria)
	covered := 0
	for _, cs := range t.criteria {
		if cs.Status == Covered {
			covered++
		}
	}

	uncovered := t.UncoveredCriteria()
	untestable := t.UntestableCriteria()

	var sb strings.Builder
	fmt.Fprintf(&sb, "Coverage: %d/%d criteria covered", covered, total)

	if len(uncovered) > 0 {
		fmt.Fprintf(&sb, "\nUncovered: %s", formatCriteriaNumbers(uncovered))
	}

	if len(untestable) > 0 {
		fmt.Fprintf(&sb, "\nUntestable: %s", formatCriteriaNumbers(untestable))
	}

	return sb.String()
}

func formatCriteriaNumbers(criteria []CriterionState) string {
	parts := make([]string, len(criteria))
	for i, cs := range criteria {
		parts[i] = fmt.Sprintf("#%d", cs.Number)
	}
	return strings.Join(parts, ", ")
}

// UntestableCriteria returns all criteria in Untestable state.
func (t *CoverageTracker) UntestableCriteria() []CriterionState {
	return t.criteriaByStatus(Untestable)
}

// UncoveredCriteria returns all criteria still in Unchecked state.
func (t *CoverageTracker) UncoveredCriteria() []CriterionState {
	return t.criteriaByStatus(Unchecked)
}

// IsComplete returns true when all criteria are Covered or Untestable.
func (t *CoverageTracker) IsComplete() bool {
	for _, cs := range t.criteria {
		if cs.Status == Unchecked {
			return false
		}
	}
	return true
}

// NextUncovered returns the lowest-numbered unchecked criterion, or nil if none remain.
func (t *CoverageTracker) NextUncovered() *Criterion {
	minUncheckedIdx := -1
	for i := range t.criteria {
		if t.criteria[i].Status != Unchecked {
			continue
		}

		if minUncheckedIdx == -1 || t.criteria[i].Number < t.criteria[minUncheckedIdx].Number {
			minUncheckedIdx = i
		}
	}
	if minUncheckedIdx == -1 {
		return nil
	}

	c := t.criteria[minUncheckedIdx].Criterion
	return &c
}

func (t *CoverageTracker) findCriterionState(number int) *CriterionState {
	for i := range t.criteria {
		if t.criteria[i].Number == number {
			return &t.criteria[i]
		}
	}
	return nil
}

func (t *CoverageTracker) criteriaByStatus(status Status) []CriterionState {
	result := make([]CriterionState, 0)
	for _, cs := range t.criteria {
		if cs.Status == status {
			result = append(result, cs)
		}
	}
	return result
}

// TotalCriteria returns the total number of tracked criteria.
func (t *CoverageTracker) TotalCriteria() int {
	if t == nil {
		return 0
	}
	return len(t.criteria)
}

// CoveredCriteria returns all criteria in Covered state.
func (t *CoverageTracker) CoveredCriteria() []CriterionState {
	return t.criteriaByStatus(Covered)
}
