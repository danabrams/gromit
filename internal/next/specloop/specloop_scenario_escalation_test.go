package specloop

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ThirdFailureTriggersModelEscalation(t *testing.T) {
	// Scenario: Third failure in a task lineage triggers model escalation
	//
	// Given: TaskLineage["t-001"] has ConsecutiveFails: 2, ChainIDs: ["t-001", "t-015"].
	//        Fix task t-028 (fixes: "t-015") has just failed.
	//        Execution policy: model_escalation_threshold: 3.
	//
	// When: The specloop processes task results and triggers a replan.
	//
	// Then: TaskLineage["t-001"] updated to ConsecutiveFails: 3,
	//       TaskLineage["t-028"] has ChainIDs: ["t-001", "t-015", "t-028"],
	//       Replan context includes prior error output,
	//       Next fix task for this lineage is executed with Opus tier.

	// ─── Seed ───────────────────────────────────────────────────
	// Note: Task.LastError and Task.ConsecutiveFails removed (Issue 4)
	tasks := []runstore.Task{
		{
			TaskID:    "t-001",
			Status:    "failed",
			Objective: "implement handler",
		},
		{
			TaskID:    "t-015",
			Status:    "failed",
			Objective: "fix t-001: correct handler logic",
			Fixes:     "t-001",
		},
		{
			TaskID:    "t-028",
			Status:    "failed",
			Objective: "fix t-015: add nil check",
			Fixes:     "t-015",
		},
	}

	taskLineage := map[string]runstore.TaskLineageEntry{
		"t-001": {
			ConsecutiveFails: 2,
			ChainIDs:         []string{"t-001"},
			LastError:        "TestHandler: expected 200 got 500",
			OriginalTaskID:   "t-001",
		},
		"t-015": {
			ConsecutiveFails: 1,
			ChainIDs:         []string{"t-001", "t-015"},
			LastError:        "TestHandler: nil pointer dereference",
			OriginalTaskID:   "t-001", // mirror entry — root is t-001
		},
	}

	// All three tasks remain failed (t-001 and t-015 from prior cycles, t-028 newly failed)
	failedTaskIDs := []string{"t-001", "t-015", "t-028"}

	// ─── Invoke: UpdateTaskLineage (simulates specloop replan handler) ──
	UpdateTaskLineage(taskLineage, tasks, failedTaskIDs)

	// ─── Assert: TaskLineage["t-001"] updated to ConsecutiveFails: 3 ────
	t001Entry, exists001 := taskLineage["t-001"]
	if !exists001 {
		t.Fatal("expected lineage entry for t-001")
	}
	if t001Entry.ConsecutiveFails != 3 {
		t.Errorf("t-001 ConsecutiveFails = %d, want 3", t001Entry.ConsecutiveFails)
	}
	// task.ConsecutiveFails no longer exists on Task struct (Issue 4)

	// ─── Assert: t-028 is in t-001's ChainIDs ──────────────────
	// With mirror entries removed (Issue 3), only the root entry is maintained.
	// t-028 (fix for t-015, which resolves to root t-001) should be in t-001's ChainIDs.
	hasT028 := false
	for _, chainID := range t001Entry.ChainIDs {
		if chainID == "t-028" {
			hasT028 = true
			break
		}
	}
	if !hasT028 {
		t.Errorf("t-028 not found in t-001 ChainIDs: %v", t001Entry.ChainIDs)
	}

	// ─── Assert: t-001 LastError is preserved from pre-seed (UpdateTaskLineage
	// does not overwrite LastError since Task.LastError was removed in Issue 4) ──
	// The pre-seeded LastError remains (no overwrite from task struct).

	// ─── Invoke: AppendPriorAttemptErrors (replan context enrichment) ──
	replanContext := []string{"--- FAIL: TestHandler (0.03s)"}
	AppendPriorAttemptErrors(&replanContext, taskLineage, 2) // error_context_threshold: 2

	// ─── Assert: Replan context includes prior error output ─────
	priorErrors := 0
	for _, ctx := range replanContext {
		if strings.HasPrefix(ctx, "prior-attempt-error:") {
			priorErrors++
		}
	}
	if priorErrors == 0 {
		t.Error("replan context should include prior-attempt-error entries for lineages with ConsecutiveFails >= 2")
	}
	// Original failure preserved at start
	if replanContext[0] != "--- FAIL: TestHandler (0.03s)" {
		t.Error("original failure should remain at start of replan context")
	}

	// ─── Invoke: ShouldEscalateModel (next cycle's execute stage) ──
	// Planner creates t-035 fixing t-001 (the root of the failing lineage)
	nextFixTask := &runstore.Task{
		TaskID:    "t-035",
		Objective: "fix t-001: rewrite handler from scratch",
		Fixes:     "t-001",
		ModelTier: "medium",
	}

	// ─── Assert: Model escalation triggered (ConsecutiveFails=3 >= threshold=3) ──
	if !ShouldEscalateModel(nextFixTask, taskLineage, 3) {
		t.Error("ShouldEscalateModel should return true: t-001 ConsecutiveFails=3 meets threshold=3")
	}

	// Verify no escalation when threshold is higher than accumulated failures
	if ShouldEscalateModel(nextFixTask, taskLineage, 4) {
		t.Error("ShouldEscalateModel should return false when threshold=4 > ConsecutiveFails=3")
	}
}
