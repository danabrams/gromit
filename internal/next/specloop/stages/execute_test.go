package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
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

func TestExecuteStage_AllTasksFailed_ReplanFrom(t *testing.T) {
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
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
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

// validatingTaskRunner validates that tasks received the expected ModelTier
type validatingTaskRunner struct {
	inner            *fakeTaskRunner
	receivedTier     string
	receivedTaskID   string
	taskToValidateID string
}

func (v *validatingTaskRunner) RunTask(ctx context.Context, task runstore.Task) (specloop.TaskResult, error) {
	if task.TaskID == v.taskToValidateID {
		v.receivedTier = task.ModelTier
		v.receivedTaskID = task.TaskID
	}
	return v.inner.RunTask(ctx, task)
}

func (v *validatingTaskRunner) RepairTask(ctx context.Context, task runstore.Task, failures []string) (specloop.TaskResult, error) {
	return v.inner.RepairTask(ctx, task, failures)
}

func TestExecuteStage_DecomposedParentNotRequeued(t *testing.T) {
	// After a task is decomposed, its status in rs.Tasks must be updated from
	// "pending" to "decomposed" so it is not re-queued on the next cycle.
	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			// t-001 returns "decomposed" (parent result emitted by taskloop after split)
			{Status: "decomposed"},
			// sub-tasks return "done"
			{Status: "done"},
			{Status: "done"},
		},
	}

	stage := NewExecuteStage(runner, ExecuteStageConfig{MaxRetries: 0})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "parent task"},
	}

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The parent task must no longer be "pending" — otherwise pendingTasks()
	// would re-queue it on the next cycle.
	if rs.Tasks[0].Status == "pending" {
		t.Fatal("parent task status is still 'pending' after decomposition — will be re-queued next cycle")
	}
	if rs.Tasks[0].Status != "decomposed" {
		t.Fatalf("expected parent status 'decomposed', got %q", rs.Tasks[0].Status)
	}
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

func TestExecuteStage_EscalatesModelWhenFixingFailedTask(t *testing.T) {
	// Use a validating runner that captures what ModelTier the task received
	innerRunner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done", Tier: "high"},
		},
	}
	validatingRunner := &validatingTaskRunner{
		inner:            innerRunner,
		taskToValidateID: "t-002",
	}

	stage := NewExecuteStage(validatingRunner, ExecuteStageConfig{
		MaxRetries: 0,
		Escalation: execpolicy.EscalationConfig{ModelEscalationThreshold: 3},
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.TaskLineage = map[string]runstore.TaskLineageEntry{
		"t-001": { // Prior failed task with 3+ consecutive failures (meets threshold)
			ConsecutiveFails: 3,
			ChainIDs:         []string{"t-001"},
		},
	}
	rs.Tasks = []runstore.Task{
		{
			TaskID:    "t-002",
			Status:    "pending",
			Objective: "fix t-001",
			Fixes:     "t-001", // This task is fixing t-001
			ModelTier: "medium",           // Initially medium
		},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify the task received the escalated ModelTier when executed
	if validatingRunner.receivedTier != "high" {
		t.Fatalf("expected task to receive ModelTier 'high' during execution, got %q", validatingRunner.receivedTier)
	}
}

func TestExecuteStage_NoEscalationWhenThresholdNotMet(t *testing.T) {
	// Use a validating runner that captures what ModelTier the task received
	innerRunner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done", Tier: "medium"},
		},
	}
	validatingRunner := &validatingTaskRunner{
		inner:            innerRunner,
		taskToValidateID: "t-002",
	}

	stage := NewExecuteStage(validatingRunner, ExecuteStageConfig{
		MaxRetries: 0,
		Escalation: execpolicy.EscalationConfig{ModelEscalationThreshold: 3},
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.TaskLineage = map[string]runstore.TaskLineageEntry{
		"t-001": { // Prior failed task with only 2 consecutive failures (below threshold of 3)
			ConsecutiveFails: 2,
			ChainIDs:         []string{"t-001"},
		},
	}
	rs.Tasks = []runstore.Task{
		{
			TaskID:    "t-002",
			Status:    "pending",
			Objective: "fix t-001",
			Fixes:     "t-001",
			ModelTier: "medium",
		},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify the task received the original ModelTier (no escalation)
	if validatingRunner.receivedTier != "medium" {
		t.Fatalf("expected task to receive ModelTier 'medium' (no escalation), got %q", validatingRunner.receivedTier)
	}
}
