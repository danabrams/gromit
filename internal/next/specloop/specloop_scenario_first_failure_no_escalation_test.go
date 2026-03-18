package specloop

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_FirstFailure_NoEscalation(t *testing.T) {
	// Seed: a run on cycle 1 where task t-001 fails with no fixes field
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{
			TaskID:    "t-001",
			Objective: "Add FailureHistory field to types.go",
			Status:    "pending",
			ModelTier: "medium",
			// No Fixes field — this is an original task
		},
	}

	executeCalls := 0
	planCalls := 0

	stages := []Stage{
		&mockStage{name: "plan", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
			planCalls++
			if planCalls == 2 {
				// Cycle 2: verify replan context does NOT contain prior-attempt-error
				for _, ctx := range rs.ReplanContext {
					if strings.HasPrefix(ctx, "prior-attempt-error:") {
						t.Errorf("replan context should NOT contain prior-attempt-error on first failure, got: %s", ctx)
					}
				}
			}
			return NextAction{Kind: Continue}, nil
		}},
		&mockStage{name: "execute", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
			executeCalls++
			if executeCalls == 1 {
				// Cycle 1: simulate t-001 failing
				for i := range rs.Tasks {
					if rs.Tasks[i].TaskID == "t-001" {
						rs.Tasks[i].Status = "failed"
						// Note: Task.LastError was removed (Issue 4); error text no longer stored on task struct
					}
				}
				return NextAction{
					Kind: ReplanFrom,
					Context: &FailureContext{
						Failures: []string{"undefined: FailureHistory"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			// Cycle 2: succeed
			return NextAction{Kind: Continue}, nil
		}},
		&countStage{name: "finalize", counts: map[string]int{}},
	}

	escalationCfg := execpolicy.EscalationConfig{
		ErrorContextThreshold:    2,
		ModelEscalationThreshold: 3,
	}
	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 2, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
		Escalation:  escalationCfg,
	})

	// Invoke
	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: TaskLineage entry exists for t-001
	entry, ok := rs.TaskLineage["t-001"]
	if !ok {
		t.Fatal("expected TaskLineage entry for t-001")
	}

	// ConsecutiveFails should be 1
	if entry.ConsecutiveFails != 1 {
		t.Errorf("ConsecutiveFails = %d, want 1", entry.ConsecutiveFails)
	}

	// Note: Task.LastError was removed (Issue 4); LastError is only set by external code (not UpdateTaskLineage)

	// ChainIDs should be ["t-001"] (task is its own root, no fixes)
	if len(entry.ChainIDs) != 1 || entry.ChainIDs[0] != "t-001" {
		t.Errorf("ChainIDs = %v, want [t-001]", entry.ChainIDs)
	}

	// ShouldEscalateModel should return false for an original task with no fixes
	fixTask := &runstore.Task{
		TaskID: "t-002",
		Fixes:  "t-001",
	}
	if ShouldEscalateModel(fixTask, rs.TaskLineage, escalationCfg.ModelEscalationThreshold) {
		t.Error("ShouldEscalateModel should return false: ConsecutiveFails(1) < threshold(3)")
	}

	// Original task with no Fixes should also not escalate
	origTask := &runstore.Task{
		TaskID: "t-001",
		Fixes:  "",
	}
	if ShouldEscalateModel(origTask, rs.TaskLineage, escalationCfg.ModelEscalationThreshold) {
		t.Error("ShouldEscalateModel should return false for task with no Fixes")
	}

	// AppendPriorAttemptErrors should NOT add anything (1 < threshold of 2)
	replanCtx := []string{}
	AppendPriorAttemptErrors(&replanCtx, rs.TaskLineage, escalationCfg.ErrorContextThreshold)
	if len(replanCtx) != 0 {
		t.Errorf("expected no prior-attempt-error entries (below threshold), got %d: %v", len(replanCtx), replanCtx)
	}
}
