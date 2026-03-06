package loop

import (
	"context"
	"errors"
	"fmt"

	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/stage"
)

var ErrMaxRetriesExceeded = errors.New("max retries exceeded")

type BeadLoopResult struct {
	StartGeneration int
	CapHit          bool
}

type StageSpec struct {
	Stage stage.Stage
	Retry stage.RetryConfig
}

type BeadLoop struct {
	stages            []StageSpec
	stageIndex        map[string]int
	GenerationCap     int
	startGeneration   int
	startGenInitialized bool
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

	// Check if generation cap is reached
	capThreshold := b.startGeneration + b.GenerationCap
	if currentGen >= capThreshold {
		result.CapHit = true
		return result, nil
	}

	for _, spec := range b.stages {
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
	if retries > spec.Retry.MaxRetries {
		return fmt.Errorf("%s: %w", stageName, ErrMaxRetriesExceeded)
	}
	state.retryCounts[stageName] = retries

	summary := fmt.Sprintf("%s failed: %v", stageName, failure)
	state.failureHistory = append(state.failureHistory, summary)

	retryCtx := newRetryContext(state.failureHistory, retries)

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

func newRetryContext(history []string, attempt int) *stage.RetryContext {
	return &stage.RetryContext{
		Attempt:       attempt,
		PriorFailures: append([]string(nil), history...),
	}
}
