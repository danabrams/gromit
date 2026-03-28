package loop

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/queue"
	"github.com/danabrams/gromit/internal/v2/dep"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/generation"
	v2review "github.com/danabrams/gromit/internal/v2/review"
	"github.com/danabrams/gromit/internal/v2/routing"
	"github.com/danabrams/gromit/internal/v2/stage"
	buildstage "github.com/danabrams/gromit/internal/v2/stage/build"
	reviewstage "github.com/danabrams/gromit/internal/v2/stage/review"
	"github.com/danabrams/gromit/internal/v2/stage/triage"
)

// defaultTierToModel maps abstract tier names to model names when no
// provider-specific model mapping is available (i.e. no router configured).
var defaultTierToModel = map[string]string{
	"low":    "haiku",
	"medium": "sonnet",
	"high":   "opus",
}

// GitCommitter abstracts git status and commit operations for the bead loop.
type GitCommitter interface {
	Status(ctx context.Context, worktree string) (string, error)
	Commit(ctx context.Context, worktree, message string) (string, error)
}

// StageCommitter commits work after each successful stage.
type StageCommitter interface {
	CommitStage(ctx context.Context, worktree, beadID, stageName string, iteration int, decision string) error
}

// BeadLoopConfig holds the stages required to process each bead.
type BeadLoopConfig struct {
	Gate                       stage.Stage
	Build                      stage.Stage
	Validate                   stage.Stage
	Review                     stage.Stage
	Epilogue                   stage.Stage
	Triage                     stage.Stage // optional triage stage for failure categorization
	Decompose                  stage.Stage // optional decompose stage for bead splitting
	Emitter                    *event.Emitter
	LegacyEmitter              *events.Emitter
	GenerationCap              int // generation cap for review-created beads
	DecompositionGenerationCap int // generation cap for triage-decomposed beads (separate from review)
	StartGeneration            int
	Git                        GitCommitter
	StageCommitter             StageCommitter
	Router                     *routing.Router   // optional: if nil, routing is skipped
	PhaseModels                map[string]string // optional: phase -> starting tier
}

// BeadLoopResult captures metadata produced by a bead loop execution.
type BeadLoopResult struct {
	OutOfScopeFindings []v2review.Finding
}

// BeadLoop orchestrates the per-bead execution pipeline.
type BeadLoop struct {
	gate                       stage.Stage
	build                      stage.Stage
	validate                   stage.Stage
	review                     stage.Stage
	epilogue                   stage.Stage
	triage                     stage.Stage
	decompose                  stage.Stage
	emitter                    *event.Emitter
	generationCap              int
	decompositionGenerationCap int
	startGeneration            int
	worktree                   string
	outOfScopeFindings         []v2review.Finding
	git                        GitCommitter
	stageCommitter             StageCommitter
	router                     *routing.Router
	phaseModels                map[string]string
	// lastBuildArtifacts stores the build artifacts from the most recent
	// successful build stage run for the current bead. Reset at the start
	// of each processBead call.
	lastBuildArtifacts *buildstage.BuildArtifacts
	// lastBuildProvider stores the provider name from the most recent
	// successful build stage run for the current bead.
	lastBuildProvider string
	// run-scoped state for in-loop decomposition
	resolver  *dep.Resolver
	beadMap   map[string]*bead.Bead
	completed []string
}

var ErrGenerationCapReached = errors.New("generation cap reached")

// errBeadSkipped is a sentinel returned when the gate decides to skip a bead.
// The loop marks the bead completed and continues to the next one.
var errBeadSkipped = errors.New("bead skipped by gate")

// errBeadBlocked is a sentinel returned when the gate decides to block a bead.
// The loop defers the bead (does not mark it completed) and continues.
var errBeadBlocked = errors.New("bead blocked by gate")

// errBeadFailed is a sentinel returned when a bead fails its stage pipeline
// (build timeout, validation failure, etc.) but the loop can continue to
// process remaining beads.
var errBeadFailed = errors.New("bead failed")

// maxTriageRetries caps how many times triage can return CategoryRetry
// before falling through to normal failure handling.
const maxTriageRetries = 3

// NewBeadLoop constructs a BeadLoop tagged with the provided stages.
func NewBeadLoop(config BeadLoopConfig) (*BeadLoop, error) {
	if config.Gate == nil {
		return nil, fmt.Errorf("gate stage required")
	}
	if config.Build == nil {
		return nil, fmt.Errorf("build stage required")
	}
	if config.Validate == nil {
		return nil, fmt.Errorf("validate stage required")
	}
	if config.Review == nil {
		return nil, fmt.Errorf("review stage required")
	}
	if config.Epilogue == nil {
		return nil, fmt.Errorf("epilogue stage required")
	}
	loop := &BeadLoop{
		gate:                       config.Gate,
		build:                      config.Build,
		validate:                   config.Validate,
		review:                     config.Review,
		epilogue:                   config.Epilogue,
		triage:                     config.Triage,
		decompose:                  config.Decompose,
		emitter:                    config.Emitter,
		generationCap:              config.GenerationCap,
		decompositionGenerationCap: config.DecompositionGenerationCap,
		startGeneration:            config.StartGeneration,
		git:                        config.Git,
		stageCommitter:             config.StageCommitter,
		router:                     config.Router,
		phaseModels:                config.PhaseModels,
	}
	if config.Emitter != nil && config.LegacyEmitter != nil {
		event.BridgeTypedToLegacy(config.Emitter, config.LegacyEmitter)
	}
	return loop, nil
}

// SetWorktree identifies the worktree path commands should run inside.
func (b *BeadLoop) SetWorktree(worktree string) {
	if b == nil {
		return
	}
	b.worktree = strings.TrimSpace(worktree)
}

// Run processes the provided beads through the stage pipeline.
func (b *BeadLoop) Run(ctx context.Context, beads []*bead.Bead, stopCh <-chan struct{}) (BeadLoopResult, error) {
	highestGeneration := 0
	generationLimit := 0
	b.outOfScopeFindings = b.outOfScopeFindings[:0]
	if b.generationCap > 0 {
		generationLimit = b.startGeneration + b.generationCap
		highestGeneration = b.findHighestGeneration(beads)
		if highestGeneration >= generationLimit {
			b.emitGenerationCapReached(highestGeneration)
			return BeadLoopResult{}, ErrGenerationCapReached
		}
	}

	// Pre-sort beads topologically so the resolver's add-order reflects
	// dependency ordering. TopologicalSort provides the initial execution
	// order; dep.Resolver handles dynamic re-ordering during execution
	// (blocking/unblocking as beads complete).
	sorted, err := TopologicalSort(beads)
	if err != nil {
		return BeadLoopResult{}, fmt.Errorf("topological pre-sort: %w", err)
	}
	beads = sorted

	b.resolver = dep.NewResolver()
	b.beadMap = make(map[string]*bead.Bead, len(beads))
	b.completed = nil

	for _, beadItem := range beads {
		if beadItem == nil {
			continue
		}
		id := strings.TrimSpace(beadItem.ID)
		if id == "" {
			continue
		}
		b.beadMap[id] = beadItem
		b.resolver.Add(id, collectDependencies(beadItem))
	}

	// blockedThisPass tracks bead IDs blocked by the gate in the current pass.
	// These are passed to the resolver as "effective completed" so it can
	// return their dependents, but they are NOT in b.completed and will be
	// retried at the start of the next pass if any progress was made.
	var blockedThisPass []string
	passProgress := 0

	iteration := 1
runLoop:
	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return BeadLoopResult{}, ctx.Err()
			default:
			}
		}
		if stopCh != nil {
			select {
			case <-stopCh:
				break runLoop
			default:
			}
		}

		// Build the effective completed set: real completed + blocked-this-pass
		// so the resolver skips blocked beads and returns their dependents.
		effectiveCompleted := b.completed
		if len(blockedThisPass) > 0 {
			effectiveCompleted = make([]string, 0, len(b.completed)+len(blockedThisPass))
			effectiveCompleted = append(effectiveCompleted, b.completed...)
			effectiveCompleted = append(effectiveCompleted, blockedThisPass...)
		}

		next, err := b.resolver.Next(effectiveCompleted)
		if err != nil {
			return BeadLoopResult{}, fmt.Errorf("resolve bead: %w", err)
		}
		if next == "" {
			if len(blockedThisPass) == 0 {
				// Normal completion — no blocked beads pending.
				break
			}
			if passProgress == 0 {
				// Full pass with zero progress: all remaining beads are blocked.
				blockedIDs := strings.Join(blockedThisPass, ", ")
				return BeadLoopResult{}, fmt.Errorf("bead loop: all remaining beads blocked with no progress: %s", blockedIDs)
			}
			// Progress was made this pass — retry the blocked beads.
			blockedThisPass = nil
			passProgress = 0
			continue
		}
		beadItem, ok := b.beadMap[next]
		if !ok {
			return BeadLoopResult{}, fmt.Errorf("bead %q missing from input list", next)
		}
		// TODO(andon-spec): budget policy integration — currently no-op, deferred to Andon spec
		if err := b.checkBudget(ctx); err != nil {
			return BeadLoopResult{}, fmt.Errorf("budget check before bead %s: %w", beadItem.ID, err)
		}
		b.emitBeadStarted(beadItem, iteration)
		if err := b.processBead(ctx, beadItem, iteration, &highestGeneration, generationLimit, stopCh); err != nil {
			if errors.Is(err, errBeadSkipped) {
				// Gate skipped this bead — mark completed and continue to next
				b.emitBeadCompleted(beadItem, iteration, true, 0)
				b.completed = append(b.completed, next)
				passProgress++
				iteration++
				continue
			}
			if errors.Is(err, errBeadBlocked) {
				// Gate blocked this bead — re-queue to end of current pass.
				// Do NOT mark completed; the bead will be retried after other
				// beads in this pass make progress. If no progress is made
				// after a full pass, the loop stops with an error.
				blockedThisPass = append(blockedThisPass, next)
				iteration++
				continue
			}
			// Recoverable per-bead failure (build timeout, stage exhaustion,
			// etc.) — the bead's epilogue already ran and events were
			// emitted by failWithReason. Log, mark completed, continue.
			if errors.Is(err, errBeadFailed) {
				log.Printf("bead %s failed (continuing loop): %v", next, err)
				b.completed = append(b.completed, next)
				passProgress++
				iteration++
				continue
			}
			// All other errors (triage unsafe, gate infrastructure failure,
			// context cancellation, generation cap) are fatal.
			return BeadLoopResult{}, err
		}
		b.emitBeadCompleted(beadItem, iteration, true, 0)
		b.completed = append(b.completed, next)
		passProgress++
		iteration++
	}
	return BeadLoopResult{
		OutOfScopeFindings: cloneOutOfScopeFindings(b.outOfScopeFindings),
	}, nil
}

type stageEntry struct {
	stage       stage.Stage
	shouldFail  func(*stage.Result) bool
	failMessage func(*stage.Result) string
	retryConfig stage.RetryConfig
}

func (b *BeadLoop) processBead(ctx context.Context, beadItem *bead.Bead, iteration int, highestGen *int, generationLimit int, stopCh <-chan struct{}) error {
	b.lastBuildArtifacts = nil // reset for each bead
	b.lastBuildProvider = ""
	// Run gate stage first; skip/block return sentinels instead of halting.
	if err := b.runGate(ctx, beadItem, iteration); err != nil {
		return err
	}

	stages := []stageEntry{
		{
			stage: b.build,
			shouldFail: func(res *stage.Result) bool {
				return stageDecision(res) == stage.DecisionFail
			},
			failMessage: func(res *stage.Result) string {
				return fmt.Sprintf("%s returned %s", b.build.Name(), stageDecision(res))
			},
			retryConfig: retryConfigForStage(b.build),
		},
		{
			stage: b.validate,
			shouldFail: func(res *stage.Result) bool {
				return stageDecision(res) == stage.DecisionFail
			},
			failMessage: func(res *stage.Result) string {
				return fmt.Sprintf("%s returned %s", b.validate.Name(), stageDecision(res))
			},
			retryConfig: retryConfigForStage(b.validate),
		},
		{
			stage: b.review,
			shouldFail: func(res *stage.Result) bool {
				return stageDecision(res) != stage.DecisionProceed
			},
			failMessage: func(res *stage.Result) string {
				return fmt.Sprintf("%s returned %s", b.review.Name(), stageDecision(res))
			},
			retryConfig: retryConfigForStage(b.review),
		},
	}

	nameToIndex := make(map[string]int, len(stages))
	for idx, entry := range stages {
		name := stageName(entry.stage)
		if name != "" {
			nameToIndex[name] = idx
		}
	}

	for idx := range stages {
		if err := b.runStageEntry(ctx, beadItem, iteration, stages, nameToIndex, idx, highestGen, generationLimit, stopCh); err != nil {
			return err
		}
	}

	if err := b.runEpilogue(ctx, beadItem, iteration, nil); err != nil {
		return err
	}
	return nil
}

// runGate runs the gate stage and returns sentinel errors for skip/block decisions.
func (b *BeadLoop) runGate(ctx context.Context, beadItem *bead.Bead, iteration int) error {
	if b.gate == nil {
		return nil
	}
	name := stageName(b.gate)
	req := b.stageRequest(beadItem, iteration, nil)
	b.emitStageStarted(name, beadItem.ID, iteration)
	start := time.Now()
	res, err := b.runStage(ctx, b.gate, req)
	duration := time.Since(start)
	if err != nil {
		b.emitStageCompleted(name, beadItem.ID, iteration, false, duration)
		b.emitStageFailed(name, beadItem.ID, iteration, err.Error())
		return err
	}
	decision := stageDecision(res)
	switch decision {
	case stage.DecisionSkip:
		b.emitStageCompleted(name, beadItem.ID, iteration, true, duration)
		return errBeadSkipped
	case stage.DecisionBlock:
		b.emitStageCompleted(name, beadItem.ID, iteration, true, duration)
		return errBeadBlocked
	case stage.DecisionFail:
		reason := fmt.Sprintf("%s decided %s", b.gate.Name(), decision)
		b.emitStageCompleted(name, beadItem.ID, iteration, false, duration)
		b.emitStageFailed(name, beadItem.ID, iteration, reason)
		return b.failWithReason(ctx, beadItem, iteration, reason, nil)
	default:
		b.emitStageCompleted(name, beadItem.ID, iteration, true, duration)
		return nil
	}
}

// commitAfterStage calls StageCommitter.CommitStage after a successful stage run.
func (b *BeadLoop) commitAfterStage(ctx context.Context, beadItem *bead.Bead, sName string, iteration int, decision string) error {
	if b.stageCommitter == nil {
		return nil
	}
	return b.stageCommitter.CommitStage(ctx, b.worktree, beadItem.ID, sName, iteration, capitalizedDecision(decision))
}

func (b *BeadLoop) runStageEntry(ctx context.Context, beadItem *bead.Bead, iteration int, entries []stageEntry, nameIndex map[string]int, idx int, highestGen *int, generationLimit int, stopCh <-chan struct{}) error {
	entry := entries[idx]
	if entry.stage == nil {
		return nil
	}
	stageName := stageName(entry.stage)
	if stageName == "" {
		return nil
	}

	// Determine starting tier for this stage's phase via routing.
	// TierForPhase is consulted even when no router is configured so that
	// phaseModels (e.g. "build" → "low") still control model selection
	// through the stage's own default provider.
	phase := barePhase(stageName)
	startTier := routing.TierForPhase(phase, b.phaseModels, routing.TierMedium)

	retriesRemaining := entry.retryConfig.MaxRetries
	if retriesRemaining < 0 {
		retriesRemaining = 0
	}
	attempt := 1
	triageRetries := 0
	priorFailures := []string{}

	for {
		req := b.stageRequest(beadItem, iteration, stageRetryContext(attempt, priorFailures))

		// Apply routing: select provider and model for this stage.
		escalationLevel := 0
		if req.RetryContext != nil {
			escalationLevel = req.RetryContext.EscalationLevel
		}
		tier := routing.EscalationTier(startTier, escalationLevel)
		req.Tier = string(tier)
		providerName := ""
		if b.router != nil {
			provider, model, pName, routeErr := b.router.Select(phase, tier)
			if routeErr != nil {
				log.Printf("WARNING: routing failed for stage %s tier %s: %v (using default provider)", stageName, tier, routeErr)
			} else if provider != nil {
				req.Provider = provider
				req.Model = model
				providerName = pName
				if entry.stage == b.build {
					b.emitModelSelected(beadItem.ID, model, providerName, string(tier), "router")
				}
			}
		} else if req.Model == "" {
			// No router configured — resolve tier to model name so the
			// stage's default provider uses the correct model.
			req.Model = routing.ResolveModel(tier, defaultTierToModel)
		}

		b.emitStageStarted(stageName, beadItem.ID, iteration)
		if entry.stage == b.build {
			b.emitBuildInvocationStart(beadItem.ID, req.Model, providerName, string(tier), attempt, retriesRemaining+attempt)
		}
		start := time.Now()
		res, err := b.runStage(ctx, entry.stage, req)
		duration := time.Since(start)

		failed := false
		reason := ""
		if err != nil {
			failed = true
			reason = err.Error()
		} else if entry.shouldFail != nil && entry.shouldFail(res) {
			failed = true
			reason = entry.failMessage(res)
		}

		if entry.stage == b.build {
			var costUSD float64
			var inputTokens int
			var outputTokens int
			var promptSize int
			if res != nil && res.Artifacts != nil {
				if ba, ok := res.Artifacts.(*buildstage.BuildArtifacts); ok {
					costUSD = ba.CostUSD
					inputTokens = ba.InputTokens
					outputTokens = ba.OutputTokens
					promptSize = len(ba.Prompt)
				}
			}
			b.emitBuildInvocationComplete(beadItem.ID, req.Model, providerName, !failed, duration, costUSD, inputTokens, outputTokens, promptSize)
		}

		b.emitStageCompleted(stageName, beadItem.ID, iteration, !failed, duration)
		if failed {
			b.emitStageFailed(stageName, beadItem.ID, iteration, reason)
			decision := stage.DecisionFail.String()
			if res != nil {
				decision = stageDecision(res).String()
			}
			if err := b.commitAfterStage(ctx, beadItem, stageName, iteration, decision); err != nil {
				return fmt.Errorf("stage commit after %s: %w", stageName, err)
			}
			priorFailures = append(priorFailures, reason)
		} else {
			// Capture build artifacts for bead-level reporting.
			if entry.stage == b.build && res != nil && res.Artifacts != nil {
				if artifacts, ok := res.Artifacts.(*buildstage.BuildArtifacts); ok {
					b.lastBuildArtifacts = artifacts
					b.lastBuildProvider = providerName
				}
			}
			if entry.stage == b.review {
				b.collectOutOfScopeFindings(res)
				if b.handleGenerationCapFromReview(res, highestGen, generationLimit) {
					return ErrGenerationCapReached
				}
			}
			decision := stageDecision(res).String()
			if err := b.commitAfterStage(ctx, beadItem, stageName, iteration, decision); err != nil {
				return fmt.Errorf("stage commit after %s: %w", stageName, err)
			}
			return nil
		}

		if retriesRemaining <= 0 {
			// When the build stage fails, retries are exhausted, and triage
			// is configured, run triage to decide how to proceed instead of
			// immediately failing.
			if entry.stage == b.build && b.triage != nil {
				triageResult, triageErr := b.runTriage(ctx, beadItem, iteration, reason)
				if triageErr != nil {
					return fmt.Errorf("triage: %w", triageErr)
				}
				switch triageResult.Category {
				case triage.CategoryDecompose:
					return b.decomposeAndRunSubBeads(ctx, beadItem, iteration, highestGen)
				case triage.CategoryRetry:
					triageRetries++
					if triageRetries > maxTriageRetries {
						// Exceeded triage retry cap; fall through to normal failure handling
						break
					}
					// Don't count against retry limit, just continue the loop
					attempt++
					continue
				case triage.CategoryUnclearSpec:
					return fmt.Errorf("triage: spec unclear for bead %s: %s", beadItem.ID, triageResult.Reasoning)
				case triage.CategoryUnsafe:
					return fmt.Errorf("triage: unsafe operation for bead %s: %s", beadItem.ID, triageResult.Reasoning)
				}
			}
			return b.failWithReason(ctx, beadItem, iteration, reason, stageRetryContext(attempt, priorFailures))
		}

		// TODO(andon-spec): budget policy integration — currently no-op, deferred to Andon spec
		if err := b.checkBudget(ctx); err != nil {
			return fmt.Errorf("budget check before stage retry: %w", err)
		}

		retriesRemaining--
		attempt++
		b.emitStageRetrying(stageName, beadItem.ID, attempt, reason)

		if err := b.runRetryWithStages(ctx, beadItem, iteration, entries, nameIndex, entry.retryConfig.RetryWith, idx, highestGen, generationLimit, stopCh); err != nil {
			return err
		}
	}
}

func (b *BeadLoop) runRetryWithStages(ctx context.Context, beadItem *bead.Bead, iteration int, entries []stageEntry, nameIndex map[string]int, retryWith []string, currentIdx int, highestGen *int, generationLimit int, stopCh <-chan struct{}) error {
	for _, retryName := range retryWith {
		idx, ok := nameIndex[retryName]
		if !ok {
			log.Printf("retry-with stage %q not found in stage index, skipping", retryName)
			continue
		}
		if idx >= currentIdx {
			log.Printf("retry-with stage %q (index %d) is not before current stage (index %d), skipping", retryName, idx, currentIdx)
			continue
		}
		if err := b.runStageEntry(ctx, beadItem, iteration, entries, nameIndex, idx, highestGen, generationLimit, stopCh); err != nil {
			return err
		}
	}
	return nil
}

func stageRetryContext(attempt int, priorFailures []string) *stage.RetryContext {
	if attempt <= 1 && len(priorFailures) == 0 {
		return nil
	}
	escalation := 0
	if attempt > 1 {
		escalation = attempt - 1
	}
	ctx := &stage.RetryContext{
		Attempt:         attempt,
		EscalationLevel: escalation,
	}
	if len(priorFailures) > 0 {
		ctx.PriorFailures = append([]string(nil), priorFailures...)
	}
	return ctx
}

func (b *BeadLoop) runStage(ctx context.Context, staged stage.Stage, req stage.Request) (*stage.Result, error) {
	if staged == nil {
		return nil, nil
	}
	res, err := staged.Run(ctx, &req)
	if err != nil {
		return res, fmt.Errorf("stage %s: %w", staged.Name(), err)
	}
	return res, nil
}

func (b *BeadLoop) failWithReason(ctx context.Context, beadItem *bead.Bead, iteration int, reason string, retryCtx *stage.RetryContext) error {
	ctxToUse := retryCtx
	if ctxToUse == nil {
		ctxToUse = &stage.RetryContext{
			Attempt:       1,
			PriorFailures: []string{reason},
		}
	} else {
		copyCtx := &stage.RetryContext{
			Attempt:         ctxToUse.Attempt,
			EscalationLevel: ctxToUse.EscalationLevel,
		}
		if len(ctxToUse.PriorFailures) > 0 {
			copyCtx.PriorFailures = append([]string(nil), ctxToUse.PriorFailures...)
		}
		ctxToUse = copyCtx
	}
	if err := b.runEpilogue(ctx, beadItem, iteration, ctxToUse); err != nil {
		return fmt.Errorf("epilogue failure: %w", err)
	}
	retryAttempt := ctxToUse.Attempt
	if retryAttempt <= 0 {
		retryAttempt = 1
	}
	b.emitBeadCompleted(beadItem, iteration, false, retryAttempt)
	return fmt.Errorf("bead %s failed: %s: %w", beadItem.ID, reason, errBeadFailed)
}

func (b *BeadLoop) runEpilogue(ctx context.Context, beadItem *bead.Bead, iteration int, retryCtx *stage.RetryContext) error {
	stageName := stageName(b.epilogue)
	req := b.stageRequest(beadItem, iteration, retryCtx)
	b.emitStageStarted(stageName, beadItem.ID, iteration)
	start := time.Now()
	_, err := b.runStage(ctx, b.epilogue, req)
	duration := time.Since(start)
	if err != nil {
		b.emitStageCompleted(stageName, beadItem.ID, iteration, false, duration)
		b.emitStageFailed(stageName, beadItem.ID, iteration, err.Error())
		return err
	}
	b.emitStageCompleted(stageName, beadItem.ID, iteration, true, duration)
	return nil
}

func stageDecision(res *stage.Result) stage.Decision {
	if res == nil {
		return stage.DecisionProceed
	}
	return res.Decision
}

// checkBudget is a no-op stub for budget integration.
// TODO(andon-spec): implement budget policy checks here.
func (l *BeadLoop) checkBudget(ctx context.Context) error {
	return nil
}

func (b *BeadLoop) stageRequest(beadItem *bead.Bead, iteration int, retryCtx *stage.RetryContext) stage.Request {
	labels := copyLabels(beadItem.Labels)
	deps := collectDependencies(beadItem)
	if deps == nil {
		deps = []string{}
	}
	return stage.Request{
		Bead: stage.BeadInfo{
			ID:           beadItem.ID,
			Title:        beadItem.Title,
			Description:  beadItem.Description,
			Priority:     strconv.Itoa(beadItem.Priority),
			Labels:       labels,
			Dependencies: deps,
		},
		Iteration:    iteration,
		RetryContext: retryCtx,
		Worktree:     b.worktree,
	}
}

func copyLabels(labels []string) []string {
	if len(labels) == 0 {
		return nil
	}
	copied := make([]string, len(labels))
	copy(copied, labels)
	return copied
}

func collectDependencies(b *bead.Bead) []string {
	if b == nil {
		return nil
	}
	deps := queue.DependencyIDs(b.DependsOn)
	deps = append(deps, queue.DependencyIDs(b.Dependencies)...)
	deps = append(deps, queue.DependencyIDs(b.BlockedBy)...)
	return deps
}

func (b *BeadLoop) emitBeadStarted(beadItem *bead.Bead, iteration int) {
	if b.emitter == nil || beadItem == nil {
		return
	}
	b.emitter.Emit(event.BeadStartedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeBeadStarted,
		},
		BeadID:    beadItem.ID,
		BeadTitle: beadItem.Title,
		Iteration: iteration,
	})
}

func (b *BeadLoop) emitBeadCompleted(beadItem *bead.Bead, iteration int, success bool, retryAttempt int) {
	if b.emitter == nil || beadItem == nil {
		return
	}
	evt := event.BeadCompletedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeBeadCompleted,
		},
		BeadID:       beadItem.ID,
		BeadTitle:    beadItem.Title,
		Iteration:    iteration,
		Success:      success,
		RetryAttempt: retryAttempt,
	}
	if b.lastBuildArtifacts != nil {
		evt.Model = b.lastBuildArtifacts.Model
		evt.Provider = b.lastBuildProvider
		evt.CostUSD = b.lastBuildArtifacts.CostUSD
		evt.InputTokens = b.lastBuildArtifacts.InputTokens
		evt.OutputTokens = b.lastBuildArtifacts.OutputTokens
		evt.Duration = b.lastBuildArtifacts.Duration
	}
	b.emitter.Emit(evt)
}

func (b *BeadLoop) emitStageStarted(stageName, beadID string, iteration int) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.StageStartedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeStageStarted,
		},
		StageName: stageName,
		BeadID:    beadID,
		Iteration: iteration,
	})
}

func (b *BeadLoop) emitStageCompleted(stageName, beadID string, iteration int, success bool, duration time.Duration) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.StageCompletedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeStageCompleted,
		},
		StageName: stageName,
		BeadID:    beadID,
		Iteration: iteration,
		Success:   success,
		Duration:  duration,
	})
}

func (b *BeadLoop) emitStageFailed(stageName, beadID string, iteration int, reason string) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.StageFailedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeStageFailed,
		},
		StageName: stageName,
		BeadID:    beadID,
		Iteration: iteration,
		Error:     reason,
	})
}

func (b *BeadLoop) emitStageRetrying(stageName, beadID string, attempt int, reason string) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.StageRetryingEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeStageRetrying,
		},
		StageName: stageName,
		BeadID:    beadID,
		Attempt:   attempt,
		Reason:    reason,
	})
}

func stageName(st stage.Stage) string {
	if st == nil {
		return ""
	}
	return st.Name()
}

// barePhase extracts the bare phase name from a profile-prefixed stage name.
// E.g., "go:build" -> "build", "build:default" -> "build", "build" -> "build".
func barePhase(stageName string) string {
	if idx := strings.LastIndex(stageName, ":"); idx >= 0 {
		after := stageName[idx+1:]
		before := stageName[:idx]
		if after == "default" {
			return before
		}
		return after
	}
	return stageName
}

type retryConfigurer interface {
	RetryConfig() stage.RetryConfig
}

func retryConfigForStage(st stage.Stage) stage.RetryConfig {
	if st == nil {
		return stage.RetryConfig{}
	}
	if rc, ok := st.(retryConfigurer); ok {
		return rc.RetryConfig()
	}
	return stage.RetryConfig{}
}

func (b *BeadLoop) findHighestGeneration(beads []*bead.Bead) int {
	maxGen := 0
	for _, bead := range beads {
		if bead == nil {
			continue
		}
		gen := generation.Current(bead.Labels)
		if gen > maxGen {
			maxGen = gen
		}
	}
	return maxGen
}

func (b *BeadLoop) handleGenerationCapFromReview(res *stage.Result, highestGen *int, generationLimit int) bool {
	if generationLimit <= 0 || highestGen == nil || res == nil || res.Artifacts == nil {
		return false
	}
	artifacts, ok := res.Artifacts.(*reviewstage.ReviewArtifacts)
	if !ok {
		return false
	}
	maxGen := *highestGen
	for _, created := range artifacts.CreatedBeads {
		if created == nil {
			continue
		}
		if gen := generation.Current(created.Labels); gen > maxGen {
			maxGen = gen
		}
	}
	*highestGen = maxGen
	if maxGen >= generationLimit {
		b.emitGenerationCapReached(maxGen)
		return true
	}
	return false
}

func (b *BeadLoop) collectOutOfScopeFindings(res *stage.Result) {
	if res == nil || res.Artifacts == nil {
		return
	}
	artifacts, ok := res.Artifacts.(*reviewstage.ReviewArtifacts)
	if !ok {
		return
	}
	for _, finding := range artifacts.OutOfScope {
		b.outOfScopeFindings = append(b.outOfScopeFindings, finding)
	}
}

func (b *BeadLoop) emitGenerationCapReached(highestGeneration int) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.GenerationCapReachedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeGenerationCapReached,
		},
		GenerationCap:     b.generationCap,
		HighestGeneration: highestGeneration,
	})
}

// runTriage runs the triage stage to categorize a build failure.
func (b *BeadLoop) runTriage(ctx context.Context, beadItem *bead.Bead, iteration int, failureReason string) (*triage.TriageArtifacts, error) {
	req := b.stageRequest(beadItem, iteration, &stage.RetryContext{
		Attempt:       1,
		PriorFailures: []string{failureReason},
	})
	// Apply routing to triage stage.
	triageTier := routing.TierForPhase("triage", b.phaseModels, routing.TierMedium)
	if b.router != nil {
		provider, model, _, routeErr := b.router.Select("triage", triageTier)
		if routeErr != nil {
			log.Printf("WARNING: routing failed for triage tier %s: %v (using default provider)", triageTier, routeErr)
		} else if provider != nil {
			req.Provider = provider
			req.Model = model
		}
	} else if req.Model == "" {
		req.Model = routing.ResolveModel(triageTier, defaultTierToModel)
	}
	b.emitTriageStarted(beadItem.ID, beadItem.Title, iteration)
	res, err := b.runStage(ctx, b.triage, req)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("triage stage returned no result")
	}
	artifacts, ok := res.Artifacts.(*triage.TriageArtifacts)
	if !ok {
		return nil, fmt.Errorf("triage stage returned unexpected artifacts type %T", res.Artifacts)
	}
	b.emitTriageCompleted(beadItem.ID, beadItem.Title, iteration, string(artifacts.Category), artifacts.Reasoning)
	return artifacts, nil
}

// decomposeAndRunSubBeads runs the decompose stage on the failing bead and
// splices the resulting sub-beads into the current loop's dependency graph.
// Dependents of the original bead are rewired to depend on all sub-beads.
// The original bead is marked completed (replaced) and the main loop
// picks up the sub-beads naturally via resolver.Next().
func (b *BeadLoop) decomposeAndRunSubBeads(ctx context.Context, beadItem *bead.Bead, iteration int, highestGen *int) error {
	if b.decompose == nil {
		return fmt.Errorf("decompose stage required for triage decomposition")
	}
	req := b.stageRequest(beadItem, iteration, nil)
	req.Remediation = true
	// Apply routing to decompose stage.
	decomposeTier := routing.TierForPhase("decompose", b.phaseModels, routing.TierMedium)
	if b.router != nil {
		provider, model, _, routeErr := b.router.Select("decompose", decomposeTier)
		if routeErr != nil {
			log.Printf("WARNING: routing failed for decompose tier %s: %v (using default provider)", decomposeTier, routeErr)
		} else if provider != nil {
			req.Provider = provider
			req.Model = model
		}
	} else if req.Model == "" {
		req.Model = routing.ResolveModel(decomposeTier, defaultTierToModel)
	}
	res, err := b.runStage(ctx, b.decompose, req)
	if err != nil {
		return fmt.Errorf("decompose: %w", err)
	}
	artifacts, ok := res.Artifacts.(*stage.DecomposeArtifacts)
	if !ok {
		return fmt.Errorf("decompose stage returned unexpected artifacts type %T", res.Artifacts)
	}
	if len(artifacts.Beads) == 0 {
		return fmt.Errorf("decomposition produced no sub-beads")
	}

	// Check decomposition generation cap on sub-beads
	decompLimit := 0
	if b.decompositionGenerationCap > 0 {
		decompLimit = b.startGeneration + b.decompositionGenerationCap
	}
	subBeadIDs := make([]string, 0, len(artifacts.Beads))
	for _, subBead := range artifacts.Beads {
		if subBead == nil {
			continue
		}
		gen := generation.Current(subBead.Labels)
		if gen > *highestGen {
			*highestGen = gen
		}
		if decompLimit > 0 && gen >= decompLimit {
			b.emitGenerationCapReached(gen)
			return ErrGenerationCapReached
		}
		id := strings.TrimSpace(subBead.ID)
		if id == "" {
			continue
		}
		subBeadIDs = append(subBeadIDs, id)
		b.beadMap[id] = subBead
		b.resolver.Add(id, collectDependencies(subBead))
	}

	// Rewire: any bead that depended on the original now depends on all sub-beads
	b.resolver.ReplaceDependency(beadItem.ID, subBeadIDs)

	// Mark the original bead as completed (it has been replaced by sub-beads)
	b.completed = append(b.completed, beadItem.ID)

	return nil
}

func (b *BeadLoop) emitTriageStarted(beadID, beadTitle string, iteration int) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.TriageStartedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeTriageStarted,
		},
		BeadID:    beadID,
		BeadTitle: beadTitle,
		Iteration: iteration,
	})
}

func (b *BeadLoop) emitModelSelected(beadID, model, provider, tier, reason string) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.ModelSelectedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeModelSelected,
		},
		BeadID:   beadID,
		Model:    model,
		Provider: provider,
		Tier:     tier,
		Reason:   reason,
	})
}

func (b *BeadLoop) emitBuildInvocationStart(beadID, model, provider, tier string, attempt, maxAttempts int) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.BuildInvocationStartEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeBuildInvocationStart,
		},
		BeadID:      beadID,
		Model:       model,
		Provider:    provider,
		Tier:        tier,
		Attempt:     attempt,
		MaxAttempts: maxAttempts,
	})
}

func (b *BeadLoop) emitBuildInvocationComplete(beadID, model, provider string, success bool, duration time.Duration, costUSD float64, inputTokens, outputTokens, promptSize int) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.BuildInvocationCompleteEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeBuildInvocationComplete,
		},
		BeadID:       beadID,
		Model:        model,
		Provider:     provider,
		Success:      success,
		Duration:     duration,
		CostUSD:      costUSD,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		PromptSize:   promptSize,
	})
}

func (b *BeadLoop) emitTriageCompleted(beadID, beadTitle string, iteration int, category, reasoning string) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(event.TriageCompletedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeTriageCompleted,
		},
		BeadID:    beadID,
		BeadTitle: beadTitle,
		Iteration: iteration,
		Category:  category,
		Reasoning: reasoning,
	})
}
