package prepare

import (
	"context"
	"fmt"
	"io"
	"strings"

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

// DataQualityBlocker determines whether data quality requirements are met for proceeding.
// When ShouldBlock returns true, the Gate returns Block to prevent further processing.
type DataQualityBlocker interface {
	ShouldBlock(ctx context.Context, b *bead.Bead) (bool, string, error) // blocked, reason, error
}

// Gate implements pipeline.Stage for Stage 1: gate decisions.
// It runs precheck, stuck-bead detection, and scope gate
// before any LLM build invocation.
// If precheck passes (work already done), returns Skip.
// If stuck (failure threshold exceeded), returns Block.
// If scope too large (expected outputs > maxScopeFiles), returns Block.
// Keyword-based proactive decomposition is intentionally disabled to avoid
// decomposition loops from title/description heuristics.
// If data quality blocked, returns Block.
// Otherwise returns Proceed.
type Gate struct {
	precheck          Prechecker        // optional; nil means skip precheck
	stuck             StuckDetector     // optional; nil means skip stuck detection
	decomposer        Decomposer        // optional; used only by scope-gate decomposition
	dataQualityChecker DataQualityBlocker // optional; nil means skip data quality checks
	output            io.Writer
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

// WithDecomposer configures an optional Decomposer for scope-gate decomposition of oversized beads.
func (g *Gate) WithDecomposer(d Decomposer) *Gate {
	g.decomposer = d
	return g
}

// WithDataQualityBlocker configures an optional DataQualityBlocker for data completeness checks.
func (g *Gate) WithDataQualityBlocker(dq DataQualityBlocker) *Gate {
	g.dataQualityChecker = dq
	return g
}

// HasDecomposer returns true if a Decomposer is wired in, false otherwise.
func (g *Gate) HasDecomposer() bool {
	return g.decomposer != nil
}

// Run executes the gate stage.
// It runs precheck (Skip if work already done), stuck detection (Block if threshold exceeded),
// and scope gate (Block/Skip based on oversized decomposition outcome),
// and returns Proceed otherwise.
func (g *Gate) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if in.Bead == nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	complexityRouting := resolveComplexityRouting(in)

	out := g.output
	if out == nil {
		out = io.Discard
	}

	// Precheck: skip beads whose acceptance criteria are already satisfied.
	// Runs before stuck detection so completed work is always closed promptly.
	if g.precheck != nil {
		done, err := g.precheck.Check(ctx, in.Bead)
		if err != nil {
			fmt.Fprintf(out, "Warning: precheck failed for bead %s: %v\n", in.Bead.ID, err)
		} else if done {
			return pipeline.Output{
				Decision:          pipeline.Skip,
				ComplexityRouting: complexityRouting,
			}, nil
		}
	}

	// Stuck detection: block beads that have exceeded the failure threshold.
	if g.stuck != nil {
		stuck, err := g.stuck.IsStuck(ctx, in.Bead)
		if err != nil {
			fmt.Fprintf(out, "Warning: stuck detection failed for bead %s: %v\n", in.Bead.ID, err)
		} else if stuck {
			return pipeline.Output{
				Decision:          pipeline.Block,
				ComplexityRouting: complexityRouting,
			}, nil
		}
	}

	// Data quality check: block beads if data quality requirements are not met.
	if g.dataQualityChecker != nil {
		blocked, reason, err := g.dataQualityChecker.ShouldBlock(ctx, in.Bead)
		if err != nil {
			fmt.Fprintf(out, "Warning: data quality check failed for bead %s: %v\n", in.Bead.ID, err)
		} else if blocked {
			fmt.Fprintf(out, "Data quality block for bead %s: %s\n", in.Bead.ID, reason)
			return pipeline.Output{
				Decision:          pipeline.Block,
				ComplexityRouting: complexityRouting,
			}, nil
		}
	}

	// Scope gate: try to handle oversized beads via decomposition, fallback to block.
	scopeGateDecision, scopeGateErr := g.runScopeGate(ctx, in, out)
	if scopeGateErr != nil {
		return pipeline.Output{}, scopeGateErr
	}
	if scopeGateDecision != nil {
		scopeGateDecision.ComplexityRouting = complexityRouting
		return *scopeGateDecision, nil
	}

	return pipeline.Output{
		Decision:          pipeline.Proceed,
		ComplexityRouting: complexityRouting,
	}, nil
}

func resolveComplexityRouting(in pipeline.Input) pipeline.ComplexityRouting {
	if normalized, ok := normalizeComplexity(in.Complexity); ok {
		return pipeline.ComplexityRouting{
			Complexity:               normalized,
			ComplexitySource:         "scope_estimate",
			ComplexityFallbackReason: "none",
		}
	}
	if normalized, ok := complexityFromLabels(in.Bead); ok {
		return pipeline.ComplexityRouting{
			Complexity:               normalized,
			ComplexitySource:         "label",
			ComplexityFallbackReason: "scope_unavailable",
		}
	}
	return pipeline.ComplexityRouting{
		Complexity:               "medium",
		ComplexitySource:         "default",
		ComplexityFallbackReason: "scope_and_label_unavailable",
	}
}

func normalizeComplexity(complexity string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(complexity))
	switch normalized {
	case "low", "medium", "high":
		return normalized, true
	}
	return "", false
}

func complexityFromLabels(b *bead.Bead) (string, bool) {
	if b == nil {
		return "", false
	}
	for _, label := range b.Labels {
		normalizedLabel := strings.ToLower(strings.TrimSpace(label))
		if !strings.HasPrefix(normalizedLabel, "complexity:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(normalizedLabel, "complexity:"))
		if normalized, ok := normalizeComplexity(value); ok {
			return normalized, true
		}
	}
	return "", false
}

// runScopeGate evaluates scope gate rules and attempts decomposition if needed.
// Returns a non-nil decision if scope gate blocks/skips; nil means scope gate allows progression.
// Returns an error if decomposition fails.
func (g *Gate) runScopeGate(ctx context.Context, in pipeline.Input, out io.Writer) (*pipeline.Output, error) {
	if in.Config == nil || !in.Config.ScopeCheck.Enabled {
		return nil, nil
	}

	blockOversized := in.Config.ScopeCheck.BlockOversized == nil || *in.Config.ScopeCheck.BlockOversized
	fileCount := bead.EstimatedFileCount(in.Bead)

	if !blockOversized || fileCount <= maxScopeFiles {
		return nil, nil
	}

	// Bead exceeds scope limit. Try decomposition if available.
	// Scope-based decomposition applies to child beads too: the finite
	// expected-outputs count bounds recursion depth naturally.
	if g.decomposer != nil {
		fmt.Fprintf(out, "Scope gate: bead %s has %d expected outputs (max %d), attempting decomposition\n",
			in.Bead.ID, fileCount, maxScopeFiles)
		if err := g.decomposer.Decompose(ctx, in.Bead); err != nil {
			fmt.Fprintf(out, "Warning: decomposition failed for bead %s: %v, falling back to block\n", in.Bead.ID, err)
			return &pipeline.Output{Decision: pipeline.Block}, nil
		}
		fmt.Fprintf(out, "Scope gate: decomposition succeeded for bead %s, skipping parent bead\n", in.Bead.ID)
		return &pipeline.Output{Decision: pipeline.Skip}, nil
	}

	// Decomposition not available or bead is child: block.
	fmt.Fprintf(out, "Scope gate: bead %s has %d expected outputs (max %d), blocking\n",
		in.Bead.ID, fileCount, maxScopeFiles)
	return &pipeline.Output{Decision: pipeline.Block}, nil
}
