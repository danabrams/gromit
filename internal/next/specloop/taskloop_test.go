package specloop

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	pass     bool
	fn       func() bool
	resultFn func() InspectResult
}

func (f *fakeInspector) Inspect(_ context.Context, _ runstore.Task) InspectResult {
	if f.resultFn != nil {
		return f.resultFn()
	}
	if f.fn != nil {
		return InspectResult{Pass: f.fn()}
	}
	return InspectResult{Pass: f.pass}
}

func (f *fakeInspector) SetKnownGaps(_ string) {
	// no-op for testing
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

func TestTaskResult_NormalizeNilFields(t *testing.T) {
	tr := TaskResult{TaskID: "t-001", Status: "done"}
	if tr.FilesChanged != nil {
		t.Fatal("precondition: FilesChanged should be nil before normalization")
	}
	tr.NormalizeNilFields()
	if tr.FilesChanged == nil {
		t.Fatal("FilesChanged should be non-nil after normalization")
	}
	if len(tr.FilesChanged) != 0 {
		t.Fatalf("FilesChanged should be empty, got %d elements", len(tr.FilesChanged))
	}
}

func TestTaskResult_NormalizeNilFields_PreservesExisting(t *testing.T) {
	tr := TaskResult{FilesChanged: []string{"main.go", "util.go"}}
	tr.NormalizeNilFields()
	if len(tr.FilesChanged) != 2 {
		t.Fatalf("expected 2 files preserved, got %d", len(tr.FilesChanged))
	}
	if tr.FilesChanged[0] != "main.go" {
		t.Fatalf("expected main.go, got %q", tr.FilesChanged[0])
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

func TestTaskLoop_DetectFilesChanged_PopulatesResult(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done"}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	callCount := 0
	detector := func(workDir string) ([]string, error) {
		callCount++
		if callCount == 1 {
			return []string{}, nil // before: no pre-existing changes
		}
		return []string{"main.go", "util.go"}, nil // after: task added two files
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:         0,
		Inspector:          inspector,
		WorkDir:            "/tmp/test",
		DetectFilesChanged: detector,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results[0].FilesChanged) != 2 {
		t.Fatalf("expected 2 files changed, got %v", results[0].FilesChanged)
	}
	if results[0].FilesChanged[0] != "main.go" || results[0].FilesChanged[1] != "util.go" {
		t.Fatalf("unexpected files: %v", results[0].FilesChanged)
	}
}

func TestTaskLoop_DetectFilesChanged_NilDetectorKeepsRunnerResult(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done", FilesChanged: []string{"from-runner.go"}}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 0,
		Inspector:  inspector,
		WorkDir:    "/tmp/test",
		// DetectFilesChanged is nil — should keep runner's result
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results[0].FilesChanged) != 1 || results[0].FilesChanged[0] != "from-runner.go" {
		t.Fatalf("expected runner's files unchanged, got %v", results[0].FilesChanged)
	}
}

func TestTaskLoop_DetectFilesChanged_ExcludesPreExistingFiles(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done"}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	// Simulates the new stateful closure semantics:
	//   call 1 (before task): captures baseline, returns []
	//   call 2 (after task):  computes delta, returns changed files
	callCount := 0
	detector := func(workDir string) ([]string, error) {
		callCount++
		if callCount == 1 {
			// before snapshot: capture baseline (return empty — pre-existing files
			// are recorded internally, not returned)
			return []string{}, nil
		}
		// after snapshot: return only files that changed during the task
		return []string{"task-added.go"}, nil
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:         0,
		Inspector:          inspector,
		WorkDir:            "/tmp/test",
		DetectFilesChanged: detector,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results[0].FilesChanged) != 1 {
		t.Fatalf("expected exactly 1 file (task-added.go), got %v", results[0].FilesChanged)
	}
	if results[0].FilesChanged[0] != "task-added.go" {
		t.Fatalf("expected task-added.go, got %q", results[0].FilesChanged[0])
	}
	if callCount != 2 {
		t.Fatalf("expected detector called twice (before and after), got %d", callCount)
	}
}

func TestTaskLoop_DetectFilesChanged_ErrorFallsBack(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done", FilesChanged: []string{"original.go"}}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	detector := func(workDir string) ([]string, error) {
		return nil, fmt.Errorf("git not available")
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:         0,
		Inspector:          inspector,
		WorkDir:            "/tmp/test",
		DetectFilesChanged: detector,
	})
	if err != nil {
		t.Fatal(err)
	}
	// On detector error, should keep the runner's original FilesChanged
	if len(results[0].FilesChanged) != 1 || results[0].FilesChanged[0] != "original.go" {
		t.Fatalf("expected original files on detector error, got %v", results[0].FilesChanged)
	}
}

// TestTaskLoop_NeedsSplit_DetectedFromFilesChanged verifies that when a task
// runner returns "done" but the files changed span 3+ distinct directories,
// the taskloop promotes the result to "needs_split" and triggers decomposition.
func TestTaskLoop_NeedsSplit_DetectedFromFilesChanged(t *testing.T) {
	// Runner returns "done" — never sets needs_split itself.
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done"}, nil
	}}
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "t-001a", Status: "pending"},
		{TaskID: "t-001b", Status: "pending"},
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	// Detector returns files in 3 different directories for the parent task only.
	// Sub-tasks return no changed files so they don't retrigger NeedsSplit.
	callCount := 0
	detector := func(workDir string) ([]string, error) {
		callCount++
		if callCount%2 == 1 {
			return []string{}, nil // baseline calls (odd): always return empty
		}
		if callCount == 2 {
			// Second call = delta for t-001: 3 distinct parent dirs triggers NeedsSplit.
			return []string{"pkg/a/file.go", "pkg/b/file.go", "pkg/c/file.go"}, nil
		}
		// Delta calls for sub-tasks: no files changed.
		return []string{}, nil
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:          0,
		Inspector:           inspector,
		Decomposer:          decomposer,
		MaxRedecompositions: 1,
		WorkDir:             "/tmp/test",
		DetectFilesChanged:  detector,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The parent t-001 should appear in results as "decomposed".
	// Sub-tasks t-001a and t-001b should be "done".
	doneCount := 0
	parentOK := false
	for _, r := range results {
		if r.Status == "done" {
			doneCount++
		}
		if r.TaskID == "t-001" {
			if r.Status != "decomposed" {
				t.Fatalf("parent t-001 expected status 'decomposed', got %q", r.Status)
			}
			parentOK = true
		}
	}
	if !parentOK {
		t.Fatal("parent t-001 not found in results")
	}
	if doneCount != 2 {
		t.Fatalf("expected 2 done sub-tasks after NeedsSplit detection, got %d done (results: %v)", doneCount, results)
	}
}

// TestTaskLoop_NeedsSplit_FilesChangedPopulatedBeforeHandler verifies that
// result.FilesChanged is populated before the needs_split handler runs,
// enabling the revert (CheckoutFiles) to work correctly.
func TestTaskLoop_NeedsSplit_FilesChangedPopulatedBeforeHandler(t *testing.T) {
	gitOps := &fakeGitOps{}

	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done"}, nil
	}}
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "t-001a", Status: "pending"},
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	// Files in 3 dirs — NeedsSplit triggers. FilesChanged must be populated
	// before the handler so CheckoutFiles receives the actual changed files.
	callCount := 0
	detector := func(workDir string) ([]string, error) {
		callCount++
		if callCount%2 == 1 {
			return []string{}, nil // baseline calls (odd): always return empty
		}
		if callCount == 2 {
			// Delta for t-001: 3 files across 3 dirs triggers NeedsSplit.
			return []string{"pkg/a/file.go", "pkg/b/file.go", "pkg/c/file.go"}, nil
		}
		return []string{}, nil // sub-task deltas: no files
	}

	RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:          0,
		Inspector:           inspector,
		Decomposer:          decomposer,
		MaxRedecompositions: 1,
		GitOps:              gitOps,
		WorkDir:             "/tmp/test",
		DetectFilesChanged:  detector,
	})

	// GitOps.CheckoutFiles should have been called with the detected files.
	if !gitOps.checkoutCalled {
		t.Fatal("expected git checkout to be called for revert before decomposition")
	}
	if len(gitOps.checkoutFiles) != 3 {
		t.Fatalf("expected 3 files to be reverted, got %d: %v", len(gitOps.checkoutFiles), gitOps.checkoutFiles)
	}
}

// TestTaskLoop_NeedsSplit_OnlyForDoneStatus verifies that NeedsSplit detection
// is skipped when the runner returns "failed" (only applied to "done" results).
func TestTaskLoop_NeedsSplit_OnlyForDoneStatus(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "failed"}, nil
	}}
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "t-001a", Status: "pending"},
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	callCount := 0
	detector := func(workDir string) ([]string, error) {
		callCount++
		if callCount == 1 {
			return []string{}, nil
		}
		// 3 distinct dirs — would trigger NeedsSplit, but runner returned "failed"
		return []string{"pkg/a/file.go", "pkg/b/file.go", "pkg/c/file.go"}, nil
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:          0,
		Inspector:           inspector,
		Decomposer:          decomposer,
		MaxRedecompositions: 1,
		WorkDir:             "/tmp/test",
		DetectFilesChanged:  detector,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Runner returned "failed" — should not be promoted to needs_split or decomposed.
	if len(results) != 1 {
		t.Fatalf("expected 1 result (no decomposition), got %d", len(results))
	}
	if results[0].Status != "failed" {
		t.Fatalf("expected status 'failed', got %q", results[0].Status)
	}
}

// TestTaskLoop_TestFileCoverage_MissingTestFile verifies that when a *_test.go
// file is listed in expected_touched_area but not in files_changed, the
// structural safety-net check fails inspection with the right message.
func TestTaskLoop_TestFileCoverage_MissingTestFile(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		// Only returns a non-test file; the test file is NOT changed.
		return TaskResult{Status: "done", FilesChanged: []string{"pkg/foo.go"}}, nil
	}}
	inspector := &fakeInspector{pass: true} // LLM checks all pass
	tasks := []runstore.Task{{
		TaskID:              "t-001",
		Status:              "pending",
		ExpectedTouchedArea: []string{"pkg/foo.go", "pkg/foo_test.go"},
	}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 0,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	if results[0].Status != "failed" {
		t.Fatalf("expected status 'failed' when test file missing from files_changed, got %q", results[0].Status)
	}
}

// TestTaskLoop_TestFileCoverage_TestFilePresent verifies that when a *_test.go
// file is in both expected_touched_area and files_changed, inspection still passes.
func TestTaskLoop_TestFileCoverage_TestFilePresent(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done", FilesChanged: []string{"pkg/foo.go", "pkg/foo_test.go"}}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{
		TaskID:              "t-001",
		Status:              "pending",
		ExpectedTouchedArea: []string{"pkg/foo.go", "pkg/foo_test.go"},
	}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 0,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	if results[0].Status != "done" {
		t.Fatalf("expected status 'done' when test file is present in files_changed, got %q", results[0].Status)
	}
}

// TestTaskLoop_TestFileCoverage_NonTestFileMissingDoesNotFail verifies that the
// structural check only applies to *_test.go files — non-test files missing from
// files_changed do NOT cause a failure.
func TestTaskLoop_TestFileCoverage_NonTestFileMissingDoesNotFail(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		// Only foo_test.go changed; bar.go (a non-test file) was not touched.
		return TaskResult{Status: "done", FilesChanged: []string{"pkg/foo_test.go"}}, nil
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{
		TaskID:              "t-001",
		Status:              "pending",
		ExpectedTouchedArea: []string{"pkg/foo_test.go", "pkg/bar.go"},
	}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 0,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	// bar.go is not a *_test.go file — its absence should not cause failure.
	if results[0].Status != "done" {
		t.Fatalf("expected status 'done' (non-test file missing is not enforced), got %q", results[0].Status)
	}
}

// TestRunTaskLoop_StructuralCheckSkippedWhenNoFilesChanged verifies that when
// a task has a *_test.go in expected_touched_area, the inspector returns Pass,
// but FilesChanged is empty (e.g. a git-only operation), the structural
// cross-check is skipped and the task is not downgraded to failed.
func TestRunTaskLoop_StructuralCheckSkippedWhenNoFilesChanged(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		// Agent did a git-only op; no file contents were modified.
		return TaskResult{Status: "done", FilesChanged: []string{}}, nil
	}}
	inspector := &fakeInspector{pass: true} // LLM checks all pass
	tasks := []runstore.Task{{
		TaskID:              "t-001",
		Status:              "pending",
		ExpectedTouchedArea: []string{"pkg/foo.go", "pkg/foo_test.go"},
	}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 0,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	// FilesChanged is empty — structural check must be skipped entirely.
	// The inspector passed, so the task should remain "done".
	if results[0].Status != "done" {
		t.Fatalf("expected status 'done' when FilesChanged is empty (git-only op), got %q", results[0].Status)
	}
}

// TestTaskLoop_DecomposedParentAppearsInResultsAsDecomposed verifies that when
// a task is successfully split into sub-tasks, the parent task appears in the
// results with status "decomposed" so execute.go can update rs.Tasks and prevent
// the parent from being re-queued on the next cycle.
func TestTaskLoop_DecomposedParentAppearsInResultsAsDecomposed(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		if task.TaskID == "t-001" {
			return TaskResult{Status: "needs_split"}, nil
		}
		return TaskResult{Status: "done"}, nil
	}}
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "t-002", Status: "pending"},
		{TaskID: "t-003", Status: "pending"},
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Inspector: inspector, Decomposer: decomposer, MaxRedecompositions: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Parent t-001 must appear in results with status "decomposed" so that
	// execute.go can mark it non-pending in rs.Tasks.
	var parentResult *TaskResult
	for i := range results {
		if results[i].TaskID == "t-001" {
			parentResult = &results[i]
			break
		}
	}
	if parentResult == nil {
		t.Fatal("parent t-001 not found in results — execute.go cannot update its status, causing re-queue next cycle")
	}
	if parentResult.Status != "decomposed" {
		t.Fatalf("expected parent status 'decomposed', got %q", parentResult.Status)
	}
}

// TestRunTaskLoop_PopulatesFailuresOnInspectionFailure verifies that when
// inspection fails after all retries, TaskResult.Failures contains the
// annotated failure messages.
func TestRunTaskLoop_PopulatesFailuresOnInspectionFailure(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done"}, nil
	}}
	// Inspector always fails with a grep failure
	inspector := &fakeInspector{
		resultFn: func() InspectResult {
			return InspectResult{
				Pass:     false,
				Failures: []string{"grep -q 'func Foo' foo.go: exit status 1"},
			}
		},
	}
	tasks := []runstore.Task{{
		TaskID:      "t-001",
		Status:      "pending",
		ProofChecks: []string{"go build ./...", "grep -q 'func Foo' foo.go"},
	}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "failed" {
		t.Errorf("expected failed, got %s", r.Status)
	}
	if len(r.Failures) == 0 {
		t.Error("expected Failures to be populated, got empty slice")
	}
	for _, f := range r.Failures {
		if !strings.HasPrefix(f, "[suspect-proof-check]") {
			t.Errorf("expected [suspect-proof-check] prefix, got: %s", f)
		}
	}
}

func TestRunTaskLoop_BuildFailurePreventsSuspectAnnotation(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done"}, nil
	}}
	inspector := &fakeInspector{
		resultFn: func() InspectResult {
			return InspectResult{
				Pass: false,
				Failures: []string{
					"go build ./...: exit status 1: undefined: Bar",
					"grep -q 'func Foo' foo.go: exit status 1",
				},
			}
		},
	}
	tasks := []runstore.Task{{
		TaskID:      "t-002",
		Status:      "pending",
		ProofChecks: []string{"go build ./...", "grep -q 'func Foo' foo.go"},
	}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if len(r.Failures) == 0 {
		t.Fatal("expected failures even when build check fails")
	}
	for _, f := range r.Failures {
		if strings.HasPrefix(f, "[suspect-proof-check]") {
			t.Fatalf("should not annotate build failures: %q", f)
		}
	}
}

func TestRunTaskLoop_NoBuildCheckPreventsSuspectAnnotation(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done"}, nil
	}}
	inspector := &fakeInspector{
		resultFn: func() InspectResult {
			return InspectResult{
				Pass: false,
				Failures: []string{
					"grep -q 'func Foo' foo.go: exit status 1",
				},
			}
		},
	}
	tasks := []runstore.Task{{
		TaskID:      "t-003",
		Status:      "pending",
		ProofChecks: []string{"grep -q 'func Foo' foo.go"},
	}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 0,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if len(r.Failures) == 0 {
		t.Fatal("expected failures even without build checks")
	}
	for _, f := range r.Failures {
		if strings.HasPrefix(f, "[suspect-proof-check]") {
			t.Fatalf("should not annotate when no build check exists: %q", f)
		}
	}
}

func TestTaskLoop_FailuresClearedOnRunnerError(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{
			Status:   "failed",
			Failures: []string{"runner: oh no"},
		}, fmt.Errorf("boom")
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{{TaskID: "t-004", Status: "pending"}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 0,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "failed" {
		t.Fatalf("expected failed status, got %q", results[0].Status)
	}
	if len(results[0].Failures) != 0 {
		t.Fatalf("expected Failures empty for runner-level failure, got %v", results[0].Failures)
	}
}

func TestTaskLoop_FailuresClearedAfterSuccessfulRetry(t *testing.T) {
	runner := &fakeTaskRunner{
		fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
			return TaskResult{Status: "done"}, nil
		},
		repairFn: func(_ context.Context, task runstore.Task, failures []string) (TaskResult, error) {
			return TaskResult{
				Status:   "done",
				Failures: []string{"repair run returned metadata"},
			}, nil
		},
	}
	callCount := 0
	inspector := &fakeInspector{
		resultFn: func() InspectResult {
			callCount++
			if callCount == 1 {
				return InspectResult{
					Pass:     false,
					Failures: []string{"grep -q 'Func' util.go: exit status 1"},
				}
			}
			return InspectResult{Pass: true}
		},
	}
	tasks := []runstore.Task{{TaskID: "t-005", Status: "pending"}}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "done" {
		t.Fatalf("expected done status, got %q", results[0].Status)
	}
	if len(results[0].Failures) != 0 {
		t.Fatalf("expected Failures empty for success after retry, got %v", results[0].Failures)
	}
}

func TestIsBuildCheck_AllowsAdditionalArgs(t *testing.T) {
	if !isBuildCheck("go build ./...") {
		t.Fatal("expected go build with ./... to be recognized as build check")
	}
	if !isBuildCheck("cargo build --release") {
		t.Fatal("expected cargo build with args to be recognized as build check")
	}
	if isBuildCheck("go test ./...") {
		t.Fatal("go test should not be classified as a build check")
	}
	if isBuildCheck("grep -q 'func Foo' foo.go") {
		t.Fatal("grep should not be classified as a build check")
	}
}

func TestIsBuildCheck_MvnMakeCases(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"mvn compile", true},
		{"mvn compile -q", true},
		{"make build", true},
		{"make build VERBOSE=1", true},
	}
	for _, tc := range cases {
		if isBuildCheck(tc.cmd) != tc.want {
			t.Errorf("isBuildCheck(%q) = %v, want %v", tc.cmd, isBuildCheck(tc.cmd), tc.want)
		}
	}
}

func TestIsBuildCheck_NpmRunBuild(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"npm run build", true},
		{"npm run build -- --mode=prod", true},
	}
	for _, tc := range cases {
		if isBuildCheck(tc.cmd) != tc.want {
			t.Errorf("isBuildCheck(%q) = %v, want %v", tc.cmd, isBuildCheck(tc.cmd), tc.want)
		}
	}
}

func TestIsBuildCheck_NpmNonBuildIsNotBuildCheck(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"npm run test", false},
		{"npm run lint", false},
		{"npm build", false},
	}
	for _, tc := range cases {
		if isBuildCheck(tc.cmd) != tc.want {
			t.Errorf("isBuildCheck(%q) = %v, want %v", tc.cmd, isBuildCheck(tc.cmd), tc.want)
		}
	}
}

// TestRunTaskLoop_RedecompositionIDsContinueFromMax verifies that when a task
// is decomposed into sub-tasks, the sub-task IDs are renumbered to continue
// from the current maximum task ID in the queue — preventing ID collisions.
func TestRunTaskLoop_RedecompositionIDsContinueFromMax(t *testing.T) {
	// Queue starts with t-001 through t-005; t-006 triggers decomposition.
	// The decomposer returns sub-tasks with colliding IDs t-001..t-003.
	// After renumbering they should become t-007, t-008, t-009.

	var executedIDs []string
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		executedIDs = append(executedIDs, task.TaskID)
		if task.TaskID == "t-006" {
			return TaskResult{Status: "needs_split"}, nil
		}
		return TaskResult{Status: "done"}, nil
	}}
	// Decomposer returns sub-tasks with IDs that collide with the existing queue.
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "t-001", Status: "pending"},
		{TaskID: "t-002", Status: "pending"},
		{TaskID: "t-003", Status: "pending"},
	}}
	inspector := &fakeInspector{pass: true}
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending"},
		{TaskID: "t-002", Status: "pending"},
		{TaskID: "t-003", Status: "pending"},
		{TaskID: "t-004", Status: "pending"},
		{TaskID: "t-005", Status: "pending"},
		{TaskID: "t-006", Status: "pending"},
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:          0,
		Inspector:           inspector,
		Decomposer:          decomposer,
		MaxRedecompositions: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Collect the IDs of sub-tasks that were actually executed (from decomposition).
	// We expect t-007, t-008, t-009 — not t-001..t-003 again.
	subTaskIDs := map[string]bool{}
	for _, id := range executedIDs {
		subTaskIDs[id] = true
	}

	// The original t-001..t-003 were run before decomposition; those entries
	// in executedIDs are fine. What we must NOT see is a *second* execution of
	// t-001, t-002, or t-003 from the decomposed sub-tasks. Instead we expect
	// t-007, t-008, t-009 in the results.
	foundRenumbered := 0
	for _, r := range results {
		if r.TaskID == "t-007" || r.TaskID == "t-008" || r.TaskID == "t-009" {
			foundRenumbered++
			if r.Status != "done" {
				t.Errorf("expected renumbered sub-task %s to be done, got %q", r.TaskID, r.Status)
			}
		}
	}
	if foundRenumbered != 3 {
		var ids []string
		for _, r := range results {
			ids = append(ids, r.TaskID+"="+r.Status)
		}
		t.Fatalf("expected 3 renumbered sub-tasks (t-007..t-009) in results, got %d; results: %v", foundRenumbered, ids)
	}
}

// TestTaskLoop_CapturesFilesChangedOnRunnerError verifies that when the runner
// returns an error, DetectFilesChanged is still called and its result is captured
// in FilesChanged (Fix A1).
func TestTaskLoop_CapturesFilesChangedOnRunnerError(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{}, fmt.Errorf("runner LLM error")
	}}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	callCount := 0
	detector := func(workDir string) ([]string, error) {
		callCount++
		if callCount == 1 {
			return []string{}, nil // baseline
		}
		return []string{"foo.go"}, nil // files written before runner errored
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:         0,
		WorkDir:            "/tmp/test",
		DetectFilesChanged: detector,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "failed" {
		t.Fatalf("expected failed status, got %q", results[0].Status)
	}
	if len(results[0].FilesChanged) != 1 || results[0].FilesChanged[0] != "foo.go" {
		t.Fatalf("expected FilesChanged=[foo.go] on runner error, got %v", results[0].FilesChanged)
	}
}

// TestTaskLoop_EmitsPhantomTaskFailureWhenExpectedFilesMissing verifies that when
// a task fails with empty files_changed and an expected file does not exist on disk,
// a phantom_task_failure event is emitted (Fix A2).
func TestTaskLoop_EmitsPhantomTaskFailureWhenExpectedFilesMissing(t *testing.T) {
	workDir := t.TempDir()
	// "new_feature.go" is not created on disk — it's the phantom file.
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{}, fmt.Errorf("runner LLM error")
	}}
	tasks := []runstore.Task{{
		TaskID:              "t-001",
		Status:              "pending",
		ExpectedTouchedArea: []string{"new_feature.go"},
	}}

	el, _ := newTestEventLog(t)
	detector := func(_ string) ([]string, error) {
		return []string{}, nil // no files written
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:         0,
		WorkDir:            workDir,
		DetectFilesChanged: detector,
		EventLog:           el,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "failed" {
		t.Fatalf("expected failed, got %q", results[0].Status)
	}

	events, readErr := el.ReadAll()
	if readErr != nil {
		t.Fatalf("read events: %v", readErr)
	}
	var found bool
	for _, ev := range events {
		if ev.EventType() == "phantom_task_failure" {
			pev := ev.(*runstore.PhantomTaskFailureEvent)
			if pev.TaskID != "t-001" {
				t.Errorf("phantom event TaskID: want t-001, got %q", pev.TaskID)
			}
			if len(pev.MissingFiles) != 1 || pev.MissingFiles[0] != "new_feature.go" {
				t.Errorf("phantom event MissingFiles: want [new_feature.go], got %v", pev.MissingFiles)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected phantom_task_failure event to be emitted, none found")
	}
}

// TestTaskLoop_NoPhantomEventWhenFilesWereChanged verifies that no phantom event
// is emitted when files_changed is non-empty, even if the task failed.
func TestTaskLoop_NoPhantomEventWhenFilesWereChanged(t *testing.T) {
	workDir := t.TempDir()
	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{Status: "done"}, nil
	}}
	// Inspector always fails — task ends up failed with files_changed set by detector.
	inspector := &fakeInspector{pass: false}
	tasks := []runstore.Task{{
		TaskID:              "t-001",
		Status:              "pending",
		ExpectedTouchedArea: []string{"new_feature.go"},
	}}

	el, _ := newTestEventLog(t)
	callCount := 0
	detector := func(_ string) ([]string, error) {
		callCount++
		if callCount == 1 {
			return []string{}, nil
		}
		return []string{"some_file.go"}, nil // files were changed
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:         0,
		Inspector:          inspector,
		WorkDir:            workDir,
		DetectFilesChanged: detector,
		EventLog:           el,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "failed" {
		t.Fatalf("expected failed, got %q", results[0].Status)
	}

	events, readErr := el.ReadAll()
	if readErr != nil {
		t.Fatalf("read events: %v", readErr)
	}
	for _, ev := range events {
		if ev.EventType() == "phantom_task_failure" {
			t.Fatal("unexpected phantom_task_failure event emitted when files_changed is non-empty")
		}
	}
}

// TestTaskLoop_NoPhantomEventWhenExpectedFilesExist verifies that no phantom event
// is emitted when files_changed is empty but the expected file already exists on disk.
func TestTaskLoop_NoPhantomEventWhenExpectedFilesExist(t *testing.T) {
	workDir := t.TempDir()
	// Create the expected file on disk so it's not "missing".
	existingFile := workDir + "/existing_feature.go"
	if err := os.WriteFile(existingFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("setup: write file: %v", err)
	}

	runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
		return TaskResult{}, fmt.Errorf("runner LLM error")
	}}
	tasks := []runstore.Task{{
		TaskID:              "t-001",
		Status:              "pending",
		ExpectedTouchedArea: []string{"existing_feature.go"},
	}}

	el, _ := newTestEventLog(t)
	detector := func(_ string) ([]string, error) {
		return []string{}, nil // no files changed
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries:         0,
		WorkDir:            workDir,
		DetectFilesChanged: detector,
		EventLog:           el,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "failed" {
		t.Fatalf("expected failed, got %q", results[0].Status)
	}

	events, readErr := el.ReadAll()
	if readErr != nil {
		t.Fatalf("read events: %v", readErr)
	}
	for _, ev := range events {
		if ev.EventType() == "phantom_task_failure" {
			t.Fatal("unexpected phantom_task_failure event: expected file exists on disk, no phantom should fire")
		}
	}
}
