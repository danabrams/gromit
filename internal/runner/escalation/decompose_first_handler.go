package escalation

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// DecomposeFirstHandler retries at a low tier before decomposing non-atomic failures.
// For atomic failures, it escalates through the existing tier chain.
type DecomposeFirstHandler struct {
	cfg                   *config.Config
	analyzer              FailureAnalyzer
	beadClient            BeadClient
	decomposeFn           DecomposeFn
	createSubFn           CreateSubFn
	logFn                 LogFn
	showPartialProgressFn ShowPartialProgressFn
	maxRetriesBeforeDecompose int
}

// NewDecomposeFirstHandler creates a DecomposeFirstHandler with narrow dependency interfaces.
func NewDecomposeFirstHandler(
	cfg *config.Config,
	analyzer FailureAnalyzer,
	beadClient BeadClient,
	decomposeFn DecomposeFn,
	createSubFn CreateSubFn,
	logFn LogFn,
	showPartialProgressFn ShowPartialProgressFn,
	maxRetriesBeforeDecompose int,
) *DecomposeFirstHandler {
	return &DecomposeFirstHandler{
		cfg:                   cfg,
		analyzer:              analyzer,
		beadClient:            beadClient,
		decomposeFn:           decomposeFn,
		createSubFn:           createSubFn,
		logFn:                 logFn,
		showPartialProgressFn: showPartialProgressFn,
		maxRetriesBeforeDecompose: maxRetriesBeforeDecompose,
	}
}

// ShouldDecomposeBeforeEscalate determines if a bead should be decomposed before escalating.
// Returns true if retries have been exhausted for the current tier.
func (h *DecomposeFirstHandler) ShouldDecomposeBeforeEscalate(bc *runtypes.BeadContext) bool {
	if bc == nil {
		return false
	}
	return bc.RetriesThisModel >= h.maxRetriesBeforeDecompose
}

// IsAtomicBead determines if a bead is atomic and should not be decomposed.
// A bead is atomic if decomposition is not available.
func (h *DecomposeFirstHandler) IsAtomicBead(bc *runtypes.BeadContext) bool {
	if bc == nil {
		return true
	}
	// A bead is atomic if decomposition is not available
	return h.decomposeFn == nil
}

// log calls the logging callback if set.
func (h *DecomposeFirstHandler) log(format string, args ...interface{}) {
	if h.logFn != nil {
		h.logFn(format, args...)
	}
}

// AttemptDecomposition tries to decompose the task into sub-beads.
// On success, sets result.Decomposed=true. On failure, sets result.Error.
// Always returns false (processBead should return after this).
func (h *DecomposeFirstHandler) AttemptDecomposition(ctx context.Context, bc *runtypes.BeadContext, failureReason string) (continueLoop bool) {
	if h.decomposeFn == nil {
		if bc != nil && bc.Result != nil {
			bc.Result.Error = fmt.Errorf("%s and decomposition not available", failureReason)
		}
		return false
	}

	h.log("Attempting to decompose task after: %s", failureReason)
	subTasks, err := h.decomposeFn(ctx, bc.Bead)
	if err != nil {
		h.log("Decomposition failed: %v", err)
		if bc != nil && bc.Result != nil {
			bc.Result.Error = fmt.Errorf("%s and decomposition failed: %w", failureReason, err)
		}
		return false
	}

	if h.createSubFn == nil {
		if bc != nil && bc.Result != nil {
			bc.Result.Error = fmt.Errorf("%s decomposition succeeded but sub-bead creation not available", failureReason)
		}
		return false
	}

	if err := h.createSubFn(ctx, bc.Bead, subTasks); err != nil {
		h.log("Failed to create sub-beads: %v", err)
		if bc != nil && bc.Result != nil {
			bc.Result.Error = fmt.Errorf("%s decomposition succeeded but failed to create sub-beads: %w", failureReason, err)
		}
		return false
	}

	h.log("Task successfully decomposed into %d sub-tasks", len(subTasks))
	if bc != nil && bc.Result != nil {
		bc.Result.Decomposed = true
		bc.Result.Error = nil
	}
	return false
}
