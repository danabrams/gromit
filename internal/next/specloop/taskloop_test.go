package specloop

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
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

func TestTaskLoop_MaxRedecompositions_GlobalBudget(t *testing.T) {
	// Two tasks both return needs_split, MaxRedecompositions=1.
	// First task decomposes; second task should be marked failed.
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		if task.TaskID == "t-001" || task.TaskID == "t-002" {
			return TaskResult{Status: "needs_split"}, nil
		}
		return TaskResult{Status: "done"}, nil
	}}
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "sub-1", Status: "pending"},
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending"},
		{TaskID: "t-002", Status: "pending"},
	}

	results, _ := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Inspector: inspector, Decomposer: decomposer,
		MaxRedecompositions: 1,
	})

	// results: t-002 (failed, budget exhausted), sub-1 (done)
	var failedCount, doneCount int
	for _, r := range results {
		switch r.Status {
		case "failed":
			failedCount++
		case "done":
			doneCount++
		}
	}
	if doneCount != 1 {
		t.Fatalf("expected 1 done (sub-task), got %d", doneCount)
	}
	if failedCount != 1 {
		t.Fatalf("expected 1 failed (second task couldn't decompose), got %d", failedCount)
	}
}

func TestTaskLoop_RetryExhaustion_SetsFailed(t *testing.T) {
	// Runner returns "done" but inspector always fails.
	// After MaxRetries, result.Status should be "failed".
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done"}, nil
	}}
	inspector := &fakeInspector{pass: false} // always fails
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	results, _ := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 2, Inspector: inspector,
	})

	if results[0].Status != "failed" {
		t.Fatalf("expected failed after retry exhaustion, got %s", results[0].Status)
	}
	if results[0].Attempts != 3 { // 1 initial + 2 retries
		t.Fatalf("expected 3 attempts, got %d", results[0].Attempts)
	}
}

func TestTaskLoop_BudgetBlocksRemainingTasks(t *testing.T) {
	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})

	executed := []string{}
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		executed = append(executed, task.TaskID)
		return TaskResult{Status: "done", Cost: 2.0}, nil // exceed budget after first task
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending"},
		{TaskID: "t-002", Status: "pending"},
	}

	results, _ := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Inspector: inspector, Budget: budget,
	})

	if len(executed) != 1 {
		t.Fatalf("expected 1 task executed before budget exceeded, got %d", len(executed))
	}
	if results[1].Status != "blocked" {
		t.Fatalf("expected second task blocked, got %s", results[1].Status)
	}
}

func TestTaskLoop_CostAddedToBudgetAfterEachTask(t *testing.T) {
	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 100.0, MaxSpecCycles: 99})

	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done", Cost: 1.5}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending"},
		{TaskID: "t-002", Status: "pending"},
	}

	_, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 0, Inspector: inspector, Budget: budget,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := budget.Cost()
	if got != 3.0 {
		t.Fatalf("expected budget cost 3.0 after two tasks, got %f", got)
	}
}

func TestTaskLoop_BudgetExceededBetweenTasks(t *testing.T) {
	// Budget of 1.0 USD; first task costs 1.5 => exceeds budget before second task
	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})

	executed := []string{}
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		executed = append(executed, task.TaskID)
		return TaskResult{Status: "done", Cost: 1.5}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending"},
		{TaskID: "t-002", Status: "pending"},
		{TaskID: "t-003", Status: "pending"},
	}

	results, _ := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 0, Inspector: inspector, Budget: budget,
	})

	if len(executed) != 1 {
		t.Fatalf("expected only 1 task executed, got %d: %v", len(executed), executed)
	}
	if results[0].Status != "done" {
		t.Fatalf("expected first task done, got %s", results[0].Status)
	}
	if results[1].Status != "blocked" {
		t.Fatalf("expected second task blocked, got %s", results[1].Status)
	}
	if results[2].Status != "blocked" {
		t.Fatalf("expected third task blocked, got %s", results[2].Status)
	}
}

func TestTaskLoop_RepairTaskGetsTimeoutContext(t *testing.T) {
	var repairDeadlineSet bool

	runner := &fakeTaskRunner{
		fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
			return TaskResult{Status: "done"}, nil
		},
		repairFn: func(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error) {
			_, ok := ctx.Deadline()
			repairDeadlineSet = ok
			return TaskResult{Status: "done"}, nil
		},
	}
	inspector := &fakeInspector{fn: func() bool {
		return false // always fail to trigger repair
	}}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:             1,
		Inspector:              inspector,
		MaxTaskDurationSeconds: 60,
	})

	if !repairDeadlineSet {
		t.Fatal("RepairTask context should have a deadline when MaxTaskDurationSeconds is set")
	}
}

func TestTaskLoop_RepairCostAddedToBudget(t *testing.T) {
	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 100.0, MaxSpecCycles: 99})

	inspectCalls := 0
	runner := &fakeTaskRunner{
		fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
			return TaskResult{Status: "done", Cost: 1.0}, nil
		},
		repairFn: func(_ context.Context, task runstore.Task, failures []string) (TaskResult, error) {
			return TaskResult{Status: "done", Cost: 2.0}, nil
		},
	}
	inspector := &fakeInspector{fn: func() bool {
		inspectCalls++
		return inspectCalls >= 2 // fail first, pass second
	}}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	_, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Inspector: inspector, Budget: budget,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1.0 from RunTask + 2.0 from RepairTask = 3.0
	got := budget.Cost()
	if got != 3.0 {
		t.Fatalf("expected budget cost 3.0 (run + repair), got %f", got)
	}
}

func TestTaskLoop_RunTaskGetsTimeoutContext(t *testing.T) {
	var runDeadlineSet bool

	runner := &fakeTaskRunner{fn: func(ctx context.Context, task runstore.Task) (TaskResult, error) {
		dl, ok := ctx.Deadline()
		runDeadlineSet = ok
		if ok {
			// Verify deadline is roughly MaxTaskDurationSeconds from now
			remaining := time.Until(dl)
			if remaining < 50*time.Second || remaining > 70*time.Second {
				return TaskResult{}, nil
			}
		}
		return TaskResult{Status: "done"}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:             0,
		Inspector:              inspector,
		MaxTaskDurationSeconds: 60,
	})

	if !runDeadlineSet {
		t.Fatal("RunTask context should have a deadline when MaxTaskDurationSeconds is set")
	}
}
