package escalation

import (
	"time"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

const (
	// proactiveDecomposeThreshold is the percentage of elapsed budget
	// at which proactive decomposition is triggered for high-risk beads.
	proactiveDecomposeThreshold = 0.60
)

// IsHighRiskBead determines if a bead is high-risk based on scope,
// retry history, and complexity factors.
// High-risk indicators:
// - Complexity is "high"
// - Estimated iterations >= 3
// - Cannot complete in single iteration and not marked as low complexity
// - Previous retries (TotalRetriesThisBead > 0)
func IsHighRiskBead(bc *runtypes.BeadContext) bool {
	if bc == nil || bc.ScopeEstimate == nil {
		return false
	}

	se := bc.ScopeEstimate

	// High complexity is an immediate high-risk indicator
	if se.Complexity == "high" {
		return true
	}

	// Many estimated iterations indicate high risk
	if se.EstimatedIterations >= 3 {
		return true
	}

	// Previous retries indicate the bead is difficult
	if bc.TotalRetriesThisBead > 0 {
		return true
	}

	// Not single-iteration (without low complexity) indicates more work
	if !se.CanCompleteInSingleIteration && se.Complexity != "low" {
		return true
	}

	return false
}

// ShouldProactivelyDecompose checks if a high-risk bead has reached
// the proactive decomposition threshold (60% elapsed budget).
// Returns true if the bead is high-risk AND has used >= 60% of its timeout budget.
func ShouldProactivelyDecompose(bc *runtypes.BeadContext) bool {
	if bc == nil {
		return false
	}

	// Only trigger for high-risk beads
	if !IsHighRiskBead(bc) {
		return false
	}

	// Check elapsed time against budget
	if bc.BeadTimeout <= 0 || bc.BeadStartTime.IsZero() {
		return false
	}

	elapsed := time.Since(bc.BeadStartTime)
	if elapsed < 0 {
		return false
	}

	elapsedRatio := float64(elapsed) / float64(bc.BeadTimeout)
	return elapsedRatio >= proactiveDecomposeThreshold
}
