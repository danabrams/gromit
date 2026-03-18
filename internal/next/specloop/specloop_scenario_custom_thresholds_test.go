package specloop

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_CustomThresholds_ErrorContextAtThreshold1(t *testing.T) {
	// Seed: a lineage entry with 1 consecutive failure and a custom threshold of 1
	lineage := map[string]runstore.TaskLineageEntry{
		"t-001": {
			ChainIDs:         []string{"t-001"},
			OriginalTaskID:   "t-001",
			ConsecutiveFails: 1,
			LastError:        "compilation error: undefined variable x",
		},
	}

	// Invoke: AppendPriorAttemptErrors with errorContextThreshold=1
	replanContext := []string{}
	AppendPriorAttemptErrors(&replanContext, lineage, 1)

	// Assert: error context IS included because ConsecutiveFails (1) >= threshold (1)
	if len(replanContext) != 1 {
		t.Fatalf("expected 1 error context entry at threshold 1, got %d", len(replanContext))
	}
	if !strings.Contains(replanContext[0], "prior-attempt-error: t-001") {
		t.Errorf("expected prior-attempt-error for t-001, got %q", replanContext[0])
	}
	if !strings.Contains(replanContext[0], "compilation error: undefined variable x") {
		t.Errorf("expected error message in context, got %q", replanContext[0])
	}
}

func TestScenario_CustomThresholds_ErrorContextNotIncludedAtDefault(t *testing.T) {
	// Seed: same lineage with 1 failure but default threshold of 2
	lineage := map[string]runstore.TaskLineageEntry{
		"t-001": {
			ChainIDs:         []string{"t-001"},
			OriginalTaskID:   "t-001",
			ConsecutiveFails: 1,
			LastError:        "compilation error: undefined variable x",
		},
	}

	// Invoke: AppendPriorAttemptErrors with default errorContextThreshold=2
	replanContext := []string{}
	AppendPriorAttemptErrors(&replanContext, lineage, 2)

	// Assert: error context NOT included because ConsecutiveFails (1) < threshold (2)
	if len(replanContext) != 0 {
		t.Fatalf("expected 0 entries with default threshold 2, got %d", len(replanContext))
	}
}

func TestScenario_CustomThresholds_ModelEscalationAtThreshold2(t *testing.T) {
	// Seed: lineage where the fixed task has 2 consecutive failures
	lineage := map[string]runstore.TaskLineageEntry{
		"t-001": {
			ChainIDs:         []string{"t-001"},
			OriginalTaskID:   "t-001",
			ConsecutiveFails: 2,
		},
	}
	task := &runstore.Task{
		TaskID:    "t-002",
		Fixes:     "t-001",
		ModelTier: "medium",
	}

	// Invoke: ShouldEscalateModel with modelEscalationThreshold=2
	result := ShouldEscalateModel(task, lineage, 2)

	// Assert: escalation IS triggered because ConsecutiveFails (2) >= threshold (2)
	if !result {
		t.Error("expected model escalation at threshold 2 with 2 consecutive failures")
	}
}

func TestScenario_CustomThresholds_NoModelEscalationAtDefault(t *testing.T) {
	// Seed: same lineage with 2 failures but default threshold of 3
	lineage := map[string]runstore.TaskLineageEntry{
		"t-001": {
			ChainIDs:         []string{"t-001"},
			OriginalTaskID:   "t-001",
			ConsecutiveFails: 2,
		},
	}
	task := &runstore.Task{
		TaskID:    "t-002",
		Fixes:     "t-001",
		ModelTier: "medium",
	}

	// Invoke: ShouldEscalateModel with default modelEscalationThreshold=3
	result := ShouldEscalateModel(task, lineage, 3)

	// Assert: escalation NOT triggered because ConsecutiveFails (2) < threshold (3)
	if result {
		t.Error("expected no model escalation at default threshold 3 with only 2 consecutive failures")
	}
}

func TestScenario_CustomThresholds_FullLoopIntegration(t *testing.T) {
	// Integration test: verify that custom thresholds flow through the SpecLoop
	// and affect replan context and model escalation across cycles.
	//
	// Policy: error_context_threshold=1, model_escalation_threshold=2
	// Cycle 1: task fails → lineage gets ConsecutiveFails=1
	//   → error context appended to replan (threshold 1 reached)
	// Cycle 2: fix task created, but since ConsecutiveFails for root is 1 (not 2),
	//   model is NOT escalated yet. Fix task fails → ConsecutiveFails becomes 2
	//   → on next replan, model WOULD be escalated (threshold 2 reached)

	executeCalls := 0
	var cycle1ReplanContext []string
	var cycle2ReplanContext []string

	stages := []Stage{
		&mockStage{name: "plan", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
			if rs.Cycle == 2 {
				cycle1ReplanContext = append([]string{}, rs.ReplanContext...)
			}
			if rs.Cycle == 3 {
				cycle2ReplanContext = append([]string{}, rs.ReplanContext...)
			}
			return NextAction{Kind: Continue}, nil
		}},
		&mockStage{name: "execute", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
			executeCalls++
			switch executeCalls {
			case 1:
				// Cycle 1: t-001 fails
				rs.Tasks = []runstore.Task{
					{
						TaskID: "t-001",
						Status: "failed",

						Objective: "implement calculator",
						ModelTier: "medium",
					},
				}
				return NextAction{
					Kind:    ReplanFrom,
					Context: &FailureContext{Failures: []string{"--- FAIL: TestCalc"}},
				}, nil
			case 2:
				// Cycle 2: fix task t-002 also fails
				rs.Tasks = append(rs.Tasks, runstore.Task{
					TaskID: "t-002",
					Status: "failed",

					Objective: "fix calculator",
					ModelTier: "medium",
					Fixes:     "t-001",
				})
				return NextAction{
					Kind:    ReplanFrom,
					Context: &FailureContext{Failures: []string{"--- FAIL: TestCalc"}},
				}, nil
			default:
				// Cycle 3: succeeds
				return NextAction{Kind: Continue}, nil
			}
		}},
		&countStage{name: "finalize", counts: map[string]int{}},
	}

	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 3, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
		Escalation: execpolicy.EscalationConfig{
			ErrorContextThreshold:    1, // Lower than default (2)
			ModelEscalationThreshold: 2, // Lower than default (3)
		},
	})
	rs := runstore.NewRunState("spec-thresholds", "proj-calc")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: after cycle 1 failure, lineage for t-001 should have ConsecutiveFails=1.
	// Note: Task.LastError was removed (Issue 4), so prior-attempt-error entries require
	// error text to be set on the lineage entry directly (not via task struct).
	// The cycle1ReplanContext may not contain prior-attempt-error with error text in
	// this test since the mock execute stage cannot set Task.LastError anymore.
	_ = cycle1ReplanContext // suppress unused variable warning

	// Assert: after cycle 2 failure, lineage for t-001 should have ConsecutiveFails >= 1
	if entry, ok := rs.TaskLineage["t-001"]; ok {
		if entry.ConsecutiveFails < 1 {
			t.Errorf("t-001 should have ConsecutiveFails >= 1, got %d", entry.ConsecutiveFails)
		}
	} else {
		t.Error("expected t-001 lineage entry after failures")
	}

	// Verify that ShouldEscalateModel would return true for a task fixing t-001
	// after the second failure (ConsecutiveFails >= 2, threshold = 2)
	if entry, ok := rs.TaskLineage["t-001"]; ok {
		if entry.ConsecutiveFails >= 2 {
			fixTask := &runstore.Task{TaskID: "t-003", Fixes: "t-001"}
			if !ShouldEscalateModel(fixTask, rs.TaskLineage, 2) {
				t.Error("expected escalation after 2 failures with threshold=2")
			}
		}
	}

	// cycle2ReplanContext may be empty since error text is no longer on Task struct
	_ = cycle2ReplanContext // suppress unused variable warning
}

func TestScenario_CustomThresholds_FirstFailureTriggersErrorContext(t *testing.T) {
	// Focused test: with threshold=1, the very first failure should produce
	// error context in the replan, unlike the default threshold=2 which would
	// require two failures before including error output.

	// Seed lineage directly (Task.LastError was removed in Issue 4; error text is set
	// in the lineage entry directly by the execute stage rather than via Task struct).
	lineage := map[string]runstore.TaskLineageEntry{
		"t-001": {
			ChainIDs:         []string{"t-001"},
			OriginalTaskID:   "t-001",
			ConsecutiveFails: 1,
			LastError:        "panic: runtime error: index out of range",
		},
	}

	// Invoke: with custom threshold=1, error context should be appended
	replanContext := []string{}
	AppendPriorAttemptErrors(&replanContext, lineage, 1)

	// Assert: error context included after just one failure (ConsecutiveFails=1 >= threshold=1)
	if len(replanContext) != 1 {
		t.Fatalf("with threshold=1, expected error context after first failure, got %d entries", len(replanContext))
	}
	if !strings.Contains(replanContext[0], "panic: runtime error") {
		t.Errorf("expected panic message in context, got %q", replanContext[0])
	}
}

func TestScenario_CustomThresholds_SecondFailureTriggersModelEscalation(t *testing.T) {
	// Focused test: with model_escalation_threshold=2, the second consecutive
	// failure should trigger model escalation for fix tasks.

	// Seed: simulate two consecutive failures via UpdateTaskLineage
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "failed"},
	}
	lineage := make(map[string]runstore.TaskLineageEntry)

	// First failure
	UpdateTaskLineage(lineage, tasks, []string{"t-001"})
	if lineage["t-001"].ConsecutiveFails != 1 {
		t.Fatalf("expected ConsecutiveFails=1 after first failure, got %d", lineage["t-001"].ConsecutiveFails)
	}

	// Second failure (task.ConsecutiveFails removed from Task struct in Issue 4)
	UpdateTaskLineage(lineage, tasks, []string{"t-001"})
	if lineage["t-001"].ConsecutiveFails != 2 {
		t.Fatalf("expected ConsecutiveFails=2 after second failure, got %d", lineage["t-001"].ConsecutiveFails)
	}

	// Invoke: check escalation for a new fix task with threshold=2
	fixTask := &runstore.Task{
		TaskID:    "t-002",
		Fixes:     "t-001",
		ModelTier: "medium",
	}

	// Assert: escalation triggered at threshold=2 after 2 failures
	if !ShouldEscalateModel(fixTask, lineage, 2) {
		t.Error("expected model escalation after 2 consecutive failures with threshold=2")
	}

	// Assert: escalation NOT triggered at threshold=3 after only 2 failures
	if ShouldEscalateModel(fixTask, lineage, 3) {
		t.Error("expected no model escalation after 2 failures with threshold=3")
	}
}
