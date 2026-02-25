package execute

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
)

// ErrNilConfig indicates the Build stage was invoked without configuration.
var ErrNilConfig = errors.New("nil config")

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
	PhaseMetrics []pipeline.PhaseMetric
	OriginalTier string
	ActualTier   string
	Model        string
	DurationMs   int64
	CostUSD      float64
	InputTokens  int
	OutputTokens int
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
	invoker        Invoker
	renderer       PromptRenderer
	output         io.Writer
	tddCycleRunner TDDCycleRunner
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

// WithTDDCycleRunner injects a TDDCycleRunner for fresh-context TDD execution.
// Returns the receiver for fluent chaining.
func (b *Build) WithTDDCycleRunner(runner TDDCycleRunner) *Build {
	b.tddCycleRunner = runner
	return b
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
	strategy := resolveBuildStrategy(in.Config, in.Bead)
	methodology := SelectMethodology(in.Bead, in.Config)

	if strategy == "tdd" && in.Config != nil {
		methodology = MethodologyTDD
	} else if strategy == "single_pass" &&
		hasBuildStrategyLabel(in.Bead, "build_strategy:single_pass") &&
		(methodology == MethodologyTDD || methodology == MethodologyRefactor) {
		methodology = MethodologyStandard
	}

	if methodology == MethodologyTDD && in.Config != nil && in.Config.Methodology.FreshContextPerCycle && b.tddCycleRunner != nil {
		result, err := b.tddCycleRunner.RunCycles(ctx, in.Bead, in.Config)
		out := pipeline.Output{
			Decision:     pipeline.Proceed,
			PhaseMetrics: result.PhaseMetrics,
			OriginalTier: result.OriginalTier,
			ActualTier:   result.ActualTier,
			Model:        result.Model,
			DurationMs:   result.DurationMs,
			CostUSD:      result.CostUSD,
			InputTokens:  result.InputTokens,
			OutputTokens: result.OutputTokens,
		}
		if err != nil {
			return out, fmt.Errorf("build: TDD cycle runner: %w", err)
		}
		return out, nil
	}

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
	if in.Config == nil {
		return pipeline.Output{}, fmt.Errorf("build: %w", ErrNilConfig)
	}

	beadTier := in.Config.SelectInitialTierForComplexity(in.Complexity)
	phase := "build"
	if methodology == MethodologyRefactor {
		phase = "refactor"
	}
	tier := in.Config.PhaseModelTier(phase, beadTier)
	originalTier := tier

	w := b.output
	if w == nil {
		w = io.Discard
	}

	result, err := b.invoker.StreamRun(ctx, prompt, tier, w, nil, nil)
	invocationErr := streamRunResultError(result, err)
	if invocationErr != nil && in.EscalationEnabled {
		for {
			nextTier := in.Config.NextEscalationTier(tier)
			if nextTier == "" {
				break
			}
			tier = nextTier
			result, err = b.invoker.StreamRun(ctx, prompt, tier, w, nil, nil)
			invocationErr = streamRunResultError(result, err)
			if invocationErr == nil {
				break
			}
		}
	}
	if invocationErr != nil {
		return pipeline.Output{}, fmt.Errorf("build: LLM invocation: %w", invocationErr)
	}

	out := pipeline.Output{
		Decision:     pipeline.Proceed,
		OriginalTier: originalTier,
		ActualTier:   tier,
	}
	if result != nil {
		out.Model = result.Model
		out.DurationMs = DurationMsFromDuration(result.Duration)
		out.CostUSD = result.CostUSD
		out.InputTokens = result.InputTokens
		out.OutputTokens = result.OutputTokens
		out.CacheHit = result.CacheHit || result.CachedInputTokens > 0
		out.CacheMiss = result.CacheMiss
		out.CacheWrite = result.CacheWrite
		out.CacheClass = result.CacheClass
		out.CacheKey = result.CacheKey
		out.CacheInvalidationReason = result.CacheInvalidationReason
		out.CacheVersionMarker = result.CacheVersionMarker
	}
	return out, nil
}

// DurationMsFromDuration converts a time.Duration to milliseconds while ensuring that
// any positive duration rounds up to at least 1ms so haiku iterations are not
// recorded as 0ms.
func DurationMsFromDuration(duration time.Duration) int64 {
	ms := duration.Milliseconds()
	if duration > 0 && ms == 0 {
		return 1
	}
	return ms
}

func resolveBuildStrategy(cfg *config.Config, b *bead.Bead) string {
	strategy := "single_pass"
	if cfg != nil && cfg.Methodology.BuildStrategy != "" {
		strategy = cfg.Methodology.BuildStrategy
	}
	if b == nil {
		return strategy
	}
	for _, label := range b.Labels {
		switch label {
		case "build_strategy:tdd":
			strategy = "tdd"
		case "build_strategy:single_pass":
			strategy = "single_pass"
		}
	}
	return strategy
}

func hasBuildStrategyLabel(b *bead.Bead, want string) bool {
	if b == nil {
		return false
	}
	for _, label := range b.Labels {
		if label == want {
			return true
		}
	}
	return false
}

func streamRunResultError(result *provider.Result, err error) error {
	if err != nil {
		return err
	}
	if result == nil {
		return errors.New("provider returned nil result")
	}
	if !result.Success {
		return fmt.Errorf("provider reported unsuccessful result (exit code: %d)", result.ExitCode)
	}
	return nil
}
