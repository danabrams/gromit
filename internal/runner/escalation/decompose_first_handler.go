package escalation

import (
	"github.com/danabrams/gromit/internal/config"
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
