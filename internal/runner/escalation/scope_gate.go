package escalation

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

const (
	// preExecutionScopeThreshold is the maximum number of files that can be
	// handled in a single invocation. If estimated file count exceeds this,
	// the task should be decomposed before attempting the first invocation.
	preExecutionScopeThreshold = 3
)

// PostExecutionScopeResult holds the outcome of a post-execution scope check.
type PostExecutionScopeResult struct {
	FilesChanged   int
	FilesEstimated int
	ScopeExceeded  bool
	Message        string
}

// postExecutionScopeMultiplier is the factor by which actual files changed
// can exceed the estimate before triggering a scope-creep warning.
const postExecutionScopeMultiplier = 2

// postExecutionScopeHardCap is the maximum files changed allowed when
// no estimate is available on the bead.
const postExecutionScopeHardCap = 6

// ShouldTriggerPreExecutionScopeDecomposition determines if a bead's estimated
// file count exceeds the pre-execution scope threshold (>3 files).
// Returns true if decomposition should be triggered before first invocation.
func ShouldTriggerPreExecutionScopeDecomposition(bc *runtypes.BeadContext) bool {
	if bc == nil || bc.Bead == nil {
		return false
	}

	fileCount := bead.EstimatedFileCount(bc.Bead)
	return fileCount > preExecutionScopeThreshold
}

// CheckPostExecutionScope compares the actual number of files changed against
// the bead's estimated file count. Returns a result indicating whether scope
// was exceeded.
func CheckPostExecutionScope(bc *runtypes.BeadContext, filesChanged int) PostExecutionScopeResult {
	if bc == nil || bc.Bead == nil {
		return PostExecutionScopeResult{}
	}

	estimated := bead.EstimatedFileCount(bc.Bead)
	result := PostExecutionScopeResult{
		FilesChanged:   filesChanged,
		FilesEstimated: estimated,
	}

	if estimated > 0 {
		threshold := estimated * postExecutionScopeMultiplier
		if filesChanged > threshold {
			result.ScopeExceeded = true
			result.Message = fmt.Sprintf(
				"post-execution scope exceeded: %d files changed vs %d estimated (threshold %dx = %d)",
				filesChanged, estimated, postExecutionScopeMultiplier, threshold,
			)
		}
	} else {
		if filesChanged > postExecutionScopeHardCap {
			result.ScopeExceeded = true
			result.Message = fmt.Sprintf(
				"post-execution scope exceeded: %d files changed with no estimate (hard cap %d)",
				filesChanged, postExecutionScopeHardCap,
			)
		}
	}

	return result
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
