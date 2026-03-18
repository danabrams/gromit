package specloop

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// SpecLoopConfig configures the SpecLoop runner.
type SpecLoopConfig struct {
	Budget      *Budget
	ReplanStage string
	EventLog    *runstore.EventLog
}

// SpecLoop runs a pipeline of stages in order, supporting cycles and replanning.
type SpecLoop struct {
	// stages holds the stage instances for the pipeline. Stage instances persist
	// across cycles — they are NOT re-created per cycle. This is load-bearing:
	// ReviewStage accumulates priorFindings across cycles for disposition matching
	// (new vs pre-existing). Do not replace stages between cycles.
	stages []Stage
	config SpecLoopConfig
}

// NewSpecLoop creates a new SpecLoop with the given stages and config.
func NewSpecLoop(stages []Stage, cfg SpecLoopConfig) *SpecLoop {
	return &SpecLoop{stages: stages, config: cfg}
}

// Run executes the stage pipeline for up to MaxCycles iterations.
// MaxCycles is derived from the Budget; if no Budget is set, defaults to 1.
func (sl *SpecLoop) Run(ctx context.Context, rs *runstore.RunState) error {
	maxCycles := 1
	if sl.config.Budget != nil {
		maxCycles = sl.config.Budget.MaxCycles()
	}
	for cycle := 0; cycle < maxCycles; cycle++ {
		rs.Cycle = cycle + 1

		// Reset per-cycle gate fields. ReplanContext and ContractsWritten are
		// intentionally preserved — see runstore.ResetForNewCycle for the full list.
		runstore.ResetForNewCycle(rs)

		startIdx := 0
		if cycle > 0 && sl.config.ReplanStage != "" {
			if idx := sl.findStageIndex(sl.config.ReplanStage); idx >= 0 {
				startIdx = idx
			}
		}

		replan := false
		var replanContext *FailureContext
		var replanSource string
		for i := startIdx; i < len(sl.stages); i++ {
			stage := sl.stages[i]

			// Check hard budget between stages
			if sl.config.Budget != nil && sl.config.Budget.HardBudgetExceeded() {
				rs.Status = runstore.StatusBlocked
				rs.TerminalReason = "budget_exceeded"
				rs.BlockerSummary = sl.config.Budget.Reason()
				rs.EndedAt = time.Now()
				sl.emitEvent(runstore.BudgetExceededEvent{
					BaseEvent:       runstore.BaseEvent{Type: "budget_exceeded", Timestamp: time.Now()},
					AccumulatedCost: rs.AccumulatedCost,
				})
				sl.emitTerminal(rs)
				sl.runEvidence(ctx, rs)
				return nil
			}

			action, err := stage.Run(ctx, rs)
			if err != nil {
				rs.Status = runstore.StatusBlocked
				rs.BlockerSummary = err.Error()
				rs.EndedAt = time.Now()
				sl.emitTerminal(rs)
				sl.runEvidence(ctx, rs)
				return nil
			}

			switch action.Kind {
			case Continue:
				// proceed to next stage
			case ReplanFrom:
				replan = true
				replanContext = action.Context
				replanSource = stage.Name()
			case NeedsHuman:
				rs.Status = runstore.StatusNeedsHuman
				if action.Context != nil && len(action.Context.Failures) > 0 {
					rs.TerminalReason = "stage_needs_human"
					rs.BlockerSummary = action.Context.Failures[0]
				}
				rs.EndedAt = time.Now()
				// Only call runAccept if accept hasn't already run as the current
				// stage — avoids double-executing accept when it returns NeedsHuman.
				if stage.Name() != "accept" {
					sl.runAccept(ctx, rs)
				}
				sl.emitTerminal(rs)
				sl.runEvidence(ctx, rs)
				return nil
			case Blocked:
				rs.Status = runstore.StatusBlocked
				rs.EndedAt = time.Now()
				sl.emitTerminal(rs)
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

		// Thread failure context into RunState for PlanStage to read on replan
		if replanContext != nil {
			// Initialize FailureHistory if nil
			if rs.FailureHistory == nil {
				rs.FailureHistory = make(map[string]int)
			}

			// Update failure history with current cycle's failures
			UpdateFailureHistory(rs.FailureHistory, extractFailureKeys(replanContext.Failures))

			// Annotate failures with persistent-failure hints for consecutive cycles
			// that may indicate a bad test specification rather than an implementation bug
			annotated := AnnotateWithPersistentHints(replanContext.Failures, rs.FailureHistory, 2)
			rs.ReplanContext = DeduplicateFailures(annotated)
		}

		// Emit replan_triggered event
		reason := ""
		if replanContext != nil && len(replanContext.Failures) > 0 {
			reason = replanContext.Failures[0]
		}
		sl.emitEvent(runstore.ReplanTriggeredEvent{
			BaseEvent: runstore.BaseEvent{Type: "replan_triggered", Timestamp: time.Now()},
			Reason:    reason,
			Source:    replanSource,
		})
		rs.TotalReplans++

		// Increment cycle in budget AFTER a completed cycle, before the next one
		if sl.config.Budget != nil {
			sl.config.Budget.IncrementCycle()
		}
	}

	// Cycle exhaustion
	if sl.config.Budget != nil && sl.config.Budget.CyclesExhausted() && !rs.IsTerminal() {
		rs.Status = runstore.StatusNeedsHuman
		rs.TerminalReason = "cycles_exhausted"
		if len(rs.ReplanContext) > 0 {
			rs.BlockerSummary = rs.ReplanContext[len(rs.ReplanContext)-1]
		}
		rs.EndedAt = time.Now()
		sl.runAccept(ctx, rs)
		sl.emitTerminal(rs)
		sl.runEvidence(ctx, rs)
	}
	return nil
}

// extractFailureKeys extracts and merges both test and contract failure keys.
func extractFailureKeys(failures []string) []string {
	testKeys := ExtractTestFailureKeys(failures)
	contractKeys := ExtractContractFailureKeys(failures)
	merged := make([]string, 0, len(testKeys)+len(contractKeys))
	merged = append(merged, testKeys...)
	merged = append(merged, contractKeys...)
	return merged
}

// emitEvent appends an event to the log if configured.
func (sl *SpecLoop) emitEvent(ev runstore.TypedEvent) {
	if sl.config.EventLog != nil {
		sl.config.EventLog.Append(ev)
	}
}

// emitTerminal emits a terminal_state event based on the current RunState.
func (sl *SpecLoop) emitTerminal(rs *runstore.RunState) {
	sl.emitEvent(runstore.TerminalStateEvent{
		BaseEvent: runstore.BaseEvent{Type: "terminal_state", Timestamp: time.Now()},
		Status:    rs.Status,
		Reason:    rs.TerminalReason,
	})
}

// runEvidence finds and runs the "evidence" stage if present.
// Errors are recorded on RunState rather than propagated, since evidence
// collection is best-effort when the run is already in a terminal state.
func (sl *SpecLoop) runEvidence(ctx context.Context, rs *runstore.RunState) {
	if es := sl.findStage("evidence"); es != nil {
		if _, err := es.Run(ctx, rs); err != nil {
			rs.BlockerSummary += "; evidence collection failed: " + err.Error()
		}
	}
}

// runAccept finds and runs the "accept" stage if present.
// Errors are recorded on RunState rather than propagated, since acceptance
// evaluation is best-effort when the run is already in a terminal state.
func (sl *SpecLoop) runAccept(ctx context.Context, rs *runstore.RunState) {
	if as := sl.findStage("accept"); as != nil {
		if _, err := as.Run(ctx, rs); err != nil {
			rs.BlockerSummary += "; accept stage failed: " + err.Error()
		}
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
