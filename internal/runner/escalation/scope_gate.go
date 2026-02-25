package escalation

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

const (
	// preExecutionScopeThreshold is the maximum number of files that can be
	// handled in a single invocation. If estimated file count exceeds this,
	// the task should be decomposed before attempting the first invocation.
	preExecutionScopeThreshold = 2
)

// ShouldTriggerPreExecutionScopeDecomposition determines if a bead's estimated
// file count exceeds the pre-execution scope threshold (>2 files).
// Returns true if decomposition should be triggered before first invocation.
func ShouldTriggerPreExecutionScopeDecomposition(bc *runtypes.BeadContext) bool {
	if bc == nil || bc.Bead == nil {
		return false
	}

	fileCount := bead.EstimatedFileCount(bc.Bead)
	return fileCount > preExecutionScopeThreshold
}

// CheckPreExecutionScopeGate checks if the bead should be decomposed before
// the first invocation attempt based on estimated file count.
// Returns true if the loop should continue (no decomposition needed).
// Returns false if decomposition was triggered or failed.
func (h *Handler) CheckPreExecutionScopeGate(ctx context.Context, bc *runtypes.BeadContext) bool {
	if !ShouldTriggerPreExecutionScopeDecomposition(bc) {
		return true // Continue with invocation
	}

	h.log("Pre-execution scope gate triggered: estimated file count > %d", preExecutionScopeThreshold)
	continueLoop := h.AttemptDecomposition(ctx, bc, "pre-execution scope gate: estimated file count exceeds threshold")
	return continueLoop
}
