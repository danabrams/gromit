package stages

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type runnerLevelFailureTaskRunner struct{}

func (r *runnerLevelFailureTaskRunner) RunTask(ctx context.Context, task runstore.Task) (specloop.TaskResult, error) {
	// Runner-level execution failure (not inspection failure).
	// Any Failures returned here should be cleared by RunTaskLoop.
	return specloop.TaskResult{
		Status:   "failed",
		Failures: []string{"runner-level failure detail that should not propagate"},
	}, errors.New("runner execution error")
}

func (r *runnerLevelFailureTaskRunner) RepairTask(ctx context.Context, task runstore.Task, failures []string) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

func TestScenario_Execute_AllFailedWithoutPerTaskFailures_UsesGenericFallback(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "first task"},
		{TaskID: "t-002", Status: "pending", Objective: "second task"},
	}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save runstate: %v", err)
	}

	// Invoke
	stage := NewExecuteStage(&runnerLevelFailureTaskRunner{}, ExecuteStageConfig{MaxRetries: 0})
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("stage.Run: %v", err)
	}

	// Assert
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected non-nil failure context")
	}
	if len(action.Context.Failures) != 1 {
		t.Fatalf("expected exactly one fallback failure, got %v", action.Context.Failures)
	}

	joined := strings.Join(action.Context.Failures, "\n")
	if !strings.Contains(joined, "all tasks failed") {
		t.Fatalf("expected generic fallback failure, got %v", action.Context.Failures)
	}
	if strings.Contains(joined, "runner execution error") {
		t.Fatalf("did not expect runner error string in failure context, got %v", action.Context.Failures)
	}
}
