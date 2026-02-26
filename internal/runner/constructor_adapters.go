package runner

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline/epilogue"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/pipeline/review"
	pipelinevalidate "github.com/danabrams/gromit/internal/pipeline/validate"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/internal/specgate"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/validate"
	"github.com/danabrams/gromit/internal/worktree"
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
	r              *prompt.Renderer
	promptRegistry *buildPromptRegistry
}

func (a *renderAdapter) RenderBuild(title, description string, validationFailures []string) (string, error) {
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: validationFailures,
	}
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
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: validationFailures,
	}
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
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: validationFailures,
	}
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

func (a *beadCreatorAdapter) Create(title string, priority int, labels []string, outputs []string) (string, error) {
	if a == nil || a.beads == nil {
		return "", fmt.Errorf("bead creator is not configured")
	}
	b, err := a.beads.Create(context.Background(), title, priority, labels, outputs)
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

// beadLifecycleAdapter wraps tracker.Client to satisfy epilogue.BeadLifecycle.
type beadLifecycleAdapter struct {
	tracker tracker.Client
}

func (a *beadLifecycleAdapter) Close(ctx context.Context, id string) error {
	return a.tracker.Close(ctx, id)
}

func (a *beadLifecycleAdapter) Sync(ctx context.Context) error {
	return a.tracker.Sync(ctx)
}

// statusWriterAdapter wraps runner.StatusWriter to satisfy epilogue.StatusWriter.
type statusWriterAdapter struct {
	sw *StatusWriter
}

func (a *statusWriterAdapter) Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error {
	if a.sw == nil {
		return nil
	}
	return a.sw.Write(iteration, beadID, beadTitle, model, true, maxIterations, timeBudgetMinutes)
}

// worktreeMergerAdapter wraps worktree.Manager to satisfy epilogue.WorktreeMerger.
type worktreeMergerAdapter struct {
	mgr *worktree.Manager
}

func (a *worktreeMergerAdapter) PendingBranches() ([]string, error) {
	if a == nil || a.mgr == nil {
		return nil, fmt.Errorf("worktree manager is nil")
	}
	return a.mgr.PendingBranches()
}

func (a *worktreeMergerAdapter) MergeBack(branch string) error {
	if a == nil || a.mgr == nil {
		return fmt.Errorf("worktree manager is nil")
	}
	return a.mgr.MergeBack(branch)
}

func (a *worktreeMergerAdapter) DeriveSessionWorktreePath(branch string) string {
	if a == nil || a.mgr == nil {
		return ""
	}
	// Extract command and timestamp from branch name
	// Branch format: gromit/{command}-{timestamp}
	if !strings.HasPrefix(branch, "gromit/") {
		return ""
	}
	suffix := strings.TrimPrefix(branch, "gromit/")
	if suffix == "" {
		return ""
	}
	// Format: {mainDir}-gromit-{command}-{timestamp}
	return fmt.Sprintf("%s-gromit-%s", a.mgr.MainDir, suffix)
}

func (a *worktreeMergerAdapter) RemoveByPath(path string) error {
	if a == nil || a.mgr == nil {
		return fmt.Errorf("worktree manager is nil")
	}
	return a.mgr.RemoveByPath(path)
}

// epilogueCommandRunnerAdapter wraps a command runner function to satisfy epilogue.CommandRunner.
type epilogueCommandRunnerAdapter struct {
	runner func(ctx context.Context, command, workDir string) (string, string, int, error)
}

func (a *epilogueCommandRunnerAdapter) Run(ctx context.Context, command string) (string, string, int, error) {
	return a.runner(ctx, command, "")
}

// iterationLogWriterAdapter wraps *logger.Logger to satisfy epilogue.IterationLogWriter.
type iterationLogWriterAdapter struct {
	logger       *logger.Logger
	trendUpdater trendTrigger
}

func (a *iterationLogWriterAdapter) Write(log *logger.IterationLog) error {
	if a.logger == nil {
		return nil
	}
	if err := a.logger.LogIteration(log); err != nil {
		return err
	}
	if a.trendUpdater != nil {
		a.trendUpdater.Trigger()
	}
	return nil
}

type trendTrigger interface {
	Trigger()
}

// specGateAdapter wraps dependencies to satisfy epilogue.SpecGateRunner.
// It evaluates acceptance criteria for a bead when spec-level methodology is active.
type specGateAdapter struct {
	renderer *prompt.Renderer
	router   *provider.Router
	cfg      *config.Config
	logFn    func(string, ...interface{})
}

// Run evaluates the acceptance criteria for the bead using the spec gate.
func (a *specGateAdapter) Run(ctx context.Context, beadID string, labels []string) error {
	if a == nil || a.router == nil || a.cfg == nil {
		return nil
	}

	// Extract spec name from labels (format: "spec:name")
	specName := extractSpecLabel(labels)
	if specName == "" {
		return nil
	}

	// Load the spec to extract acceptance criteria
	if a.renderer == nil {
		return nil
	}
	specContent, err := a.renderer.LoadSpec(specName)
	if err != nil {
		if a.logFn != nil {
			a.logFn("Warning: could not load spec %q: %v", specName, err)
		}
		return nil
	}

	// Parse acceptance criteria from the spec
	criteria, err := coverage.ParseCriteria(specContent)
	if err != nil {
		if a.logFn != nil {
			a.logFn("Warning: could not parse criteria from spec %q: %v", specName, err)
		}
		return nil
	}

	if len(criteria) == 0 {
		return nil
	}

	// Convert criteria to strings for the gate
	criteriaTexts := make([]string, len(criteria))
	for i, c := range criteria {
		criteriaTexts[i] = c.Text
	}

	// Get the spec gate config
	if !a.cfg.SpecGate.IsEnabled() {
		return nil
	}

	// Create a gate with the necessary dependencies
	gate := &specgate.Gate{
		RunTests: func(ctx context.Context) (string, error) {
			// For now, skip test running; return empty output
			return "", nil
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return getGitDiff("")
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			// Render the gate prompt using the renderer
			if a.renderer == nil {
				return "", fmt.Errorf("renderer is nil")
			}
			criteriaText := strings.Join(criteria, "\n")
			sgCtx := &prompt.SpecGateContext{
				TestOutput:         testOutput,
				CumulativeDiff:     diff,
				AcceptanceCriteria: criteriaText,
			}
			return a.renderer.RenderSpecGate(sgCtx)
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			// Invoke the LLM with the spec gate model
			if a.router == nil {
				return nil, fmt.Errorf("router is nil")
			}
			p, _ := a.router.Select("spec-gate", provider.TierMedium)
			if p == nil {
				return nil, fmt.Errorf("no provider available for spec gate")
			}
			result, err := p.Run(ctx, prompt, a.cfg.SpecGate.Model)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, fmt.Errorf("nil result from provider")
			}
			return []byte(result.Output), nil
		},
		Model:     a.cfg.SpecGate.Model,
		MaxCycles: a.cfg.SpecGate.MaxCycles,
	}

	// Run the gate
	verdict, err := gate.Run(ctx, specName, criteriaTexts)
	if err != nil {
		if a.logFn != nil {
			a.logFn("Warning: spec gate evaluation failed: %v", err)
		}
		return nil
	}

	// Check if the gate passed
	if verdict != nil && !verdict.Passed {
		if a.logFn != nil {
			a.logFn("Warning: spec gate failed for bead %q (spec %q)", beadID, specName)
		}
		return nil
	}

	return nil
}

// extractSpecLabel returns the spec name from labels (format: "spec:name").
func extractSpecLabel(labels []string) string {
	const specPrefix = "spec:"
	for _, label := range labels {
		if strings.HasPrefix(label, specPrefix) {
			specName := strings.TrimPrefix(label, specPrefix)
			return strings.TrimSpace(specName)
		}
	}
	return ""
}

// scopeGateSubBead represents a single sub-bead from the LLM decomposition response.
type scopeGateSubBead struct {
	Title           string   `json:"title"`
	ExpectedOutputs []string `json:"expected_outputs"`
}

// decomposerAdapter uses provider routing to invoke LLM-powered decomposition of oversized beads.
type decomposerAdapter struct {
	tracker     tracker.Client
	beads       BeadClient
	router      *provider.Router
	maxSubBeads int

	mu               sync.Mutex
	createdChildKeys map[string]map[string]struct{}
}

func (a *decomposerAdapter) Decompose(ctx context.Context, b *bead.Bead) error {
	if a.router == nil {
		return fmt.Errorf("decomposerAdapter: no router configured for LLM decomposition")
	}

	p, _ := a.router.Select("decompose", provider.TierMedium)
	if p == nil {
		return fmt.Errorf("decomposerAdapter: no provider available for decomposition")
	}

	promptText := buildScopeGateDecomposePrompt(b)
	result, err := p.Run(ctx, promptText, provider.TierMedium)
	if err != nil {
		return fmt.Errorf("decomposerAdapter: LLM invocation failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("decomposerAdapter: LLM returned failure (exit %d)", result.ExitCode)
	}

	var subBeads []scopeGateSubBead
	if err := jsonutil.ExtractJSON(strings.TrimSpace(result.Output), &subBeads); err != nil {
		return fmt.Errorf("decomposerAdapter: parsing LLM output: %w", err)
	}
	if err := validateRuntimeScopeGateDecomposeOutput(subBeads, b.Title, a.maxSubBeads); err != nil {
		return err
	}

	labels := a.resolveInheritedLabels(b)
	successfullyCreatedCount := 0
	for _, sb := range subBeads {
		dedupeLabel := scopeGateChildDedupeLabel(b.ID, sb)
		if a.hasCreatedChildKey(b.ID, dedupeLabel) {
			continue
		}
		exists, err := a.childWithDedupeLabelExists(b.ID, dedupeLabel)
		if err != nil {
			return fmt.Errorf("decomposerAdapter: checking existing child bead %q: %w", sb.Title, err)
		}
		if exists {
			a.rememberCreatedChildKey(b.ID, dedupeLabel)
			continue
		}

		childLabels := append(append([]string(nil), labels...), dedupeLabel)
		beadClient := a.beads
		if beadClient == nil {
			return fmt.Errorf("decomposerAdapter: unable to access bead client")
		}
		if _, err := beadClient.CreateWithParent(context.Background(), sb.Title, b.Priority, childLabels, sb.ExpectedOutputs, b.ID); err != nil {
			// If we've already created some children and now one fails, this is a partial state
			if successfullyCreatedCount > 0 {
				return fmt.Errorf("decomposerAdapter: partial decomposition state: %w", escalation.ErrPartialDecompositionState)
			}
			return fmt.Errorf("decomposerAdapter: creating child bead %q: %w", sb.Title, err)
		}
		a.rememberCreatedChildKey(b.ID, dedupeLabel)
		successfullyCreatedCount++
	}

	beadClient := a.beads
	if beadClient == nil {
		return fmt.Errorf("decomposerAdapter: unable to access bead client for closing")
	}
	if err := beadClient.Close(context.Background(), b.ID); err != nil {
		return fmt.Errorf("decomposerAdapter: closing parent bead: %w", err)
	}
	return nil
}

func scopeGateSubBeadsToCandidates(subBeads []scopeGateSubBead) []validate.BeadCandidate {
	candidates := make([]validate.BeadCandidate, 0, len(subBeads))
	for _, sb := range subBeads {
		candidates = append(candidates, validate.BeadCandidate{
			Title:           sb.Title,
			ExpectedOutputs: sb.ExpectedOutputs,
		})
	}
	return candidates
}

func validateRuntimeScopeGateDecomposeOutput(subBeads []scopeGateSubBead, parentTitle string, maxSubBeads int) error {
	if maxSubBeads <= 0 {
		maxSubBeads = validate.MaxSubBeads
	}

	validation := validate.ValidateDecomposeOutputWithMax(
		scopeGateSubBeadsToCandidates(subBeads),
		validate.DecomposeValidationModeRuntime,
		parentTitle,
		maxSubBeads,
	)
	if len(validation.BatchViolations) > 0 {
		v := validation.BatchViolations[0]
		return fmt.Errorf("decomposerAdapter: decomposition contract violation [%s]: %s", v.Rule, v.Message)
	}
	if len(validation.Violations) > 0 {
		v := validation.Violations[0]
		return fmt.Errorf("decomposerAdapter: decomposition contract violation [%s]: bead %d: %s", v.Rule, v.BeadIndex, v.Message)
	}

	return nil
}

func scopeGateChildDedupeLabel(parentID string, sb scopeGateSubBead) string {
	const prefix = "scope_decomp:"

	outputs := make([]string, 0, len(sb.ExpectedOutputs))
	for _, output := range sb.ExpectedOutputs {
		outputs = append(outputs, normalizeScopeGateDedupeText(output))
	}
	sort.Strings(outputs)
	sum := sha1.Sum([]byte(parentID + "\x00" + normalizeScopeGateDedupeText(sb.Title) + "\x00" + strings.Join(outputs, "\x00")))
	return prefix + hex.EncodeToString(sum[:8])
}

func normalizeScopeGateDedupeText(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func (a *decomposerAdapter) hasCreatedChildKey(parentID, key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	perParent := a.createdChildKeys[parentID]
	if perParent == nil {
		return false
	}
	_, ok := perParent[key]
	return ok
}

func (a *decomposerAdapter) rememberCreatedChildKey(parentID, key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.createdChildKeys == nil {
		a.createdChildKeys = make(map[string]map[string]struct{})
	}
	if a.createdChildKeys[parentID] == nil {
		a.createdChildKeys[parentID] = make(map[string]struct{})
	}
	a.createdChildKeys[parentID][key] = struct{}{}
}

func (a *decomposerAdapter) childWithDedupeLabelExists(parentID, dedupeLabel string) (bool, error) {
	beadClient := a.beads
	if beadClient == nil {
		return false, fmt.Errorf("bead client is nil")
	}

	matches, err := beadClient.ListWithLabel(context.Background(), dedupeLabel)
	if err != nil {
		return false, err
	}
	for _, existing := range matches {
		if existing != nil && existing.Parent == parentID {
			return true, nil
		}
	}
	return false, nil
}

func (a *decomposerAdapter) resolveInheritedLabels(parent *bead.Bead) []string {
	const buildStrategyPrefix = "build_strategy:"
	const specPrefix = "spec:"

	labels := make([]string, 0, 4)
	appendUnique := func(label string) {
		if label == "" {
			return
		}
		for _, existing := range labels {
			if existing == label {
				return
			}
		}
		labels = append(labels, label)
	}

	for _, label := range labelsWithPrefix(parent.Labels, specPrefix) {
		appendUnique(label)
	}
	appendUnique(findLabelWithPrefix(parent.Labels, buildStrategyPrefix))

	beadClient := a.beads
	if beadClient == nil || parent.ID == "" {
		return labels
	}
	fullParent, err := beadClient.Show(context.Background(), parent.ID)
	if err != nil || fullParent == nil {
		return labels
	}
	for _, label := range labelsWithPrefix(fullParent.Labels, specPrefix) {
		appendUnique(label)
	}
	appendUnique(findLabelWithPrefix(fullParent.Labels, buildStrategyPrefix))

	return labels
}

func findLabelWithPrefix(labels []string, prefix string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, prefix) {
			return label
		}
	}
	return ""
}

func labelsWithPrefix(labels []string, prefix string) []string {
	matches := make([]string, 0, len(labels))
	for _, label := range labels {
		if strings.HasPrefix(label, prefix) {
			matches = append(matches, label)
		}
	}
	return matches
}

// buildScopeGateDecomposePrompt builds the LLM prompt for scope gate decomposition.
func buildScopeGateDecomposePrompt(b *bead.Bead) string {
	outputs := strings.Join(b.ExpectedOutputs, "\n- ")
	if outputs != "" {
		outputs = "- " + outputs
	}
	return fmt.Sprintf(`You are decomposing an oversized task into smaller sub-tasks that each touch 5 or fewer files.

## Oversized Task
Title: %s
Description: %s
Expected outputs (too many):
%s

## Instructions
Split this task into 2-5 sub-tasks. Each sub-task must:
- Have a clear, specific title
- Touch 5 or fewer files (expected_outputs list)
- Together cover all the work of the original task

## Output
Output ONLY a JSON array. No markdown, no explanation.
Each element: {"title": "...", "expected_outputs": ["file1", "file2", ...]}
`, b.Title, b.Description, outputs)
}

// failureLearnerAdapter wraps analyzer and related dependencies to satisfy epilogue.FailureLearner.
type failureLearnerAdapter struct {
	renderer *prompt.Renderer
	router   *provider.Router
	analyzer FailureAnalyzer
	logFn    func(string, ...interface{})
}

func (a *failureLearnerAdapter) ExtractFailureLearning(ctx context.Context, beadID, beadTitle, failureOutput string) error {
	if a.analyzer == nil {
		return nil
	}
	b := &bead.Bead{ID: beadID, Title: beadTitle}
	analysis, err := a.analyzer.Analyze(ctx, b, failureOutput)
	if err != nil {
		if a.logFn != nil {
			a.logFn("Warning: failure analysis error: %v", err)
		}
		return nil
	}
	if analysis == nil || analysis.Learning == nil {
		return nil
	}
	if a.renderer != nil {
		lf := a.renderer.GetLearningsFile()
		if lf != nil {
			_, _ = lf.Add(beadID, *analysis.Learning, analysis.LearningCategory())
		}
	}
	return nil
}

// selectEscalationHandler selects the appropriate escalation handler based on routing strategy.
// For cost_optimized strategy, returns a DecomposeFirstHandler.
// For priority_based strategy (or default), returns the standard Handler.
func selectEscalationHandler(
	cfg *config.Config,
	analyzer escalation.FailureAnalyzer,
	beadClient escalation.BeadClient,
	decomposeFn escalation.DecomposeFn,
	createSubFn escalation.CreateSubFn,
	logFn escalation.LogFn,
	showPartialProgressFn escalation.ShowPartialProgressFn,
) interface{} {
	if cfg == nil {
		// Default to Handler when config is nil
		return escalation.NewHandler(cfg, analyzer, beadClient, decomposeFn, createSubFn, logFn, showPartialProgressFn)
	}

	strategy := strings.TrimSpace(cfg.Routing.Strategy)
	if strings.EqualFold(strategy, "cost_optimized") {
		// Use DecomposeFirstHandler for cost_optimized strategy
		maxRetries := cfg.Routing.CostOptimized.MaxRetriesBeforeDecompose
		if maxRetries <= 0 {
			maxRetries = 2 // Default if not configured
		}
		return escalation.NewDecomposeFirstHandler(
			cfg,
			analyzer,
			beadClient,
			decomposeFn,
			createSubFn,
			logFn,
			showPartialProgressFn,
			maxRetries,
		)
	}

	// Default to Handler for priority_based and empty strategy
	return escalation.NewHandler(cfg, analyzer, beadClient, decomposeFn, createSubFn, logFn, showPartialProgressFn)
}

// toJSONList converts a string slice to a JSON array string.
func toJSONList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(items)
	return string(data)
}

// Compile-time interface checks
var _ execute.Invoker = (*invokerAdapter)(nil)
var _ execute.PromptRenderer = (*renderAdapter)(nil)
var _ pipelinevalidate.CommandRunner = (*cmdRunnerAdapter)(nil)
var _ review.Invoker = (*reviewInvokerAdapter)(nil)
var _ review.BeadCreator = (*beadCreatorAdapter)(nil)
var _ review.PromptRenderer = (*reviewRendererAdapter)(nil)
var _ epilogue.BeadLifecycle = (*beadLifecycleAdapter)(nil)
var _ epilogue.StatusWriter = (*statusWriterAdapter)(nil)
var _ epilogue.WorktreeMerger = (*worktreeMergerAdapter)(nil)
var _ epilogue.CommandRunner = (*epilogueCommandRunnerAdapter)(nil)
var _ epilogue.IterationLogWriter = (*iterationLogWriterAdapter)(nil)
var _ epilogue.SpecGateRunner = (*specGateAdapter)(nil)
var _ prepare.Decomposer = (*decomposerAdapter)(nil)
var _ epilogue.FailureLearner = (*failureLearnerAdapter)(nil)
var _ execution.Router = (*executionRouterAdapter)(nil)
