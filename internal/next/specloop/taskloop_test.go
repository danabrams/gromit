package specloop

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestTaskLoop_ExecutesAllTasks(t *testing.T) {
	executed := []string{}
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		executed = append(executed, task.TaskID)
		return TaskResult{Status: "done"}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending"},
		{TaskID: "t-002", Status: "pending"},
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Inspector: inspector,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if executed[0] != "t-001" || executed[1] != "t-002" {
		t.Fatalf("want [t-001 t-002], got %v", executed)
	}
}

func TestTaskLoop_RetriesOnFailure(t *testing.T) {
	runCalls := 0
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		runCalls++
		return TaskResult{Status: "done"}, nil
	}}
	inspectCalls := 0
	inspector := &fakeInspector{fn: func() bool {
		inspectCalls++
		return inspectCalls >= 2 // fail first, pass second
	}}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	results, _ := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Inspector: inspector,
	})
	if results[0].Status != "done" {
		t.Fatalf("expected done after retry, got %s", results[0].Status)
	}
	if results[0].Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", results[0].Attempts)
	}
}

// --- fakes ---

type fakeTaskRunner struct {
	fn       func(ctx context.Context, task runstore.Task) (TaskResult, error)
	repairFn func(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error)
}

func (f *fakeTaskRunner) RunTask(ctx context.Context, task runstore.Task) (TaskResult, error) {
	return f.fn(ctx, task)
}

func (f *fakeTaskRunner) RepairTask(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error) {
	if f.repairFn != nil {
		return f.repairFn(ctx, task, failures)
	}
	return f.fn(ctx, task)
}

type fakeInspector struct {
	pass bool
	fn   func() bool
}

func (f *fakeInspector) Inspect(_ context.Context, _ runstore.Task) InspectResult {
	if f.fn != nil {
		return InspectResult{Pass: f.fn()}
	}
	return InspectResult{Pass: f.pass}
}
