package tdd

// CycleSnapshot captures the state of a TDD cycle for convergence evaluation.
type CycleSnapshot struct {
	CycleNumber  int
	CoveredSoFar []string
	Remaining    []string
	TouchedFiles []string
}

// ConvergenceResult classifies the outcome of evaluating recent cycles.
type ConvergenceResult string

const (
	ConvergenceStable      ConvergenceResult = "stable"
	ConvergenceDeadlock    ConvergenceResult = "deadlock"
	ConvergenceOscillation ConvergenceResult = "oscillation"
)

// EvaluateCycleConvergence inspects recent snapshots and reports the latest convergence state.
func EvaluateCycleConvergence(snapshots []CycleSnapshot) ConvergenceResult {
	n := len(snapshots)
	if n == 0 {
		return ConvergenceStable
	}

	latest := snapshots[n-1]
	if len(latest.Remaining) == 0 {
		return ConvergenceStable
	}

	if n >= 2 {
		prev := snapshots[n-2]
		if len(prev.Remaining) > 0 && stringSlicesEqual(latest.Remaining, prev.Remaining) {
			return ConvergenceDeadlock
		}
	}

	if n >= 3 {
		third := snapshots[n-3]
		if len(third.Remaining) > 0 && stringSlicesEqual(latest.Remaining, third.Remaining) {
			return ConvergenceOscillation
		}
	}

	return ConvergenceStable
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
