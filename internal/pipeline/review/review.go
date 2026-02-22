package review

import (
	"context"
	"fmt"
	"io"
	"strings"

	reviewpkg "github.com/danabrams/gromit/internal/review"

	"github.com/danabrams/gromit/internal/pipeline"
)

// Invoker executes an LLM invocation via streaming and returns the collected output.
// Implementations must use StreamRun for live output visibility.
type Invoker interface {
	StreamRun(ctx context.Context, prompt string, model string, w io.Writer) (string, error)
}

// BeadCreator creates a bead and returns its ID.
type BeadCreator interface {
	Create(title string, priority int, labels []string, outputs []string) (string, error)
}

// PromptRenderer renders the review prompt from bead and diff context.
type PromptRenderer interface {
	RenderReview(beadTitle, diff string) (string, error)
}

// GitDiffFn returns the current git diff.
type GitDiffFn func() (string, error)

// Review implements pipeline.Stage for Stage 4: optional LLM code review.
// It is stateless across iterations; all state flows through Input and Output.
type Review struct {
	invoker  Invoker
	beads    BeadCreator
	renderer PromptRenderer
	gitDiff  GitDiffFn
	output   io.Writer
}

// Compile-time check: *Review must implement pipeline.Stage.
var _ pipeline.Stage = (*Review)(nil)

// New creates a Review stage with the given dependencies.
// output receives streamed LLM output for live visibility; pass io.Discard to suppress.
func New(invoker Invoker, beads BeadCreator, renderer PromptRenderer, gitDiff GitDiffFn, output io.Writer) *Review {
	return &Review{
		invoker:  invoker,
		beads:    beads,
		renderer: renderer,
		gitDiff:  gitDiff,
		output:   output,
	}
}

// Run executes the review stage.
// If review is disabled in config, it returns Proceed immediately with no LLM call.
// When enabled, it invokes LLM review via StreamRun, parses findings, creates
// [from-review]-labeled beads, and returns Output{Decision: Proceed, ReviewBeadIDs: [...]}.
func (r *Review) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if !in.Config.Review.Enabled {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	diff, err := r.gitDiff()
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("review: getting git diff: %w", err)
	}

	prompt, err := r.renderer.RenderReview(in.Bead.Title, diff)
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("review: rendering prompt: %w", err)
	}

	w := r.output
	if w == nil {
		w = io.Discard
	}

	var sb strings.Builder
	llmOutput, err := r.invoker.StreamRun(ctx, prompt, in.Config.Review.Model, io.MultiWriter(w, &sb))
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("review: LLM invocation: %w", err)
	}

	result, err := reviewpkg.ParseReviewResult(llmOutput)
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("review: parsing result: %w", err)
	}

	beadIDs := []string{}
	for _, bp := range result.BeadsToCreate {
		labels := pipeline.BuildFromReviewLabels(bp.Labels)
		id, err := r.beads.Create(bp.Title, bp.Priority, labels, bp.ExpectedOutputs)
		if err != nil {
			return pipeline.Output{}, fmt.Errorf("review: creating bead %q: %w", bp.Title, err)
		}
		beadIDs = append(beadIDs, id)
	}

	return pipeline.Output{
		Decision:      pipeline.Proceed,
		ReviewBeadIDs: beadIDs,
	}, nil
}
