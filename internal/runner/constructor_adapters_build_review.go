package runner

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/pipeline/review"
	pipelinevalidate "github.com/danabrams/gromit/internal/pipeline/validate"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/internal/specflow"
	"github.com/danabrams/gromit/internal/tracker"
)

// Adapter types for bridging existing infrastructure to pipeline stage interfaces.

// invokerAdapter wraps *provider.Router to satisfy execute.Invoker.
type invokerAdapter struct {
	execInvoker      *execution.Invoker
	promptRegistry   *buildPromptRegistry
	cacheVersionKey  string
	providerCostDefs map[string]config.ProviderDef
}

func (a *invokerAdapter) Run(ctx context.Context, promptText, tier string) (*provider.Result, error) {
	return a.StreamRun(ctx, promptText, tier, io.Discard, nil, nil)
}

func (a *invokerAdapter) StreamRun(ctx context.Context, promptText, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if a == nil || a.execInvoker == nil {
		return nil, fmt.Errorf("build invoker is not configured")
	}

	promptCtx := &prompt.Context{}
	if a.promptRegistry != nil {
		if cacheClass, cacheKey, ok := a.promptRegistry.lookup(promptText); ok {
			promptCtx.StaticPreambleCacheClass = cacheClass
			promptCtx.StaticPreambleCacheKey = cacheKey
		}
	}

	bc := &runtypes.BeadContext{
		Tier:        tier,
		BuildPrompt: promptText,
		PromptCtx:   promptCtx,
		Result:      &runtypes.IterationResult{},
	}
	inv := a.execInvoker
	if a.cacheVersionKey != "" {
		inv = inv.WithCacheVersionKey(a.cacheVersionKey)
	}
	result, err := inv.Execute(ctx, bc, promptText)
	if err != nil {
		return nil, err
	}
	if result == nil || result.ProviderResult == nil {
		return nil, fmt.Errorf("build invocation returned nil provider result")
	}
	providerResult := result.ProviderResult
	applyCostFallback(providerResult, result.ProviderName, a.providerCostDefs)
	providerResult.CacheHit = bc.Result.CacheHit
	providerResult.CacheMiss = bc.Result.CacheMiss
	providerResult.CacheWrite = bc.Result.CacheWrite
	providerResult.CacheClass = bc.Result.CacheClass
	providerResult.CacheKey = bc.Result.CacheKey
	providerResult.CacheInvalidationReason = bc.Result.CacheInvalidationReason
	providerResult.CacheVersionMarker = bc.Result.CacheVersionMarker
	return providerResult, nil
}

func applyCostFallback(result *provider.Result, providerName string, defs map[string]config.ProviderDef) {
	if result == nil || result.CostUSD > 0 {
		return
	}
	if result.InputTokens <= 0 && result.OutputTokens <= 0 {
		return
	}
	if len(defs) == 0 {
		return
	}
	def, ok := defs[providerName]
	if !ok {
		return
	}
	estimate := def.EstimateCostForModel(result.Model, result.InputTokens, result.OutputTokens)
	if estimate > 0 {
		result.CostUSD = estimate
	}
}

// renderAdapter wraps prompt.Renderer to satisfy execute.PromptRenderer.
type renderAdapter struct {
	r                 *prompt.Renderer
	promptRegistry    *buildPromptRegistry
	midReviewFindings []string
}

func (a *renderAdapter) SetMidBuildReviewFindings(findings []string) {
	if a == nil {
		return
	}
	if len(findings) == 0 {
		a.midReviewFindings = nil
		return
	}
	a.midReviewFindings = append([]string(nil), findings...)
}

func (a *renderAdapter) buildPromptContext(title, description string, validationFailures []string) *prompt.Context {
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: append([]string(nil), validationFailures...),
	}
	if len(a.midReviewFindings) > 0 {
		ctx.MidBuildReviewFindings = append([]string(nil), a.midReviewFindings...)
	}
	a.midReviewFindings = nil
	return ctx
}

func (a *renderAdapter) RenderBuild(title, description string, validationFailures []string) (string, error) {
	ctx := a.buildPromptContext(title, description, validationFailures)
	rendered, err := a.r.RenderBuild(ctx)
	if err != nil {
		return "", err
	}
	if a.promptRegistry != nil {
		a.promptRegistry.remember(rendered, ctx.StaticPreambleCacheClass, ctx.StaticPreambleCacheKey)
	}
	return rendered, nil
}

func (a *renderAdapter) RenderTDDBuild(title, description string, validationFailures []string) (string, error) {
	ctx := a.buildPromptContext(title, description, validationFailures)
	rendered, err := a.r.RenderTDDBuild(ctx)
	if err != nil {
		return "", err
	}
	if a.promptRegistry != nil {
		a.promptRegistry.remember(rendered, ctx.StaticPreambleCacheClass, ctx.StaticPreambleCacheKey)
	}
	return rendered, nil
}

func (a *renderAdapter) RenderRefactorBuild(title, description string, validationFailures []string) (string, error) {
	ctx := a.buildPromptContext(title, description, validationFailures)
	rendered, err := a.r.RenderRefactor(ctx)
	if err != nil {
		return "", err
	}
	if a.promptRegistry != nil {
		a.promptRegistry.remember(rendered, ctx.StaticPreambleCacheClass, ctx.StaticPreambleCacheKey)
	}
	return rendered, nil
}

type buildPromptRegistry struct {
	mu      sync.Mutex
	entries map[string]buildPromptMetadata
}

type buildPromptMetadata struct {
	cacheClass string
	cacheKey   string
}

func newBuildPromptRegistry() *buildPromptRegistry {
	return &buildPromptRegistry{
		entries: make(map[string]buildPromptMetadata),
	}
}

func (r *buildPromptRegistry) remember(promptText, cacheClass, cacheKey string) {
	if r == nil || promptText == "" {
		return
	}
	key := promptDigest(promptText)
	r.mu.Lock()
	r.entries[key] = buildPromptMetadata{
		cacheClass: strings.TrimSpace(cacheClass),
		cacheKey:   strings.TrimSpace(cacheKey),
	}
	r.mu.Unlock()
}

func (r *buildPromptRegistry) lookup(promptText string) (cacheClass, cacheKey string, ok bool) {
	if r == nil || promptText == "" {
		return "", "", false
	}
	key := promptDigest(promptText)
	r.mu.Lock()
	meta, exists := r.entries[key]
	r.mu.Unlock()
	if !exists || meta.cacheClass == "" || meta.cacheKey == "" {
		return "", "", false
	}
	return meta.cacheClass, meta.cacheKey, true
}

func promptDigest(promptText string) string {
	sum := sha1.Sum([]byte(promptText))
	return hex.EncodeToString(sum[:])
}

type executionRouterAdapter struct {
	router *provider.Router
}

func (a *executionRouterAdapter) Select(phase, tier string) (execution.Provider, string) {
	if a == nil || a.router == nil {
		return nil, ""
	}
	p, model := a.router.Select(phase, tier)
	return p, model
}

func (a *executionRouterAdapter) MarkUnavailable(name string) {
	if a == nil || a.router == nil {
		return
	}
	a.router.MarkUnavailable(name)
}

func (a *executionRouterAdapter) RecordOutcome(providerName, failureCategory string) {
	if a == nil || a.router == nil {
		return
	}
	a.router.RecordOutcome(providerName, failureCategory)
}

// cmdRunnerAdapter wraps a command runner function to satisfy validate.CommandRunner.
type cmdRunnerAdapter struct {
	runner func(ctx context.Context, command, workDir string) (string, string, int, error)
}

func (a *cmdRunnerAdapter) Run(ctx context.Context, command, workDir string) (string, string, int, error) {
	return a.runner(ctx, command, workDir)
}

// reviewInvokerAdapter wraps *provider.Router to satisfy review.Invoker.
type reviewInvokerAdapter struct {
	router  *provider.Router
	syncOut *syncWriter
}

func (a *reviewInvokerAdapter) StreamRun(ctx context.Context, prompt string, model string, w io.Writer) (string, error) {
	if a.router == nil {
		return "", fmt.Errorf("router is nil")
	}
	p, _ := a.router.Select("review", model)
	if p == nil {
		return "", fmt.Errorf("no provider available for review")
	}
	var out io.Writer
	var sb strings.Builder
	if w == nil {
		out = &sb
	} else {
		out = io.MultiWriter(w, &sb)
	}

	result, err := p.StreamRun(ctx, prompt, model, out, nil, nil)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("review invoker returned nil result")
	}
	return sb.String(), nil
}

// beadCreatorAdapter wraps tracker.Client to satisfy review.BeadCreator.
type beadCreatorAdapter struct {
	beads BeadClient
}

func (a *beadCreatorAdapter) Create(ctx context.Context, title string, priority int, labels []string, outputs []string) (string, error) {
	if a == nil || a.beads == nil {
		return "", fmt.Errorf("bead creator is not configured")
	}
	b, err := a.beads.Create(ctx, title, priority, labels, outputs)
	if err != nil {
		return "", err
	}
	if b == nil {
		return "", fmt.Errorf("beads.Create returned nil")
	}
	return b.ID, nil
}

// reviewRendererAdapter wraps prompt.Renderer to satisfy review.PromptRenderer.
type reviewRendererAdapter struct {
	r *prompt.Renderer
}

func (a *reviewRendererAdapter) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	return a.r.RenderReview(ctx)
}

func (a *reviewRendererAdapter) LoadRulesForPhase(phase string) (string, error) {
	return a.r.LoadRulesForPhase(phase)
}

type providerRouter interface {
	Select(phase, tier string) (provider.Provider, string)
}

type reviewPromptRenderer interface {
	RenderReview(ctx *prompt.ReviewContext) (string, error)
	LoadRulesForPhase(phase string) (string, error)
	LoadSpec(name string) (string, error)
}

type specMergeRouterAdapter struct {
	router providerRouter
}

func (a *specMergeRouterAdapter) Select(phase, tier string) (provider.Provider, string) {
	if a == nil || a.router == nil {
		return nil, ""
	}
	return a.router.Select(phase, tier)
}

type specMergeReviewRendererAdapter struct {
	renderer reviewPromptRenderer
}

func (a *specMergeReviewRendererAdapter) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	if a == nil || a.renderer == nil {
		return "", fmt.Errorf("spec merge renderer is not configured")
	}
	return a.renderer.RenderReview(ctx)
}

func (a *specMergeReviewRendererAdapter) LoadRulesForPhase(phase string) (string, error) {
	if a == nil || a.renderer == nil {
		return "", fmt.Errorf("spec merge renderer is not configured")
	}
	return a.renderer.LoadRulesForPhase(phase)
}

func (a *specMergeReviewRendererAdapter) LoadSpec(name string) (string, error) {
	if a == nil || a.renderer == nil {
		return "", fmt.Errorf("spec merge renderer is not configured")
	}
	return a.renderer.LoadSpec(name)
}

func newSpecMergeReviewDependencies(router providerRouter, renderer reviewPromptRenderer) specmerge.ReviewStageDependencies {
	return specmerge.ReviewStageDependencies{
		Router:   &specMergeRouterAdapter{router: router},
		Renderer: &specMergeReviewRendererAdapter{renderer: renderer},
	}
}

func newSpecMergeFinalizeDependencies(gitOps specmerge.GitOps, resolver specmerge.ConflictResolver, mainBranch string) specmerge.FinalizeDependencies {
	return specmerge.FinalizeDependencies{
		Git:              gitOps,
		ConflictResolver: resolver,
		MainBranch:       mainBranch,
	}
}

func newSpecMergeController(cfg *config.Config, client tracker.Client, router providerRouter, renderer reviewPromptRenderer, stageCtx *StageContext) specmerge.Controller {
	if cfg == nil || client == nil {
		return nil
	}
	if stageCtx != nil {
		_ = stageCtx
	}
	if cfg.Methodology.Granularity != config.MethodologyGranularitySpec {
		return nil
	}
	query := specmerge.NewTrackerBeadQuery(client)
	if query == nil {
		return nil
	}

	var flow specmerge.FlowExecutor
	if router != nil && renderer != nil {
		deps := newSpecMergeReviewDependencies(router, renderer)
		flow = &specmerge.TriggerFlow{
			Stages:       specmerge.ReviewStages(deps),
			DiffProvider: specmerge.DiffProviderFunc(func(ctx context.Context, specName string) (string, error) { return "", nil }),
		}
	}

	fixDeps := specmerge.FixBeadDependencies{
		BeadCreator: specmerge.NewTrackerBeadCreator(client),
	}

	retryCap := cfg.MergePipeline.RetryCapDefaultValue()

	pipeline := specmerge.NewPipeline(query, nil, flow, fixDeps, retryCap)

	if cfg.SpecPR.Enabled != nil && *cfg.SpecPR.Enabled {
		ghClient := specmerge.NewGhCLIClient(nil)
		pipeline.WithPRDeps(specmerge.PRDeps{
			PRClient: ghClient,
			GitPush: func(ctx context.Context, branch string) error {
				cmd := exec.CommandContext(ctx, "git", "push", "-u", "origin", branch)
				return cmd.Run()
			},
		})
	}

	return pipeline
}

func newStageAwarePreImplementationHook(stageCtx *StageContext) func(context.Context) error {
	if stageCtx == nil || stageCtx.SpecName == "" || stageCtx.Stage != specflow.StageAcceptanceTests {
		return nil
	}
	return func(ctx context.Context) error {
		// TODO: orchestrate acceptance-test authoring beads for stageCtx.SpecName before implementation starts.
		return nil
	}
}

var (
	_ execute.Invoker                = (*invokerAdapter)(nil)
	_ execute.PromptRenderer         = (*renderAdapter)(nil)
	_ pipelinevalidate.CommandRunner = (*cmdRunnerAdapter)(nil)
	_ review.Invoker                 = (*reviewInvokerAdapter)(nil)
	_ review.BeadCreator             = (*beadCreatorAdapter)(nil)
	_ review.PromptRenderer          = (*reviewRendererAdapter)(nil)
	_ execution.Router               = (*executionRouterAdapter)(nil)
)
