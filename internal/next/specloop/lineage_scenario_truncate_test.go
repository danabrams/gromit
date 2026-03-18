package specloop

import (
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_LastError_TruncatedForLongOutput(t *testing.T) {
	// Note: Task.LastError has been removed from the Task struct (Issue 4).
	// Error text is now stored exclusively in TaskLineage. The execute stage
	// is responsible for writing error text to the lineage directly.
	// This test now verifies that a lineage entry is created on failure.
	tasks := []runstore.Task{
		{
			TaskID:    "t-001",
			Status:    "failed",
			Objective: "implement feature",
		},
	}
	lineage := make(map[string]runstore.TaskLineageEntry)

	UpdateTaskLineage(lineage, tasks, []string{"t-001"})

	// Assert: lineage entry is created for the failed task
	entry, exists := lineage["t-001"]
	if !exists {
		t.Fatal("expected lineage entry for t-001")
	}
	if entry.ConsecutiveFails != 1 {
		t.Fatalf("expected ConsecutiveFails=1, got %d", entry.ConsecutiveFails)
	}
}
