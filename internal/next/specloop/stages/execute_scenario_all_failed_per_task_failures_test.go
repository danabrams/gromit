package stages

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_Execute_AllFailed_UsesPerTaskFailures_NotGenericFallback(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "first task"},
		{TaskID: "t-002", Status: "pending", Objective: "second task"},
	}
	mustSaveRunState(t, store, rs)

	seeded, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("store.Get(%s): %v", rs.RunID, err)
	}

	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{
				TaskID:   "t-001",
				Status:   "failed",
				Failures: []string{"[suspect-proof-check] grep failed for --flag-a"},
			},
			{
				TaskID:   "t-002",
				Status:   "failed",
				Failures: []string{"[suspect-proof-check] awk failed for subcommand ordering"},
			},
		},
	}
	stage := NewExecuteStage(runner, ExecuteStageConfig{MaxRetries: 0})

	// Invoke
	action, err := stage.Run(context.Background(), seeded)
	if err != nil {
		t.Fatalf("ExecuteStage.Run: %v", err)
	}

	// Assert
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected non-nil FailureContext")
	}

	failuresJoined := strings.Join(action.Context.Failures, "\n")
	if !strings.Contains(failuresJoined, "[suspect-proof-check] grep failed for --flag-a") {
		t.Fatalf("expected first per-task failure in FailureContext.Failures, got: %v", action.Context.Failures)
	}
	if !strings.Contains(failuresJoined, "[suspect-proof-check] awk failed for subcommand ordering") {
		t.Fatalf("expected second per-task failure in FailureContext.Failures, got: %v", action.Context.Failures)
	}
	if strings.Contains(failuresJoined, "all tasks failed") {
		t.Fatalf("did not expect generic fallback failure, got: %v", action.Context.Failures)
	}
}

func mustSaveRunState(t *testing.T, store *runstore.Store, rs *runstore.RunState) {
	t.Helper()
	if err := store.Save(rs); err != nil {
		t.Fatalf("store.Save(%s): %v", rs.RunID, err)
	}
}
