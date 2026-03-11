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

type fakeDecomposer struct {
	subTasks []runstore.Task
}

func (f *fakeDecomposer) Decompose(_ context.Context, _ runstore.Task) ([]runstore.Task, error) {
	return f.subTasks, nil
}

type fakeGitOps struct {
	checkoutCalled bool
	checkoutFiles  []string
}

func (f *fakeGitOps) CheckoutFiles(_ string, files []string) error {
	f.checkoutCalled = true
	f.checkoutFiles = files
	return nil
}

func TestTaskLoop_RedecomposesOnNeedsSplit(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		if task.TaskID == "t-001" {
			return TaskResult{Status: "needs_split"}, nil
		}
		return TaskResult{Status: "done"}, nil
	}}
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "t-001a", Status: "pending"},
		{TaskID: "t-001b", Status: "pending"},
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	results, _ := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Inspector: inspector, Decomposer: decomposer, MaxRedecompositions: 1,
	})

	doneCount := 0
	for _, r := range results {
		if r.Status == "done" {
			doneCount++
		}
	}
	if doneCount != 2 {
		t.Fatalf("expected 2 done sub-tasks, got %d", doneCount)
	}
}

func TestTaskLoop_RevertBeforeRedecompose(t *testing.T) {
	gitOps := &fakeGitOps{}
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		if task.TaskID == "t-001" {
			return TaskResult{Status: "needs_split", FilesChanged: []string{"a.go", "b.go"}}, nil
		}
		return TaskResult{Status: "done"}, nil
	}}
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{{TaskID: "t-001a", Status: "pending"}}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Inspector: inspector, Decomposer: decomposer,
		MaxRedecompositions: 1, GitOps: gitOps,
	})

	if !gitOps.checkoutCalled {
		t.Fatal("git checkout should be called before sub-task execution")
	}
}

func TestTaskLoop_SubTasksCannotFurtherDecompose(t *testing.T) {
	decomposeCalls := 0
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "t-001a", Status: "pending"},
	}}
	// Wrap to count calls
	origDecompose := decomposer.Decompose
	_ = origDecompose

	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		// Both parent and sub-task return needs_split
		return TaskResult{Status: "needs_split"}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	countingDecomposer := &countingDecomposer{
		subTasks: []runstore.Task{{TaskID: "t-001a", Status: "pending"}},
		calls:    &decomposeCalls,
	}

	RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Inspector: inspector, Decomposer: countingDecomposer,
		MaxRedecompositions: 2,
	})

	// Only the parent should trigger decomposition, not the sub-task
	if decomposeCalls != 1 {
		t.Fatalf("expected 1 decompose call (parent only), got %d", decomposeCalls)
	}
}

type countingDecomposer struct {
	subTasks []runstore.Task
	calls    *int
}

func (f *countingDecomposer) Decompose(_ context.Context, _ runstore.Task) ([]runstore.Task, error) {
	*f.calls++
	return f.subTasks, nil
}
