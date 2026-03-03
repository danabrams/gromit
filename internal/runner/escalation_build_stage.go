package runner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// escalationBuildStage wraps the escalation handler to implement pipeline.Stage
// for the Build phase. It delegates to the escalation handler's
// ExecuteWithRetryWithEscalation for full retry/escalation/decomposition behavior.
type escalationBuildStage struct {
	handler          *escalation.Handler
	execInvoker      *execution.Invoker
	renderer         execute.PromptRenderer
	fallback         pipeline.Stage
	promptRegistry   *buildPromptRegistry
	cacheVersionKey  string
	providerCostDefs map[string]config.ProviderDef
}

// Compile-time check: *escalationBuildStage must implement pipeline.Stage.
var _ pipeline.Stage = (*escalationBuildStage)(nil)

// Run implements pipeline.Stage by rendering the prompt, building a BeadContext,
// creating an InvokeFn wrapping execution.Invoker.Execute, and calling
// handler.ExecuteWithRetryWithEscalation.
func (s *escalationBuildStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	methodology := execute.SelectMethodology(in.Bead, in.Config)

	// Render prompt based on methodology
	promptText, err := s.renderPrompt(methodology, in)
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("build: rendering prompt: %w", err)
	}
	if in.Config == nil {
		return pipeline.Output{}, fmt.Errorf("build: %w", execute.ErrNilConfig)
	}

	// Determine tier
	beadTier := in.Config.BuildTierForStrategy(in.Bead.Priority, in.Bead.Labels, in.Complexity)
	phase := "build"
	if methodology == execute.MethodologyRefactor {
		phase = "refactor"
	}
	tier := in.Config.PhaseModelTier(phase, beadTier)
	originalTier := tier

	// Build BeadContext for escalation handler
	bc := s.buildBeadContext(in, tier, promptText)

	// Create InvokeFn wrapping execution.Invoker.Execute
	invokeFn := s.makeInvokeFn(promptText)

	// Execute with escalation handler
	success := s.handler.ExecuteWithRetryWithEscalation(ctx, bc, invokeFn, in.EscalationEnabled)

	// Map result back to pipeline.Output
	out := pipeline.Output{
		Decision:     pipeline.Proceed,
		OriginalTier: originalTier,
		ActualTier:   bc.Tier,
	}

	if bc.Result != nil {
		out.Model = bc.Result.Model
		out.DurationMs = bc.Result.Duration.Milliseconds()
		out.CostUSD = bc.Result.CostUSD
		out.InputTokens = bc.Result.InputTokens
		out.OutputTokens = bc.Result.OutputTokens
		out.CacheHit = bc.Result.CacheHit
		out.CacheMiss = bc.Result.CacheMiss
		out.CacheWrite = bc.Result.CacheWrite
		out.CacheClass = bc.Result.CacheClass
		out.CacheKey = bc.Result.CacheKey
		out.CacheInvalidationReason = bc.Result.CacheInvalidationReason
		out.CacheVersionMarker = bc.Result.CacheVersionMarker
	}

	if !success {
		failurePhase := "build"
		if bc.Result != nil && bc.Result.FailurePhase != "" {
			failurePhase = bc.Result.FailurePhase
		}
		return out, fmt.Errorf("build: escalation handler failed at phase %s", failurePhase)
	}

	return out, nil
}

// renderPrompt renders the appropriate prompt based on methodology.
func (s *escalationBuildStage) renderPrompt(methodology execute.Methodology, in pipeline.Input) (string, error) {
	switch methodology {
	case execute.MethodologyTDD:
		return s.renderer.RenderTDDBuild(in.Bead.Title, in.Bead.Description, in.ValidationFailures)
	case execute.MethodologyRefactor:
		return s.renderer.RenderRefactorBuild(in.Bead.Title, in.Bead.Description, in.ValidationFailures)
	default:
		return s.renderer.RenderBuild(in.Bead.Title, in.Bead.Description, in.ValidationFailures)
	}
}

// buildBeadContext creates a runtypes.BeadContext from pipeline.Input.
func (s *escalationBuildStage) buildBeadContext(in pipeline.Input, tier, promptText string) *runtypes.BeadContext {
	promptCtx := &prompt.Context{}
	if s.promptRegistry != nil {
		if cacheClass, cacheKey, ok := s.promptRegistry.lookup(promptText); ok {
			promptCtx.StaticPreambleCacheClass = cacheClass
			promptCtx.StaticPreambleCacheKey = cacheKey
		}
	}

	bc := &runtypes.BeadContext{
		Bead:        in.Bead,
		Tier:        tier,
		BuildPrompt: promptText,
		PromptCtx:   promptCtx,
		Result:      &runtypes.IterationResult{},
		Iteration:   in.Iteration,
		RunDeadline: in.Deadline,
	}

	// Apply retry limits from config
	if in.Config != nil {
		bc.MaxRetries = in.Config.Escalation.MaxRetriesPerModel
		bc.MaxRetriesPerBead = in.Config.Escalation.MaxRetriesPerBead
		bc.BeadTimeout = time.Duration(in.Config.Claude.BeadTimeout) * time.Second
	}

	return bc
}

// newEscalationBuildStage creates an escalationBuildStage wired with the given dependencies.
func newEscalationBuildStage(
	cfg *config.Config,
	analyzer escalation.FailureAnalyzer,
	beadClient escalation.BeadClient,
	decomposeFn escalation.DecomposeFn,
	createSubFn escalation.CreateSubFn,
	execInvoker *execution.Invoker,
	renderer *prompt.Renderer,
	fallback pipeline.Stage,
	registry *buildPromptRegistry,
	cacheVersionKey string,
	costDefs map[string]config.ProviderDef,
	output io.Writer,
) *escalationBuildStage {
	handler := escalation.NewHandler(
		cfg, analyzer, beadClient,
		decomposeFn, createSubFn,
		func(format string, args ...interface{}) { _, _ = fmt.Fprintf(output, format+"\n", args...) },
		nil,
	)
	return &escalationBuildStage{
		handler:          handler,
		execInvoker:      execInvoker,
		renderer:         &renderAdapter{r: renderer, promptRegistry: registry},
		fallback:         fallback,
		promptRegistry:   registry,
		cacheVersionKey:  cacheVersionKey,
		providerCostDefs: costDefs,
	}
}

// makeInvokeFn creates an escalation.InvokeFn wrapping execution.Invoker.Execute
// with prompt cache metadata plumbing.
func (s *escalationBuildStage) makeInvokeFn(promptText string) escalation.InvokeFn {
	return func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		inv := s.execInvoker
		if s.cacheVersionKey != "" {
			inv = inv.WithCacheVersionKey(s.cacheVersionKey)
		}
		result, err := inv.Execute(ctx, bc, prompt)
		if err != nil {
			return nil, err
		}
		if result == nil || result.ProviderResult == nil {
			return nil, fmt.Errorf("build invocation returned nil provider result")
		}
		applyCostFallback(result.ProviderResult, result.ProviderName, s.providerCostDefs)
		return result, nil
	}
}
