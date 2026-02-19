package runner

import (
	"context"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

type cycleOrchestrator struct {
	runner    *Runner
	executeFn func(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, executeWithRetry func() bool) *IterationResult
}

func (o *cycleOrchestrator) Execute(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, executeWithRetry func() bool) *IterationResult {
	if bc == nil {
		return &IterationResult{}
	}
	if o != nil && o.executeFn != nil {
		return o.executeFn(ctx, bc, atddActive, executeWithRetry)
	}
	if o.runner == nil {
		return bc.Result
	}
	return o.runner.executeBuildAndMethodologyLoop(ctx, bc, atddActive, true, executeWithRetry)
}
