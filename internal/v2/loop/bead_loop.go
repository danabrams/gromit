package loop

import (
	"context"
	"errors"
	"fmt"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/stage"
)

var ErrMaxRetriesExceeded = errors.New("max retries exceeded")

const epilogueStageName = "epilogue"

type BeadLoopResult struct {
	StartGeneration int
	CapHit          bool
}

type StageSpec struct {
	Stage stage.Stage
	Retry stage.RetryConfig
}

type BeadLoop struct {
	stages              []StageSpec
	stageIndex          map[string]int
	GenerationCap       int
	startGeneration     int
	startGenInitialized bool
	capReached          bool
	Emitter             *events.Emitter
}

func NewBeadLoop(stages []StageSpec) (*BeadLoop, error) {
	if len(stages) == 0 {
		return nil, fmt.Errorf("at least one stage required")
	}
	index := make(map[string]int)
	for idx, spec := range stages {
		name := spec.Stage.Name()
		if name == "" {
			return nil, fmt.Errorf("stage at index %d has empty name", idx)
		}
		if _, exists := index[name]; exists {
			return nil, fmt.Errorf("duplicate stage name %q", name)
		}
		index[name] = idx
	}
	return &BeadLoop{stages: stages, stageIndex: index, GenerationCap: 3}, nil
}

func (b *BeadLoop) Run(ctx context.Context, req stage.Request) (*BeadLoopResult, error) {
	state := newLoopState()
	currentGen := generation.Current(req.Bead.Labels)

	// Initialize startGeneration on first call
	if !b.startGenInitialized {
		b.startGeneration = currentGen
		b.startGenInitialized = true
	}

	result := &BeadLoopResult{
		StartGeneration: b.startGeneration,
	}

	// Check if generation cap is reached and persist the flag once triggered
	capThreshold := b.startGeneration + b.GenerationCap
	if currentGen >= capThreshold && !b.capReached {
		b.capReached = true
		b.emitGenerationCapReached()
	}

	if b.capReached {
		result.CapHit = true
	}

	for _, spec := range b.stages {
		stageName := spec.Stage.Name()
		if b.capReached && shouldSkipStageWhenCapReached(stageName) {
			continue
		}
		if err := b.runStage(ctx, req, spec, state, req.RetryContext); err != nil {
			return result, err
		}
	}
	return result, nil
}

type loopState struct {
	retryCounts    map[string]int
	failureHistory []string
}

func newLoopState() *loopState {
	return &loopState{
		retryCounts: make(map[string]int),
	}
}

func (b *BeadLoop) runStage(ctx context.Context, baseReq stage.Request, spec StageSpec, state *loopState, retryCtx *stage.RetryContext) error {
	reqCopy := baseReq
	reqCopy.RetryContext = retryCtx

	res, err := spec.Stage.Run(ctx, &reqCopy)
	if err != nil {
		return b.handleFailure(ctx, baseReq, spec, state, err)
	}
	if res != nil && res.Decision == stage.DecisionFail {
		return b.handleFailure(ctx, baseReq, spec, state, fmt.Errorf("stage %s returned fail", spec.Stage.Name()))
	}
	return nil
}

func (b *BeadLoop) handleFailure(ctx context.Context, baseReq stage.Request, spec StageSpec, state *loopState, failure error) error {
	stageName := spec.Stage.Name()

	retries := state.retryCounts[stageName] + 1
	summary := fmt.Sprintf("%s failed: %v", stageName, failure)
	state.failureHistory = append(state.failureHistory, summary)

	retryCtx := newRetryContext(state.failureHistory, retries)

	if retries > spec.Retry.MaxRetries {
		if stageName != epilogueStageName {
			if err := b.runEpilogueFailurePath(ctx, baseReq, state, retryCtx); err != nil {
				return fmt.Errorf("%s: %w (epilogue failure: %v)", stageName, ErrMaxRetriesExceeded, err)
			}
		}
		return fmt.Errorf("%s: %w", stageName, ErrMaxRetriesExceeded)
	}

	state.retryCounts[stageName] = retries

	b.emitStageRetrying(stageName, retries, failure.Error())

	for _, rerun := range spec.Retry.RetryWith {
		rerunIdx, ok := b.stageIndex[rerun]
		if !ok {
			return fmt.Errorf("retry stage %s not found", rerun)
		}
		if err := b.runStage(ctx, baseReq, b.stages[rerunIdx], state, retryCtx); err != nil {
			return err
		}
	}

	return b.runStage(ctx, baseReq, spec, state, retryCtx)
}

func (b *BeadLoop) runEpilogueFailurePath(ctx context.Context, baseReq stage.Request, state *loopState, retryCtx *stage.RetryContext) error {
	epilogueIdx, found := b.stageIndex[epilogueStageName]
	if !found {
		return nil
	}

	epilogueSpec := b.stages[epilogueIdx]
	return b.runStage(ctx, baseReq, epilogueSpec, state, retryCtx)
}

func newRetryContext(history []string, attempt int) *stage.RetryContext {
	return &stage.RetryContext{
		Attempt:       attempt,
		PriorFailures: append([]string(nil), history...),
	}
}

func (b *BeadLoop) emitGenerationCapReached() {
	if b.Emitter != nil {
		b.Emitter.Emit(&events.GenerationCapReachedEvent{})
	}
}

func (b *BeadLoop) emitStageRetrying(stageName string, attempt int, reason string) {
	if b.Emitter != nil {
		b.Emitter.Emit(&events.StageRetryingEvent{
			StageName: stageName,
			Attempt:   attempt,
			Reason:    reason,
		})
	}
}

func shouldSkipStageWhenCapReached(name string) bool {
	switch name {
	case "decompose", "review":
		return true
	default:
		return false
	}
}
