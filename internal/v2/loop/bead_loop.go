package loop

import (
	"context"
	"errors"
	"fmt"
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
)

// BeadLoopConfig holds the stages required to process each bead.
type BeadLoopConfig struct {
	Gate            stage.Stage
	Build           stage.Stage
	Validate        stage.Stage
	Review          stage.Stage
	Epilogue        stage.Stage
	Emitter         *event.Emitter
	LegacyEmitter   *events.Emitter
	GenerationCap   int
	StartGeneration int
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
	emitter            *event.Emitter
	generationCap      int
	startGeneration    int
	worktree           string
	outOfScopeFindings []v2review.Finding
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
		emitter:         config.Emitter,
		generationCap:   config.GenerationCap,
		startGeneration: config.StartGeneration,
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

	resolver := dep.NewResolver()
	beadMap := make(map[string]*bead.Bead, len(beads))

	for _, beadItem := range beads {
		if beadItem == nil {
			continue
		}
		id := strings.TrimSpace(beadItem.ID)
		if id == "" {
			continue
		}
		beadMap[id] = beadItem
		resolver.Add(id, collectDependencies(beadItem))
	}

	var completed []string
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
		next, err := resolver.Next(completed)
		if err != nil {
			return BeadLoopResult{}, fmt.Errorf("resolve bead: %w", err)
		}
		if next == "" {
			break
		}
		beadItem, ok := beadMap[next]
		if !ok {
			return BeadLoopResult{}, fmt.Errorf("bead %q missing from input list", next)
		}
		b.emitBeadStarted(beadItem, iteration)
		if err := b.processBead(ctx, beadItem, iteration, &highestGeneration, generationLimit); err != nil {
			return BeadLoopResult{}, err
		}
		b.emitBeadCompleted(beadItem, iteration, true, 0)
		completed = append(completed, next)
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

func (b *BeadLoop) processBead(ctx context.Context, beadItem *bead.Bead, iteration int, highestGen *int, generationLimit int) error {
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
		if err := b.runStageEntry(ctx, beadItem, iteration, stages, nameToIndex, idx, highestGen, generationLimit); err != nil {
			return err
		}
	}

	if err := b.runEpilogue(ctx, beadItem, iteration, nil); err != nil {
		return err
	}
	return nil
}

func (b *BeadLoop) runStageEntry(ctx context.Context, beadItem *bead.Bead, iteration int, entries []stageEntry, nameIndex map[string]int, idx int, highestGen *int, generationLimit int) error {
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

		if retriesRemaining <= 0 {
			return b.failWithReason(ctx, beadItem, iteration, reason, stageRetryContext(attempt, priorFailures))
		}

		retriesRemaining--
		attempt++
		b.emitStageRetrying(stageName, beadItem.ID, attempt, reason)

		if err := b.runRetryWithStages(ctx, beadItem, iteration, entries, nameIndex, entry.retryConfig.RetryWith, idx, highestGen, generationLimit); err != nil {
			return err
		}
	}
}

func (b *BeadLoop) runRetryWithStages(ctx context.Context, beadItem *bead.Bead, iteration int, entries []stageEntry, nameIndex map[string]int, retryWith []string, currentIdx int, highestGen *int, generationLimit int) error {
	for _, retryName := range retryWith {
		idx, ok := nameIndex[retryName]
		if !ok || idx == currentIdx || idx >= currentIdx {
			continue
		}
		if err := b.runStageEntry(ctx, beadItem, iteration, entries, nameIndex, idx, highestGen, generationLimit); err != nil {
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

func (b *BeadLoop) stageRequest(beadItem *bead.Bead, iteration int, retryCtx *stage.RetryContext) stage.Request {
	labels := copyLabels(beadItem.Labels)
	return stage.Request{
		Bead:         stage.BeadInfo{ID: beadItem.ID, Labels: labels},
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
