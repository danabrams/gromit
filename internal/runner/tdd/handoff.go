package tdd

// RedHandoff contains context for generating failing tests in the red phase.
type RedHandoff struct {
	SpecExcerpt  string
	TestFiles    map[string]string
	ImplFiles    map[string]string
	APISurface   string
	CycleSummary string
}

// GreenHandoff contains context for implementing code to satisfy failing tests.
type GreenHandoff struct {
	FailingTest       string
	TestFailureOutput string
	ImplFiles         map[string]string
}

// RefactorHandoff contains context for behavior-preserving cleanup in the refactor phase.
type RefactorHandoff struct {
	ImplFiles map[string]string
	TestFiles map[string]string
}

// CycleState tracks TDD cycle progress across iterations.
type CycleState struct {
	CycleNumber  int
	MaxCycles    int
	CoveredSoFar []string
	Remaining    []string
	TouchedFiles []string
	Done         bool
}

// IsComplete reports whether the cycle is finished.
func (c CycleState) IsComplete() bool {
	if c.Done {
		return true
	}
	if c.CycleNumber >= c.MaxCycles {
		return true
	}
	// Only treat empty remaining as complete if at least one cycle ran
	if c.CycleNumber > 0 && len(c.Remaining) == 0 {
		return true
	}
	return false
}
