package specloop

import (
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_FixTaskSucceeds_LineageResets(t *testing.T) {
	// Seed: A run where TaskLineage["t-001"] has ConsecutiveFails: 3,
	// was escalated to Opus. The planner generated fix task t-035
	// with fixes: "t-028". Task t-035 succeeds.
	//
	// Chain history: t-001 (original, failed) -> t-028 (fix, failed) -> t-035 (fix, succeeded)
	lineage := map[string]runstore.TaskLineageEntry{
		"t-001": {
			ConsecutiveFails: 3,
			LastError:        "undefined: Calculator.Multiply",
			ChainIDs:         []string{"t-001", "t-028"},
		},
		"t-028": {
			ConsecutiveFails: 2,
			LastError:        "test timeout after 30s",
			ChainIDs:         []string{"t-001", "t-028"},
		},
	}

	tasks := []runstore.Task{
		{
			TaskID:    "t-001",
			Status:    "done",
			Objective: "implement calculator multiply",
			Kind:      "original",
			Cycle:     1,
		},
		{
			TaskID:    "t-028",
			Status:    "failed",
			Objective: "fix multiply implementation",
			Fixes:     "t-001",
			Kind:      "fix",
			Cycle:     2,
		},
		{
			TaskID:    "t-035",
			Status:    "done",
			Objective: "fix timeout in multiply test",
			Fixes:     "t-028",
			Kind:      "fix",
			Cycle:     3,
		},
	}

	// Invoke: specloop processes task results via UpdateTaskLineage.
	// t-035 succeeded — no failed task IDs for this cycle.
	failedTaskIDs := []string{}
	UpdateTaskLineage(lineage, tasks, failedTaskIDs)

	// Assert: TaskLineage["t-001"] is updated to ConsecutiveFails: 0, LastError: ""
	entry, exists := lineage["t-001"]
	if !exists {
		t.Fatal("TaskLineage[\"t-001\"] entry must be kept (not deleted)")
	}
	if entry.ConsecutiveFails != 0 {
		t.Errorf("TaskLineage[\"t-001\"].ConsecutiveFails = %d, want 0 (success resets counter)", entry.ConsecutiveFails)
	}
	if entry.LastError != "" {
		t.Errorf("TaskLineage[\"t-001\"].LastError = %q, want empty (success clears error)", entry.LastError)
	}

	// ConsecutiveFails and LastError are no longer on Task — check lineage only.

	// ChainIDs is updated to include t-035 (chain history preserved for debugging)
	foundT035 := false
	for _, id := range entry.ChainIDs {
		if id == "t-035" {
			foundT035 = true
			break
		}
	}
	if !foundT035 {
		t.Errorf("TaskLineage[\"t-001\"].ChainIDs = %v, want to include \"t-035\" (chain history preserved for debugging)", entry.ChainIDs)
	}

	// t-035's own lineage entry should exist and be reset
	t035Entry, t035Exists := lineage["t-035"]
	if !t035Exists {
		t.Fatal("TaskLineage[\"t-035\"] entry must exist after success")
	}
	if t035Entry.ConsecutiveFails != 0 {
		t.Errorf("TaskLineage[\"t-035\"].ConsecutiveFails = %d, want 0", t035Entry.ConsecutiveFails)
	}
	if t035Entry.LastError != "" {
		t.Errorf("TaskLineage[\"t-035\"].LastError = %q, want empty", t035Entry.LastError)
	}

	// t-028's lineage entry should also be reset (it was not re-failed this cycle)
	t028Entry, t028Exists := lineage["t-028"]
	if !t028Exists {
		t.Fatal("TaskLineage[\"t-028\"] entry must be kept")
	}
	if t028Entry.ConsecutiveFails != 0 {
		t.Errorf("TaskLineage[\"t-028\"].ConsecutiveFails = %d, want 0 (not re-failed this cycle)", t028Entry.ConsecutiveFails)
	}
	if t028Entry.LastError != "" {
		t.Errorf("TaskLineage[\"t-028\"].LastError = %q, want empty", t028Entry.LastError)
	}
}
