package loop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/queue"
	"github.com/danabrams/gromit/internal/v2/dep"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/stage"
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

// BeadLoop orchestrates the per-bead execution pipeline.
type BeadLoop struct {
	gate              stage.Stage
	build             stage.Stage
	validate          stage.Stage
	review            stage.Stage
	epilogue          stage.Stage
	emitter           *event.Emitter
	generationCap     int
	startGeneration   int
}

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

// Run processes the provided beads through the stage pipeline.
func (b *BeadLoop) Run(ctx context.Context, beads []*bead.Bead, stopCh <-chan struct{}) error {
	// Check generation cap before processing any beads
	if b.generationCap > 0 {
		highestGen := b.findHighestGeneration(beads)
		if highestGen >= b.startGeneration+b.generationCap-1 {
			b.emitGenerationCapReached(highestGen)
			return fmt.Errorf("generation cap reached")
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
		if stopCh != nil {
			select {
			case <-stopCh:
				break runLoop
			default:
			}
		}
		next, err := resolver.Next(completed)
		if err != nil {
			return fmt.Errorf("resolve bead: %w", err)
		}
		if next == "" {
			break
		}
		beadItem, ok := beadMap[next]
		if !ok {
			return fmt.Errorf("bead %q missing from input list", next)
		}
		b.emitBeadStarted(beadItem, iteration)
		if err := b.processBead(ctx, beadItem, iteration); err != nil {
			return err
		}
		b.emitBeadCompleted(beadItem, iteration, true, 0)
		completed = append(completed, next)
		iteration++
	}
	return nil
}

func (b *BeadLoop) processBead(ctx context.Context, beadItem *bead.Bead, iteration int) error {
	stages := []struct {
		stage       stage.Stage
		shouldFail  func(*stage.Result) bool
		failMessage func(*stage.Result) string
	}{
		{
			stage: b.gate,
			shouldFail: func(res *stage.Result) bool {
				decision := stageDecision(res)
				return decision == stage.DecisionSkip || decision == stage.DecisionBlock || decision == stage.DecisionFail
			},
			failMessage: func(res *stage.Result) string {
				return fmt.Sprintf("%s decided %s", b.gate.Name(), stageDecision(res))
			},
		},
		{
			stage: b.build,
			shouldFail: func(res *stage.Result) bool {
				return stageDecision(res) == stage.DecisionFail
			},
			failMessage: func(res *stage.Result) string {
				return fmt.Sprintf("%s returned %s", b.build.Name(), stageDecision(res))
			},
		},
		{
			stage: b.validate,
			shouldFail: func(res *stage.Result) bool {
				return stageDecision(res) == stage.DecisionFail
			},
			failMessage: func(res *stage.Result) string {
				return fmt.Sprintf("%s returned %s", b.validate.Name(), stageDecision(res))
			},
		},
		{
			stage: b.review,
			shouldFail: func(res *stage.Result) bool {
				return stageDecision(res) != stage.DecisionProceed
			},
			failMessage: func(res *stage.Result) string {
				return fmt.Sprintf("%s returned %s", b.review.Name(), stageDecision(res))
			},
		},
	}

	for _, entry := range stages {
		stageName := stageName(entry.stage)
		req := b.stageRequest(beadItem, iteration, nil)
		b.emitStageStarted(stageName, beadItem.ID, iteration)
		start := time.Now()
		res, err := b.runStage(ctx, entry.stage, req)
		duration := time.Since(start)

		if err != nil {
			b.emitStageCompleted(stageName, beadItem.ID, iteration, false, duration)
			b.emitStageFailed(stageName, beadItem.ID, iteration, err.Error())
			return b.failWithReason(ctx, beadItem, iteration, err.Error())
		}
		if entry.shouldFail != nil && entry.shouldFail(res) {
			reason := entry.failMessage(res)
			b.emitStageCompleted(stageName, beadItem.ID, iteration, false, duration)
			b.emitStageFailed(stageName, beadItem.ID, iteration, reason)
			return b.failWithReason(ctx, beadItem, iteration, reason)
		}

		b.emitStageCompleted(stageName, beadItem.ID, iteration, true, duration)
	}

	if err := b.runEpilogue(ctx, beadItem, iteration, nil); err != nil {
		return err
	}
	return nil
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

func (b *BeadLoop) failWithReason(ctx context.Context, beadItem *bead.Bead, iteration int, reason string) error {
	retryCtx := &stage.RetryContext{
		Attempt:       1,
		PriorFailures: []string{reason},
	}
	if err := b.runEpilogue(ctx, beadItem, iteration, retryCtx); err != nil {
		return fmt.Errorf("epilogue failure: %w", err)
	}
	b.emitBeadCompleted(beadItem, iteration, false, 1)
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

func stageName(st stage.Stage) string {
	if st == nil {
		return ""
	}
	return st.Name()
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
