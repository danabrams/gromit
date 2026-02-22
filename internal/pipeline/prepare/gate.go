package prepare

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/pipeline"
)

// maxScopeFiles is the file-count cap: beads with more than this many expected outputs
// are blocked by the scope gate to prevent oversized single-iteration work.
// Based on the decomposition rule: split beads touching 6+ files.
const maxScopeFiles = 5

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

// Decomposer proactively decomposes an oversized bead into sub-beads.
// When Decompose succeeds, the original bead is closed and sub-beads are created;
// the Gate then returns Skip so the orchestrator moves to the next bead.
type Decomposer interface {
	Decompose(ctx context.Context, b *bead.Bead) error
}

// Gate implements pipeline.Stage for Stage 1: gate decisions.
// It runs precheck, stuck-bead detection, scope gate, and proactive decomposition
// before any LLM build invocation.
// If precheck passes (work already done), returns Skip.
// If stuck (failure threshold exceeded), returns Block.
// If scope too large (expected outputs > maxScopeFiles), returns Block.
// If proactive decomposition candidate (keyword title, no parent), decomposes and returns Skip.
// Otherwise returns Proceed.
type Gate struct {
	precheck   Prechecker    // optional; nil means skip precheck
	stuck      StuckDetector // optional; nil means skip stuck detection
	decomposer Decomposer    // optional; nil means skip proactive decomposition
	output     io.Writer
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

// WithDecomposer configures an optional Decomposer for proactive decomposition of oversized beads.
func (g *Gate) WithDecomposer(d Decomposer) *Gate {
	g.decomposer = d
	return g
}

// Run executes the gate stage.
// It runs precheck (Skip if work already done), stuck detection (Block if threshold exceeded),
// scope gate (Block if file count too large), proactive decomposition (Skip if keyword candidate),
// and returns Proceed otherwise.
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

	// Scope gate: block beads whose expected output count exceeds the file threshold.
	// Only runs when scope check is enabled and block_oversized is true (default).
	// If a decomposer is available and the bead is a root bead, attempt decomposition instead of blocking.
	if in.Config != nil && in.Config.ScopeCheck.Enabled {
		blockOversized := in.Config.ScopeCheck.BlockOversized == nil || *in.Config.ScopeCheck.BlockOversized
		if blockOversized && bead.EstimatedFileCount(in.Bead) > maxScopeFiles {
			beadSize := bead.EstimatedFileCount(in.Bead)

			// Attempt decomposition if available and bead is a root bead
			if g.decomposer != nil && in.Bead.Parent == "" {
				fmt.Fprintf(w, "Scope gate: bead %s has %d expected outputs (max %d), attempting decomposition\n",
					in.Bead.ID, beadSize, maxScopeFiles)
				if err := g.decomposer.Decompose(ctx, in.Bead); err != nil {
					fmt.Fprintf(w, "Warning: scope decomposition failed for bead %s: %v\n", in.Bead.ID, err)
					return pipeline.Output{Decision: pipeline.Block}, nil
				}
				return pipeline.Output{Decision: pipeline.Skip}, nil
			}

			fmt.Fprintf(w, "Scope gate: bead %s has %d expected outputs (max %d), blocking\n",
				in.Bead.ID, beadSize, maxScopeFiles)
			return pipeline.Output{Decision: pipeline.Block}, nil
		}
	}

	// Proactive decomposition: skip beads whose title contains broad-scope keywords.
	// Only applies to root beads (no parent) to prevent decompose loops.
	if g.decomposer != nil && in.Bead.Parent == "" {
		if bead.IsProactiveDecompositionCandidateWithDesc(in.Bead.Title, in.Bead.Description) {
			if err := g.decomposer.Decompose(ctx, in.Bead); err != nil {
				fmt.Fprintf(w, "Warning: proactive decomposition failed for bead %s: %v\n", in.Bead.ID, err)
			} else {
				return pipeline.Output{Decision: pipeline.Skip}, nil
			}
		}
	}

	return pipeline.Output{Decision: pipeline.Proceed}, nil
}
