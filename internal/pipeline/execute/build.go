package execute

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
)

// Methodology represents the build methodology selected for a bead.
type Methodology string

const (
	// MethodologyTDD uses a TDD-specific build prompt (red-green-refactor).
	MethodologyTDD Methodology = "tdd"
	// MethodologyRefactor uses a refactor-specific build prompt.
	MethodologyRefactor Methodology = "refactor"
	// MethodologyStandard uses the default build prompt.
	MethodologyStandard Methodology = "standard"
)

// TDDCycleResult holds the aggregated output from a TDDCycleRunner.
type TDDCycleResult struct {
	PhaseMetrics []pipeline.PhaseMetrics
}

// TDDCycleRunner runs multiple TDD cycles (red-green-refactor) for a bead,
// making a fresh LLM invocation for each phase.
type TDDCycleRunner interface {
	RunCycles(ctx context.Context, b *bead.Bead, cfg *config.Config) (TDDCycleResult, error)
}

// Invoker executes LLM invocations.
// Implementations must use StreamRun for live output visibility.
// Run is part of the interface to allow fakes to panic on it and prove
// the Build stage never calls it directly.
type Invoker interface {
	Run(ctx context.Context, prompt, tier string) (*provider.Result, error)
	StreamRun(ctx context.Context, prompt, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
}

// PromptRenderer renders build prompts for each methodology.
type PromptRenderer interface {
	RenderBuild(title, description string, validationFailures []string) (string, error)
	RenderTDDBuild(title, description string, validationFailures []string) (string, error)
	RenderRefactorBuild(title, description string, validationFailures []string) (string, error)
}

// Build implements pipeline.Stage for Stage 2: LLM code authoring.
// It selects the methodology (TDD, refactor, standard), renders the appropriate
// prompt, and invokes the provider via StreamRun only.
type Build struct {
	invoker  Invoker
	renderer PromptRenderer
	output   io.Writer
}

// Compile-time check: *Build must implement pipeline.Stage.
var _ pipeline.Stage = (*Build)(nil)

// New creates a Build stage with the given dependencies.
// output receives streamed LLM output for live visibility; pass io.Discard to suppress.
func New(invoker Invoker, renderer PromptRenderer, output io.Writer) *Build {
	return &Build{
		invoker:  invoker,
		renderer: renderer,
		output:   output,
	}
}

// SelectMethodology determines the methodology for a bead based on its labels and config.
// Priority order: label override (tdd:true/false, refactor:true) > global config default.
func SelectMethodology(b *bead.Bead, cfg *config.Config) Methodology {
	tddGlobal := cfg != nil && cfg.Methodology.TDD
	if bead.IsMethodologyActive(b.Labels, "tdd", tddGlobal) {
		return MethodologyTDD
	}
	if bead.IsMethodologyActive(b.Labels, "refactor", false) {
		return MethodologyRefactor
	}
	return MethodologyStandard
}

// ShouldRunPostSuccess reports whether post-success stages (review, learning, epilogue)
// should run immediately after this build succeeds.
// Returns false for TDD methodology because the refactor phase must complete first.
func (b *Build) ShouldRunPostSuccess(bead *bead.Bead, cfg *config.Config) bool {
	return SelectMethodology(bead, cfg) != MethodologyTDD
}

// Run executes the build stage: selects methodology, renders the prompt, and
// invokes the provider via StreamRun. Returns Proceed on both success and failure
// so the orchestrator can inspect Output and decide whether to proceed to Validate.
func (b *Build) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	methodology := SelectMethodology(in.Bead, in.Config)

	var (
		prompt string
		err    error
	)
	switch methodology {
	case MethodologyTDD:
		prompt, err = b.renderer.RenderTDDBuild(in.Bead.Title, in.Bead.Description, in.ValidationFailures)
	case MethodologyRefactor:
		prompt, err = b.renderer.RenderRefactorBuild(in.Bead.Title, in.Bead.Description, in.ValidationFailures)
	default:
		prompt, err = b.renderer.RenderBuild(in.Bead.Title, in.Bead.Description, in.ValidationFailures)
	}
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("build: rendering prompt: %w", err)
	}

	tier := in.Config.SelectTier(in.Bead.Priority, in.Bead.Labels)

	w := b.output
	if w == nil {
		w = io.Discard
	}

	_, err = b.invoker.StreamRun(ctx, prompt, tier, w, nil, nil)
	if err != nil && in.EscalationEnabled {
		for {
			nextTier := in.Config.NextEscalationTier(tier)
			if nextTier == "" {
				break
			}
			tier = nextTier
			_, err = b.invoker.StreamRun(ctx, prompt, tier, w, nil, nil)
			if err == nil {
				break
			}
		}
	}
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("build: LLM invocation: %w", err)
	}

	return pipeline.Output{Decision: pipeline.Proceed}, nil
}
