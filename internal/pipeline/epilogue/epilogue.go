package epilogue

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/pipeline"
)

// BeadLifecycle handles bead close and sync operations after a successful iteration.
type BeadLifecycle interface {
	Close(id string) error
	Sync() error
}

// StatusWriter writes execution status after each iteration.
type StatusWriter interface {
	Write(iteration int, beadID, beadTitle string) error
}

// Epilogue implements pipeline.Stage for Stage 5: bead lifecycle and cleanup.
// It closes and syncs the bead on the success path, and writes status after every iteration.
type Epilogue struct {
	beads  BeadLifecycle
	status StatusWriter
	output io.Writer
}

// Compile-time check: *Epilogue must implement pipeline.Stage.
var _ pipeline.Stage = (*Epilogue)(nil)

// New creates an Epilogue stage with the given dependencies.
// output receives warning messages; pass io.Discard to suppress.
func New(beads BeadLifecycle, status StatusWriter, output io.Writer) *Epilogue {
	return &Epilogue{
		beads:  beads,
		status: status,
		output: output,
	}
}

// Run executes the epilogue stage.
// On the success path (in.BuildSucceeded == true), it closes the bead and syncs.
// After every iteration, it writes status.
func (e *Epilogue) Run(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
	w := e.output
	if w == nil {
		w = io.Discard
	}

	if in.BuildSucceeded {
		if err := e.beads.Close(in.Bead.ID); err != nil {
			fmt.Fprintf(w, "Warning: failed to close bead: %v\n", err)
		}
		if err := e.beads.Sync(); err != nil {
			fmt.Fprintf(w, "Warning: failed to sync beads: %v\n", err)
		}
	}

	if e.status != nil {
		if err := e.status.Write(in.Iteration, in.Bead.ID, in.Bead.Title); err != nil {
			fmt.Fprintf(w, "Warning: failed to write status: %v\n", err)
		}
	}

	return pipeline.Output{Decision: pipeline.Proceed}, nil
}
