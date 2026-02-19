package tdd

// CycleState tracks TDD cycle progress across iterations.
type CycleState struct {
	CycleNumber int
	MaxCycles   int
	CoveredSoFar []string
	Remaining   []string
	TouchedFiles []string
	Done        bool
}

// IsComplete reports whether the cycle is finished.
func (c CycleState) IsComplete() bool {
	return c.Done || c.CycleNumber >= c.MaxCycles
}
