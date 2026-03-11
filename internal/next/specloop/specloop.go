package specloop

import (
	"context"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// SpecLoopConfig configures the SpecLoop runner.
type SpecLoopConfig struct {
	MaxCycles   int
	Budget      *Budget
	ReplanStage string
}

// SpecLoop runs a pipeline of stages in order, supporting cycles and replanning.
type SpecLoop struct {
	stages []Stage
	config SpecLoopConfig
}

// NewSpecLoop creates a new SpecLoop with the given stages and config.
func NewSpecLoop(stages []Stage, cfg SpecLoopConfig) *SpecLoop {
	return &SpecLoop{stages: stages, config: cfg}
}

// Run executes the stage pipeline for up to MaxCycles iterations.
func (sl *SpecLoop) Run(ctx context.Context, rs *runstore.RunState) error {
	for cycle := 0; cycle < sl.config.MaxCycles; cycle++ {
		if sl.config.Budget != nil {
			sl.config.Budget.IncrementCycle()
		}
		rs.Cycle = cycle + 1

		startIdx := 0
		if cycle > 0 && sl.config.ReplanStage != "" {
			if idx := sl.findStageIndex(sl.config.ReplanStage); idx >= 0 {
				startIdx = idx
			}
		}

		replan := false
		for i := startIdx; i < len(sl.stages); i++ {
			stage := sl.stages[i]

			// Check hard budget between stages
			if sl.config.Budget != nil && sl.config.Budget.HardBudgetExceeded() {
				rs.Status = runstore.StatusBlocked
				rs.TerminalReason = "budget_exceeded"
				rs.BlockerSummary = sl.config.Budget.Reason()
				sl.runEvidence(ctx, rs)
				return nil
			}

			action, err := stage.Run(ctx, rs)
			if err != nil {
				rs.Status = runstore.StatusBlocked
				rs.BlockerSummary = err.Error()
				sl.runEvidence(ctx, rs)
				return nil
			}

			switch action.Kind {
			case Continue:
				// proceed to next stage
			case ReplanFrom:
				replan = true
			case NeedsHuman:
				rs.Status = runstore.StatusNeedsHuman
				sl.runEvidence(ctx, rs)
				return nil
			case Blocked:
				rs.Status = runstore.StatusBlocked
				sl.runEvidence(ctx, rs)
				return nil
			}

			if replan {
				break
			}
		}

		if rs.IsTerminal() {
			return nil
		}

		// If no replan was requested, the pipeline completed successfully.
		if !replan {
			return nil
		}
	}

	// Cycle exhaustion
	if sl.config.Budget != nil && sl.config.Budget.CyclesExhausted() && !rs.IsTerminal() {
		rs.Status = runstore.StatusNeedsHuman
		rs.TerminalReason = "cycles_exhausted"
		sl.runEvidence(ctx, rs)
	}
	return nil
}

// runEvidence finds and runs the "evidence" stage if present.
func (sl *SpecLoop) runEvidence(ctx context.Context, rs *runstore.RunState) {
	if es := sl.findStage("evidence"); es != nil {
		es.Run(ctx, rs)
	}
}

// findStage returns the stage with the given name, or nil.
func (sl *SpecLoop) findStage(name string) Stage {
	for _, s := range sl.stages {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

// findStageIndex returns the index of the stage with the given name, or -1.
func (sl *SpecLoop) findStageIndex(name string) int {
	for i, s := range sl.stages {
		if s.Name() == name {
			return i
		}
	}
	return -1
}
