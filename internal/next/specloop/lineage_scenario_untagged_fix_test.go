package specloop

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_FixTaskWithoutFixesField_TreatedAsNewLineageRoot(t *testing.T) {
	// Seed: a fix task t-020 on cycle 2 with kind "fix" but no fixes field,
	// plus a prior failed task t-010 from cycle 1 with existing lineage.
	lineage := map[string]runstore.TaskLineageEntry{
		"t-010": {
			ChainIDs:         []string{"t-010"},
			ConsecutiveFails: 3,
			LastError:        "compilation error in handler.go",
		},
	}

	tasks := []runstore.Task{
		{
			TaskID: "t-010",
			Status: "failed",
			Kind:   "original",
			Cycle:  1,
		},
		{
			TaskID: "t-020",
			Status: "failed",
			Kind:   "fix",
			Cycle:  2,
			Fixes:  "", // planner didn't tag it — empty fixes
		},
	}

	// Invoke: process task results through UpdateTaskLineage with t-020 as failed
	UpdateTaskLineage(lineage, tasks, []string{"t-020"})

	// Assert: t-020 gets its own lineage entry as a new root
	entry, exists := lineage["t-020"]
	if !exists {
		t.Fatal("expected TaskLineage entry for t-020 to be created")
	}
	if len(entry.ChainIDs) != 1 || entry.ChainIDs[0] != "t-020" {
		t.Errorf("t-020 should be its own lineage root, got ChainIDs=%v", entry.ChainIDs)
	}
	if entry.ConsecutiveFails != 1 {
		t.Errorf("ConsecutiveFails = %d, want 1", entry.ConsecutiveFails)
	}

	// Assert: t-010's lineage is unchanged (no cross-contamination)
	priorEntry := lineage["t-010"]
	if priorEntry.ConsecutiveFails != 3 {
		t.Errorf("t-010 ConsecutiveFails should remain 3, got %d", priorEntry.ConsecutiveFails)
	}
}

func TestScenario_FixTaskWithoutFixesField_NoEscalation(t *testing.T) {
	// Seed: lineage has a heavily-failed task t-010, but t-020 is an untagged
	// fix task (no fixes field) so it should NOT inherit escalation.
	lineage := map[string]runstore.TaskLineageEntry{
		"t-010": {
			ChainIDs:         []string{"t-010"},
			ConsecutiveFails: 5,
			LastError:        "persistent compilation error",
		},
	}

	task := &runstore.Task{
		TaskID:    "t-020",
		Kind:      "fix",
		Cycle:     2,
		Fixes:     "", // untagged — planner didn't include lineage
		ModelTier: "medium",
	}

	// Invoke: check if model escalation would apply
	threshold := 3
	shouldEscalate := ShouldEscalateModel(task, lineage, threshold)

	// Assert: no escalation because Fixes is empty
	if shouldEscalate {
		t.Error("untagged fix task should NOT trigger model escalation")
	}
}

func TestScenario_FixTaskWithoutFixesField_NoPriorErrorContext(t *testing.T) {
	// Seed: lineage has a heavily-failed task, but the untagged fix task
	// creates its own root, so prior errors don't bleed into its context.
	lineage := map[string]runstore.TaskLineageEntry{
		"t-010": {
			ChainIDs:         []string{"t-010"},
			ConsecutiveFails: 5,
			LastError:        "persistent error from t-010",
		},
	}

	// Simulate: t-020 fails without fixes field, gets its own lineage root
	tasks := []runstore.Task{
		{TaskID: "t-020", Kind: "fix", Cycle: 2, Fixes: ""},
	}
	UpdateTaskLineage(lineage, tasks, []string{"t-020"})

	// Invoke: append prior attempt errors with threshold=2
	replanContext := []string{}
	AppendPriorAttemptErrors(&replanContext, lineage, 2)

	// Assert: replan context includes t-010's errors (meets threshold=5>=2)
	// but t-020's entry has ConsecutiveFails=1, below threshold=2, so excluded.
	for _, ctx := range replanContext {
		if strings.Contains(ctx, "t-020") {
			t.Errorf("untagged fix task t-020 should NOT appear in prior error context (only 1 fail, threshold 2), got: %s", ctx)
		}
	}
}

func TestScenario_FixTaskWithoutFixesField_GracefulDegradation(t *testing.T) {
	// Seed: two fix tasks — one tagged (t-021 fixes t-010), one untagged (t-020).
	// The tagged one should get escalation benefits; the untagged one should not.
	lineage := map[string]runstore.TaskLineageEntry{
		"t-010": {
			ChainIDs:         []string{"t-010"},
			ConsecutiveFails: 4,
			LastError:        "persistent compilation error",
		},
	}

	untaggedTask := &runstore.Task{
		TaskID:    "t-020",
		Kind:      "fix",
		Cycle:     2,
		Fixes:     "", // untagged
		ModelTier: "medium",
	}

	taggedTask := &runstore.Task{
		TaskID:    "t-021",
		Kind:      "fix",
		Cycle:     2,
		Fixes:     "t-010", // properly tagged
		ModelTier: "medium",
	}

	threshold := 3

	// Assert: tagged task gets escalation, untagged does not
	if ShouldEscalateModel(untaggedTask, lineage, threshold) {
		t.Error("untagged fix task should NOT escalate")
	}
	if !ShouldEscalateModel(taggedTask, lineage, threshold) {
		t.Error("tagged fix task SHOULD escalate (t-010 has 4 consecutive fails >= threshold 3)")
	}
}