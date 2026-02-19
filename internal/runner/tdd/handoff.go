package tdd

// RedHandoff contains context for generating failing tests in the red phase.
type RedHandoff struct {
	SpecExcerpt  string
	TestFiles    map[string]string
	ImplFiles    map[string]string
	APISurface   string
	CycleSummary string
}

// CycleState tracks TDD cycle progress across iterations.
type CycleState struct {
	CycleNumber int
	MaxCycles   int
	CoveredSoFar []string
	Remaining    []string
	TouchedFiles []string
	Done         bool
}

// IsComplete reports whether the cycle is finished.
func (c CycleState) IsComplete() bool {
	return c.Done || c.CycleNumber >= c.MaxCycles
}
