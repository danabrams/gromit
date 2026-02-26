package escalation

import (
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
