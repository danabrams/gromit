package prepare

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/pipeline"
)

// Prechecker determines whether a bead's acceptance criteria are already satisfied.
// When Check returns true, the Gate returns Skip to close the bead without build work.
type Prechecker interface {
	Check(ctx context.Context, b *bead.Bead) (bool, error)
}

// StuckDetector determines whether a bead has exceeded its failure threshold.
// When IsStuck returns true, the Gate returns Block to surface the bead for decomposition.
type StuckDetector interface {
	IsStuck(ctx context.Context, b *bead.Bead) (bool, error)
}

// Gate implements pipeline.Stage for Stage 1: gate decisions.
// It runs precheck and stuck-bead detection before any LLM build invocation.
// If precheck passes (work already done), returns Skip.
// If stuck (failure threshold exceeded), returns Block.
// Otherwise returns Proceed.
type Gate struct {
	precheck Prechecker    // optional; nil means skip precheck
	stuck    StuckDetector // optional; nil means skip stuck detection
	output   io.Writer
}

// Compile-time check: *Gate must implement pipeline.Stage.
var _ pipeline.Stage = (*Gate)(nil)

// New creates a Gate stage with the given output writer.
// output receives diagnostic messages; pass io.Discard to suppress.
func New(output io.Writer) *Gate {
	return &Gate{output: output}
}

// WithPrechecker configures an optional Prechecker for detecting already-completed work.
func (g *Gate) WithPrechecker(p Prechecker) *Gate {
	g.precheck = p
	return g
}

// WithStuckDetector configures an optional StuckDetector for detecting exceeded failure thresholds.
func (g *Gate) WithStuckDetector(s StuckDetector) *Gate {
	g.stuck = s
	return g
}

// Run executes the gate stage.
// It first runs precheck (Skip if work already done), then stuck detection (Block if threshold
// exceeded), and returns Proceed otherwise.
func (g *Gate) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if in.Bead == nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	w := g.output
	if w == nil {
		w = io.Discard
	}

	// Precheck: skip beads whose acceptance criteria are already satisfied.
	// Runs before stuck detection so completed work is always closed promptly.
	if g.precheck != nil {
		done, err := g.precheck.Check(ctx, in.Bead)
		if err != nil {
			fmt.Fprintf(w, "Warning: precheck failed for bead %s: %v\n", in.Bead.ID, err)
		} else if done {
			return pipeline.Output{Decision: pipeline.Skip}, nil
		}
	}

	// Stuck detection: block beads that have exceeded the failure threshold.
	if g.stuck != nil {
		stuck, err := g.stuck.IsStuck(ctx, in.Bead)
		if err != nil {
			fmt.Fprintf(w, "Warning: stuck detection failed for bead %s: %v\n", in.Bead.ID, err)
		} else if stuck {
			return pipeline.Output{Decision: pipeline.Block}, nil
		}
	}

	return pipeline.Output{Decision: pipeline.Proceed}, nil
}
