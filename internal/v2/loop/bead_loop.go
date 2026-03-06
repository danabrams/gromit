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
	"github.com/danabrams/gromit/internal/v2/stage"
	reviewstage "github.com/danabrams/gromit/internal/v2/stage/review"
	"github.com/danabrams/gromit/internal/v2/stage/triage"
)

// GitCommitter abstracts git status and commit operations for the bead loop.
type GitCommitter interface {
	Status(ctx context.Context, worktree string) (string, error)
	Commit(ctx context.Context, worktree, message string) (string, error)
}

// BeadLoopConfig holds the stages required to process each bead.
type BeadLoopConfig struct {
	Gate            stage.Stage
	Build           stage.Stage
	Validate        stage.Stage
	Review          stage.Stage
	Epilogue        stage.Stage
	Triage                   stage.Stage // optional triage stage for failure categorization
	Decompose                stage.Stage // optional decompose stage for bead splitting
	Emitter                  *event.Emitter
	LegacyEmitter            *events.Emitter
	GenerationCap            int // generation cap for review-created beads
	DecompositionGenerationCap int // generation cap for triage-decomposed beads (separate from review)
	StartGeneration          int
	Git                      GitCommitter
}

// BeadLoopResult captures metadata produced by a bead loop execution.
type BeadLoopResult struct {
	OutOfScopeFindings []v2review.Finding
}

// BeadLoop orchestrates the per-bead execution pipeline.
type BeadLoop struct {
	gate               stage.Stage
	build              stage.Stage
	validate           stage.Stage
	review             stage.Stage
	epilogue           stage.Stage
	triage                   stage.Stage
	decompose                stage.Stage
	emitter                  *event.Emitter
	generationCap            int
	decompositionGenerationCap int
	startGeneration          int
	worktree                 string
	outOfScopeFindings       []v2review.Finding
	git                      GitCommitter
	// run-scoped state for in-loop decomposition
	resolver *dep.Resolver
	beadMap  map[string]*bead.Bead
	completed []string
}

var ErrGenerationCapReached = errors.New("generation cap reached")

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
		gate:            config.Gate,
		build:           config.Build,
		validate:        config.Validate,
		review:          config.Review,
		epilogue:        config.Epilogue,
		triage:                   config.Triage,
		decompose:                config.Decompose,
		emitter:                  config.Emitter,
		generationCap:            config.GenerationCap,
		decompositionGenerationCap: config.DecompositionGenerationCap,
		startGeneration:          config.StartGeneration,
		git:                      config.Git,
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
		next, err := b.resolver.Next(b.completed)
		if err != nil {
			return BeadLoopResult{}, fmt.Errorf("resolve bead: %w", err)
		}
		if next == "" {
			break
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
			return BeadLoopResult{}, err
		}
		b.emitBeadCompleted(beadItem, iteration, true, 0)
		b.completed = append(b.completed, next)
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
	stages := []stageEntry{
		{
			stage: b.gate,
			shouldFail: func(res *stage.Result) bool {
				decision := stageDecision(res)
				return decision == stage.DecisionSkip || decision == stage.DecisionBlock || decision == stage.DecisionFail
			},
			failMessage: func(res *stage.Result) string {
				return fmt.Sprintf("%s decided %s", b.gate.Name(), stageDecision(res))
			},
			retryConfig: retryConfigForStage(b.gate),
		},
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

	if err := b.commitBeadWork(ctx, beadItem); err != nil {
		return fmt.Errorf("git commit after bead %s: %w", beadItem.ID, err)
	}

	if err := b.runEpilogue(ctx, beadItem, iteration, nil); err != nil {
		return err
	}
	return nil
}

// commitBeadWork commits any uncommitted changes after the review stage completes.
// This ensures bead work survives crashes. Only commits if there are actual changes.
func (b *BeadLoop) commitBeadWork(ctx context.Context, beadItem *bead.Bead) error {
	if b.git == nil {
		return nil
	}
	worktree := b.worktree
	status, err := b.git.Status(ctx, worktree)
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}
	title := beadItem.Title
	if title == "" {
		title = beadItem.ID
	}
	message := fmt.Sprintf("[gromit] bead %s: %s", beadItem.ID, title)
	_, err = b.git.Commit(ctx, worktree, message)
	return err
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

	retriesRemaining := entry.retryConfig.MaxRetries
	if retriesRemaining < 0 {
		retriesRemaining = 0
	}
	attempt := 1
	priorFailures := []string{}

	for {
		req := b.stageRequest(beadItem, iteration, stageRetryContext(attempt, priorFailures))
		b.emitStageStarted(stageName, beadItem.ID, iteration)
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

		b.emitStageCompleted(stageName, beadItem.ID, iteration, !failed, duration)
		if failed {
			b.emitStageFailed(stageName, beadItem.ID, iteration, reason)
		} else {
			if entry.stage == b.review {
				b.collectOutOfScopeFindings(res)
				if b.handleGenerationCapFromReview(res, highestGen, generationLimit) {
					return ErrGenerationCapReached
				}
			}
			return nil
		}

		priorFailures = append(priorFailures, reason)

		// When the build stage fails and triage is configured, run triage
		// to decide how to proceed instead of falling through to the
		// normal retry/fail path.
		if entry.stage == b.build && b.triage != nil {
			triageResult, triageErr := b.runTriage(ctx, beadItem, iteration, reason)
			if triageErr != nil {
				return fmt.Errorf("triage: %w", triageErr)
			}
			switch triageResult.Category {
			case triage.CategoryDecompose:
				return b.decomposeAndRunSubBeads(ctx, beadItem, iteration, highestGen)
			case triage.CategoryRetry:
				// Don't count against retry limit, just continue the loop
				attempt++
				continue
			case triage.CategoryUnclearSpec:
				return fmt.Errorf("triage: spec unclear for bead %s: %s", beadItem.ID, triageResult.Reasoning)
			case triage.CategoryUnsafe:
				return fmt.Errorf("triage: unsafe operation for bead %s: %s", beadItem.ID, triageResult.Reasoning)
			}
		}

		if retriesRemaining <= 0 {
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
	ctx := &stage.RetryContext{Attempt: attempt}
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
	return fmt.Errorf("bead %s failed: %s", beadItem.ID, reason)
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
	b.emitter.Emit(event.BeadCompletedEvent{
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
	})
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
	b.emitTriageStarted(beadItem.ID, beadItem.Title, iteration)
	res, err := b.runStage(ctx, b.triage, req)
	if err != nil {
		return nil, err
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
