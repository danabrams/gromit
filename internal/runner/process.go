package runner

import (
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// SetupBeadContext initializes bead-specific fields in the BeadContext.
// It extracts the spec label from the bead and sets it on the IterationResult.
func SetupBeadContext(bc *runtypes.BeadContext) {
	if bc == nil || bc.Bead == nil || bc.Result == nil {
		return
	}
	bc.Result.SpecID = bead.FindSpecLabel(bc.Bead.Labels)
}
