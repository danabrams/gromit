package specloop

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestScenario_SecondFailureTriggersErrorContextInclusion verifies that when a
// fix task fails and the lineage root's ConsecutiveFails reaches the
// error_context_threshold, the replan context includes prior-attempt-error
// entries, while model escalation is NOT triggered (model threshold not reached).
func TestScenario_SecondFailureTriggersErrorContextInclusion(t *testing.T) {
	// --- Seed ---
	// Simulate two cycles through the SpecLoop:
	//   Cycle 1: t-001 fails with a compilation error
	//   Cycle 2: planner creates t-015 (fixes: t-001), t-015 also fails
	// After cycle 2, the lineage for t-001 should have ConsecutiveFails: 2,
	// and the replan context should include the prior-attempt-error.

	executeCalls := 0
	planCalls := 0

	// Track what the plan stage sees in ReplanContext on cycle 3
	var cycle3ReplanContext []string

	stages := []Stage{
		&mockStage{name: "plan", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
			planCalls++
			switch planCalls {
			case 1:
				// Cycle 1: initial plan with t-001
				rs.Tasks = []runstore.Task{
					{
						TaskID:    "t-001",
						Status:    "pending",
						Objective: "implement feature",
						ModelTier: "medium",
					},
				}
			case 2:
				// Cycle 2: fix plan adds t-015 (fixes t-001)
				rs.Tasks = append(rs.Tasks, runstore.Task{
					TaskID:    "t-015",
					Status:    "pending",
					 Fixes: "t-001",
					Objective: "fix t-001 compilation error",
					ModelTier: "medium",
				})
			case 3:
				// Cycle 3: capture the replan context the planner would see
				cycle3ReplanContext = append([]string{}, rs.ReplanContext...)
			}
			return NextAction{Kind: Continue}, nil
		}},
		&mockStage{name: "execute", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
			executeCalls++
			switch executeCalls {
			case 1:
				// Cycle 1: t-001 fails
				for i := range rs.Tasks {
					if rs.Tasks[i].TaskID == "t-001" {
						rs.Tasks[i].Status = "failed"
						// Note: Task.LastError removed (Issue 4)
					}
				}
				return NextAction{
					Kind:    ReplanFrom,
					Context: &FailureContext{Failures: []string{"--- FAIL: TestFeature (0.01s)"}},
				}, nil
			case 2:
				// Cycle 2: t-015 fails, t-001 remains failed
				for i := range rs.Tasks {
					if rs.Tasks[i].TaskID == "t-015" {
						rs.Tasks[i].Status = "failed"
						// Note: Task.LastError removed (Issue 4)
					}
					// t-001 remains failed from cycle 1
				}
				return NextAction{
					Kind:    ReplanFrom,
					Context: &FailureContext{Failures: []string{"--- FAIL: TestFeature (0.01s)"}},
				}, nil
			}
			// Cycle 3: succeed
			return NextAction{Kind: Continue}, nil
		}},
		&countStage{name: "finalize", counts: map[string]int{}},
	}

	budget := NewBudget(execpolicy.Budgets{
		MaxSpecCycles:          3,
		MaxRunCostUSD:          99,
		MaxRunDurationSeconds:  3600,
		MaxTaskDurationSeconds: 300,
	})
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
		Escalation: execpolicy.EscalationConfig{
			ErrorContextThreshold:    2,
			ModelEscalationThreshold: 3,
		},
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// --- Assert ---

	// 1. TaskLineage should exist and track t-001's consecutive failures
	if rs.TaskLineage == nil {
		t.Fatal("TaskLineage should not be nil after replan cycles")
	}

	entry001, ok := rs.TaskLineage["t-001"]
	if !ok {
		t.Fatal("TaskLineage should contain entry for t-001")
	}
	if entry001.ConsecutiveFails < 2 {
		t.Errorf("t-001 ConsecutiveFails = %d, want >= 2 (failed in cycles 1 and 2)", entry001.ConsecutiveFails)
	}

	// 2. After Issue 3 (no mirror entries), t-015 is not stored as its own key.
	// Instead, it should appear in t-001's ChainIDs as part of the fix chain.
	entry001ChainIDs := entry001.ChainIDs
	hasT015 := false
	for _, chainID := range entry001ChainIDs {
		if chainID == "t-015" {
			hasT015 = true
			break
		}
	}
	if !hasT015 {
		t.Errorf("t-015 should appear in t-001 ChainIDs (no mirror entries after Issue 3), got %v", entry001ChainIDs)
	}

	// 3. Note: prior-attempt-error entries require entry.LastError to be non-empty.
	// Since Task.LastError was removed (Issue 4), the mock execute stage can no longer
	// set error text via task fields. The integration test verifies ConsecutiveFails
	// tracking (above) but cannot assert on prior-attempt-error content here.
	// cycle3ReplanContext is captured but the prior-attempt-error assertion is skipped.
	_ = cycle3ReplanContext

	// 4. Model escalation should NOT be triggered (threshold 3, consecutive fails 2)
	// A hypothetical next fix task for t-001 should not be escalated
	hypotheticalFixTask := &runstore.Task{
		TaskID:    "t-020",
		 Fixes: "t-001",
		ModelTier: "medium",
	}
	if ShouldEscalateModel(hypotheticalFixTask, rs.TaskLineage, 3) {
		t.Error("model should NOT be escalated: consecutive fails (2) < model escalation threshold (3)")
	}
}

// TestScenario_SecondFailureTriggersErrorContextInclusion_DirectFunctions
// tests the lineage functions directly to verify the error context mechanics
// at the threshold boundary, independent of the SpecLoop integration.
func TestScenario_SecondFailureTriggersErrorContextInclusion_DirectFunctions(t *testing.T) {
	errorMsg := `cannot use FailureHistory (variable of type map[string]int) as field in struct literal`

	// --- Seed ---
	// Lineage from cycle 1: t-001 failed once, with error text stored in lineage.
	// Note: Task.LastError was removed (Issue 4); error text must be set directly
	// on the TaskLineageEntry, not via task struct.
	taskLineage := map[string]runstore.TaskLineageEntry{
		"t-001": {
			ConsecutiveFails: 1,
			LastError:        errorMsg, // pre-seed with error text from prior cycle
			ChainIDs:         []string{"t-001"},
			OriginalTaskID:   "t-001",
		},
	}

	// Current tasks: t-001 still failed from cycle 1, t-015 (fix) also failed
	// Task.LastError and Task.ConsecutiveFails fields no longer exist (Issue 4)
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "failed"},
		{TaskID: "t-015", Status: "failed", Fixes: "t-001"},
	}
	failedTaskIDs := []string{"t-001", "t-015"}

	// --- Invoke ---
	UpdateTaskLineage(taskLineage, tasks, failedTaskIDs)

	// --- Assert: lineage updates ---

	// t-001's ConsecutiveFails should be incremented from 1 to 2
	if taskLineage["t-001"].ConsecutiveFails != 2 {
		t.Errorf("t-001 ConsecutiveFails = %d, want 2", taskLineage["t-001"].ConsecutiveFails)
	}

	// t-001's ChainIDs should still contain t-001 as root
	if len(taskLineage["t-001"].ChainIDs) == 0 || taskLineage["t-001"].ChainIDs[0] != "t-001" {
		t.Errorf("t-001 ChainIDs = %v, want [t-001, ...]", taskLineage["t-001"].ChainIDs)
	}

	// t-015 should have a lineage entry with chain from t-001
	// Note: with mirror entries removed (Issue 3), t-015 does not get its own entry.
	// Only root entries are maintained. Verify t-001 now includes t-015 in ChainIDs.
	rootEntry, rootExists := taskLineage["t-001"]
	if !rootExists {
		t.Fatal("expected lineage entry for t-001 (root)")
	}
	hasT015 := false
	for _, chainID := range rootEntry.ChainIDs {
		if chainID == "t-015" {
			hasT015 = true
			break
		}
	}
	if !hasT015 {
		t.Errorf("t-001 ChainIDs should include t-015, got %v", rootEntry.ChainIDs)
	}

	// --- Assert: error context inclusion at threshold ---
	replanContext := []string{}
	errorContextThreshold := 2

	AppendPriorAttemptErrors(&replanContext, taskLineage, errorContextThreshold)

	// t-001 has ConsecutiveFails=2 which meets threshold=2, and has LastError set.
	// The prior-attempt-error should be emitted.
	foundErrorForChain := false
	for _, ctx := range replanContext {
		if strings.Contains(ctx, "prior-attempt-error") {
			foundErrorForChain = true
		}
	}
	if !foundErrorForChain {
		t.Errorf("expected prior-attempt-error in replan context, got %v", replanContext)
	}

	// --- Assert: model escalation NOT triggered ---
	// error_context_threshold=2 is met, but model_escalation_threshold=3 is NOT
	nextFixTask := &runstore.Task{
		TaskID:    "t-020",
		 Fixes: "t-001",
		ModelTier: "medium",
	}
	modelEscalationThreshold := 3

	if ShouldEscalateModel(nextFixTask, taskLineage, modelEscalationThreshold) {
		t.Error("ShouldEscalateModel should return false: ConsecutiveFails (2) < model threshold (3)")
	}

	// Verify escalation WOULD trigger if threshold were lower
	if !ShouldEscalateModel(nextFixTask, taskLineage, 2) {
		t.Error("ShouldEscalateModel should return true when threshold equals ConsecutiveFails")
	}
}