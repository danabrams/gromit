package coverage

// Status represents the coverage state of a single criterion.
type Status int

const (
	Unchecked  Status = iota
	Covered    Status = iota
	Untestable Status = iota
)

// CriterionState tracks a criterion and its coverage status.
type CriterionState struct {
	Criterion
	Status         Status
	RejectionCount int
}

// CoverageTracker manages a checklist of criteria states.
type CoverageTracker struct {
	criteria      []CriterionState
	maxRejections int
}

// NewTracker creates a CoverageTracker from a list of criteria.
func NewTracker(criteria []Criterion, maxRejections int) *CoverageTracker {
	states := make([]CriterionState, len(criteria))
	for i, c := range criteria {
		states[i] = CriterionState{Criterion: c, Status: Unchecked}
	}
	return &CoverageTracker{criteria: states, maxRejections: maxRejections}
}

// NextUncovered returns the lowest-numbered unchecked criterion, or nil if none remain.
func (t *CoverageTracker) NextUncovered() *Criterion {
	for i := range t.criteria {
		if t.criteria[i].Status == Unchecked {
			c := t.criteria[i].Criterion
			return &c
		}
	}
	return nil
}
