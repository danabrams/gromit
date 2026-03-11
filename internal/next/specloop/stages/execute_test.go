package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type fakeTaskRunner struct {
	results []specloop.TaskResult
	idx     int
}

func (f *fakeTaskRunner) RunTask(ctx context.Context, task runstore.Task) (specloop.TaskResult, error) {
	if f.idx >= len(f.results) {
		return specloop.TaskResult{Status: "done"}, nil
	}
	r := f.results[f.idx]
	f.idx++
	return r, nil
}

func (f *fakeTaskRunner) RepairTask(ctx context.Context, task runstore.Task, failures []string) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

// Verify ExecuteStage satisfies the Stage interface.
var _ specloop.Stage = (*ExecuteStage)(nil)

func TestExecuteStage_RunsTaskLoop(t *testing.T) {
	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done", FilesChanged: []string{"a.go"}},
			{Status: "done", FilesChanged: []string{"b.go"}},
		},
	}

	stage := NewExecuteStage(runner, ExecuteStageConfig{MaxRetries: 1})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "first"},
		{TaskID: "t-002", Status: "pending", Objective: "second"},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	// Verify tasks were updated
	for _, task := range rs.Tasks {
		if task.Status != "done" {
			t.Fatalf("expected task %s status 'done', got %q", task.TaskID, task.Status)
		}
	}
}

func TestExecuteStage_AllTasksFailed_NeedsHuman(t *testing.T) {
	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "failed"},
			{Status: "failed"},
		},
	}

	stage := NewExecuteStage(runner, ExecuteStageConfig{MaxRetries: 0})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "first"},
		{TaskID: "t-002", Status: "pending", Objective: "second"},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.NeedsHuman {
		t.Fatalf("expected NeedsHuman, got %v", action.Kind)
	}
}

func TestExecuteStage_SkipsCompletedTasks(t *testing.T) {
	callCount := 0
	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done", FilesChanged: []string{"c.go"}},
		},
	}
	// Override RunTask to count calls
	countingRunner := &countingTaskRunner{inner: runner, count: &callCount}

	stage := NewExecuteStage(countingRunner, ExecuteStageConfig{MaxRetries: 0})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Objective: "already done from cycle 1"},
		{TaskID: "t-002", Status: "failed", Objective: "failed in cycle 1"},
		{TaskID: "t-003", Status: "pending", Objective: "new fix task"},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	// Only the pending task should have been executed
	if callCount != 1 {
		t.Fatalf("expected 1 RunTask call (only pending), got %d", callCount)
	}
	// Verify the completed task was not re-run
	if rs.Tasks[0].Status != "done" {
		t.Fatalf("cycle-1 done task should remain done, got %q", rs.Tasks[0].Status)
	}
}

type countingTaskRunner struct {
	inner *fakeTaskRunner
	count *int
}

func (c *countingTaskRunner) RunTask(ctx context.Context, task runstore.Task) (specloop.TaskResult, error) {
	*c.count++
	return c.inner.RunTask(ctx, task)
}

func (c *countingTaskRunner) RepairTask(ctx context.Context, task runstore.Task, failures []string) (specloop.TaskResult, error) {
	return c.inner.RepairTask(ctx, task, failures)
}

func TestExecuteStage_PartialFailure_Continue(t *testing.T) {
	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done", FilesChanged: []string{"a.go"}},
			{Status: "failed"},
		},
	}

	stage := NewExecuteStage(runner, ExecuteStageConfig{MaxRetries: 0})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "first"},
		{TaskID: "t-002", Status: "pending", Objective: "second"},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
}
