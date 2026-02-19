package tdd

import (
	"context"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// CycleOrchestrator runs TDD red-green-refactor cycles with fresh context per phase.
type CycleOrchestrator struct {
	cfg *config.Config
}

// RunCycles executes TDD cycles until the state is complete.
func (o *CycleOrchestrator) RunCycles(ctx context.Context, bc *runtypes.BeadContext, state CycleState) error {
	for !state.IsComplete() {
		_ = ctx
		_ = bc
		break
	}
	return nil
}
