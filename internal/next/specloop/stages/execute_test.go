package stages

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/playbook"
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

type failingInspector struct {
	failures map[string][]string
}

func (f *failingInspector) Inspect(ctx context.Context, task runstore.Task) specloop.InspectResult {
	return specloop.InspectResult{
		Pass:     false,
		Failures: append([]string(nil), f.failures[task.TaskID]...),
	}
}

func (f *failingInspector) SetKnownGaps(gaps string) {}

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

// TestExecuteStage_AllFailed_PropagatesPerTaskFailures verifies that when all
// tasks fail, the FailureContext contains per-task failure strings from
// TaskResult.Failures rather than just the generic "all tasks failed" message.
func TestExecuteStage_AllFailed_PropagatesPerTaskFailures(t *testing.T) {
	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{
				Status:   "failed",
				Failures: []string{"[suspect-proof-check] grep failed"},
			},
			{
				Status:   "failed",
				Failures: []string{"[suspect-proof-check] awk failed"},
			},
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
		t.Fatal(err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected non-nil Context")
	}
	expected := []string{
		"[suspect-proof-check] grep failed",
		"[suspect-proof-check] awk failed",
	}
	if !reflect.DeepEqual(action.Context.Failures, expected) {
		t.Fatalf("expected failure list %v, got %v", expected, action.Context.Failures)
	}
}

func TestExecute_AllFailed_FailureContextReflectsTaskFailures(t *testing.T) {
	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done"},
			{Status: "done"},
		},
	}
	inspector := &failingInspector{
		failures: map[string][]string{
			"t-001": {"first task failure"},
			"t-002": {"second task failure", "second task extra failure"},
		},
	}
	stage := NewExecuteStage(runner, ExecuteStageConfig{MaxRetries: 0, Inspector: inspector})
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "first"},
		{TaskID: "t-002", Status: "pending", Objective: "second"},
	}
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected non-nil Context")
	}
	expected := []string{"first task failure", "second task failure", "second task extra failure"}
	if len(action.Context.Failures) != len(expected) {
		t.Fatalf("expected %d failures, got %d: %v", len(expected), len(action.Context.Failures), action.Context.Failures)
	}
	for i, failure := range action.Context.Failures {
		if failure != expected[i] {
			t.Fatalf("expected failure %d to be %q, got %q", i, expected[i], failure)
		}
	}
}

func TestExecute_AllFailed_FailureContextFallsBackWithoutTaskFailures(t *testing.T) {
	// When tasks fail but have no per-task failures, fall back to generic message.
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
		t.Fatal(err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if !reflect.DeepEqual(action.Context.Failures, []string{"all tasks failed"}) {
		t.Errorf("expected [\"all tasks failed\"], got: %v", action.Context.Failures)
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
			Fixes:     "t-001",  // This task is fixing t-001
			ModelTier: "medium", // Initially medium
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

func TestExecuteStage_LoadsPlaybookValidationGaps(t *testing.T) {
	tmpDir := t.TempDir()
	playbookDir := filepath.Join(tmpDir, "playbook")
	if err := os.MkdirAll(playbookDir, 0755); err != nil {
		t.Fatalf("failed to create playbook dir: %v", err)
	}

	// Create playbook with validation_gap entries
	entries := []playbook.Entry{
		{
			ID:      "pb-001",
			Type:    "validation_gap",
			Title:   "Check for error handling",
			Content: "Always verify error cases are handled",
			Status:  "active",
		},
		{
			ID:      "pb-002",
			Type:    "validation_gap",
			Title:   "Check for nil pointers",
			Content: "Verify nil checks before dereferencing",
			Status:  "active",
		},
		{
			ID:      "pb-003",
			Type:    "planner_heuristic",
			Title:   "Prefer small tasks",
			Content: "Smaller tasks are easier to fix",
			Status:  "active",
		},
		{
			ID:      "pb-004",
			Type:    "validation_gap",
			Title:   "Superseded gap",
			Content: "This is old",
			Status:  "superseded",
		},
	}
	store := &playbook.Store{Dir: playbookDir}
	if err := store.Save(entries); err != nil {
		t.Fatalf("failed to save playbook entries: %v", err)
	}

	// Create a mock ShellTaskInspector
	inspector := &specloop.ShellTaskInspector{}

	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done"},
		},
	}

	stage := NewExecuteStage(runner, ExecuteStageConfig{
		MaxRetries: 0,
		CellPath:   tmpDir,
		Inspector:  inspector,
		Escalation: execpolicy.EscalationConfig{},
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "test"},
	}

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify KnownGaps were set on the inspector
	if inspector.KnownGaps == "" {
		t.Fatal("expected KnownGaps to be set, got empty string")
	}

	// Verify KnownGaps contains active validation_gap entries
	if !strings.Contains(inspector.KnownGaps, "Check for error handling") {
		t.Errorf("expected 'Check for error handling' in KnownGaps, got %q", inspector.KnownGaps)
	}
	if !strings.Contains(inspector.KnownGaps, "Check for nil pointers") {
		t.Errorf("expected 'Check for nil pointers' in KnownGaps, got %q", inspector.KnownGaps)
	}

	// Verify planner_heuristic is NOT included
	if strings.Contains(inspector.KnownGaps, "Prefer small tasks") {
		t.Errorf("should not include planner_heuristic in KnownGaps, got %q", inspector.KnownGaps)
	}

	// Verify superseded entry is NOT included
	if strings.Contains(inspector.KnownGaps, "Superseded gap") {
		t.Errorf("should not include superseded entries in KnownGaps, got %q", inspector.KnownGaps)
	}
}

func TestExecuteStage_NoCellPath_SkipsPlaybookLoading(t *testing.T) {
	inspector := &specloop.ShellTaskInspector{}

	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done"},
		},
	}

	stage := NewExecuteStage(runner, ExecuteStageConfig{
		MaxRetries: 0,
		CellPath:   "", // Empty CellPath
		Inspector:  inspector,
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "test"},
	}

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify KnownGaps were NOT set
	if inspector.KnownGaps != "" {
		t.Errorf("expected KnownGaps to be empty when CellPath is empty, got %q", inspector.KnownGaps)
	}
}

func TestExecuteStage_NoPlaybookFile_NoError(t *testing.T) {
	tmpDir := t.TempDir()
	// Do NOT create playbook directory or file

	inspector := &specloop.ShellTaskInspector{}

	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done"},
		},
	}

	stage := NewExecuteStage(runner, ExecuteStageConfig{
		MaxRetries: 0,
		CellPath:   tmpDir,
		Inspector:  inspector,
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "test"},
	}

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle gracefully and not set KnownGaps
	if inspector.KnownGaps != "" {
		t.Errorf("expected KnownGaps to be empty when playbook doesn't exist, got %q", inspector.KnownGaps)
	}
}

func TestExecuteStage_NoInspector_NoError(t *testing.T) {
	tmpDir := t.TempDir()
	playbookDir := filepath.Join(tmpDir, "playbook")
	if err := os.MkdirAll(playbookDir, 0755); err != nil {
		t.Fatalf("failed to create playbook dir: %v", err)
	}

	// Create playbook with validation_gap entry
	entries := []playbook.Entry{
		{
			ID:        "pb-001",
			Type:      "validation_gap",
			Title:     "Check for errors",
			Content:   "Always handle errors",
			Status:    "active",
			CreatedAt: time.Now(),
		},
	}
	store := &playbook.Store{Dir: playbookDir}
	if err := store.Save(entries); err != nil {
		t.Fatalf("failed to save playbook entries: %v", err)
	}

	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done"},
		},
	}

	// No Inspector provided
	stage := NewExecuteStage(runner, ExecuteStageConfig{
		MaxRetries: 0,
		CellPath:   tmpDir,
		Inspector:  nil,
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "test"},
	}

	// Should not panic
	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestExecuteStageAllFailedFailureCollection verifies that when all tasks fail,
// failure messages are collected from TaskResult.Failures and propagated to
// FailureContext, including [suspect-proof-check] annotated messages.
func TestExecuteStageAllFailedFailureCollection(t *testing.T) {
	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{
				TaskID: "t-001",
				Status: "failed",
				Failures: []string{
					"[suspect-proof-check] All build checks pass but pattern-matching checks failed. The implementation may be correct; proof checks may be testing source structure rather than behavior. grep -q '--title' cmd/foo.go: exit status 1",
				},
			},
			{
				TaskID: "t-002",
				Status: "failed",
				Failures: []string{
					"[suspect-proof-check] All build checks pass but pattern-matching checks failed. The implementation may be correct; proof checks may be testing source structure rather than behavior. awk '/flagB/' pkg/bar.go: exit status 1",
					"[suspect-proof-check] All build checks pass but pattern-matching checks failed. The implementation may be correct; proof checks may be testing source structure rather than behavior. grep -q 'SubcommandB' cmd/bar.go: exit status 1",
				},
			},
			{
				TaskID: "t-003",
				Status: "failed",
				Failures: []string{
					"go build ./...: exit status 1: undefined: UnfinishedFeature",
				},
			},
		},
	}

	stage := NewExecuteStage(runner, ExecuteStageConfig{MaxRetries: 0})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "first"},
		{TaskID: "t-002", Status: "pending", Objective: "second"},
		{TaskID: "t-003", Status: "pending", Objective: "third"},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}

	if action.Context == nil {
		t.Fatal("expected non-nil FailureContext")
	}

	// Verify that all failure messages are collected
	failures := action.Context.Failures
	if len(failures) != 4 {
		t.Fatalf("expected 4 failures total, got %d: %v", len(failures), failures)
	}

	// Count failures by type
	suspectCount := 0
	buildCount := 0
	for _, f := range failures {
		if strings.HasPrefix(f, "[suspect-proof-check]") {
			suspectCount++
		} else if strings.Contains(f, "go build") {
			buildCount++
		}
	}

	if suspectCount != 3 {
		t.Fatalf("expected 3 suspect-proof-check failures, got %d", suspectCount)
	}
	if buildCount != 1 {
		t.Fatalf("expected 1 build failure, got %d", buildCount)
	}

	// Verify specific failure messages are present
	failuresJoined := strings.Join(failures, "\n")
	expectedPatterns := []string{
		"grep -q '--title'",
		"awk '/flagB/'",
		"SubcommandB",
		"undefined: UnfinishedFeature",
	}
	for _, pattern := range expectedPatterns {
		if !strings.Contains(failuresJoined, pattern) {
			t.Fatalf("expected failure to contain %q, got: %v", pattern, failures)
		}
	}

	// Verify generic fallback is NOT used
	if strings.Contains(failuresJoined, "all tasks failed") {
		t.Fatalf("did not expect generic fallback failure, got: %v", failures)
	}
}
