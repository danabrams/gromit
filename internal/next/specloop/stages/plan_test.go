package stages

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type fakePlanner struct {
	plans []planner.Plan
	errs  []error
	calls int
}

func (f *fakePlanner) CreatePlan(ctx context.Context, req planner.PlanRequest) (planner.Plan, error) {
	idx := f.calls
	f.calls++
	if idx < len(f.errs) && f.errs[idx] != nil {
		return planner.Plan{}, f.errs[idx]
	}
	if idx < len(f.plans) {
		return f.plans[idx], nil
	}
	return planner.Plan{}, errors.New("no more plans")
}

type fakeFixPlanner struct {
	plans []planner.Plan
	errs  []error
	calls int
	reqs  []planner.FixPlanRequest
}

func (f *fakeFixPlanner) CreateFixPlan(ctx context.Context, req planner.FixPlanRequest) (planner.Plan, error) {
	f.reqs = append(f.reqs, req)
	idx := f.calls
	f.calls++
	if idx < len(f.errs) && f.errs[idx] != nil {
		return planner.Plan{}, f.errs[idx]
	}
	if idx < len(f.plans) {
		return f.plans[idx], nil
	}
	return planner.Plan{}, errors.New("no more plans")
}

// Verify PlanStage satisfies the Stage interface.
var _ specloop.Stage = (*PlanStage)(nil)

func validPlan() planner.Plan {
	return planner.Plan{
		SpecID: "spec-001",
		Cycle:  1,
		Kind:   "original",
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-001",
				Objective:           "Do something",
				ExpectedTouchedArea: []string{"pkg/foo"},
				ProofChecks:         []string{"go test ./pkg/foo/..."},
			},
			{
				TaskID:              "t-002",
				Objective:           "Do another thing",
				ExpectedTouchedArea: []string{"pkg/bar"},
				ProofChecks:         []string{"go test ./pkg/bar/..."},
			},
		},
	}
}

func invalidPlan() planner.Plan {
	return planner.Plan{
		Tasks: []planner.TaskDef{}, // empty tasks = invalid
	}
}

func TestPlanStage_CreatesPlanAndTasks(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)

	if stage.Name() != "plan" {
		t.Fatalf("expected name 'plan', got %q", stage.Name())
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if len(rs.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(rs.Tasks))
	}
	if rs.Tasks[0].TaskID != "t-001" {
		t.Fatalf("expected task ID t-001, got %q", rs.Tasks[0].TaskID)
	}

	// Verify plan.md written
	if _, err := os.Stat(filepath.Join(runDir, "plan.md")); os.IsNotExist(err) {
		t.Fatal("plan.md not written")
	}
	// Verify tasks.json written
	if _, err := os.Stat(filepath.Join(runDir, "tasks.json")); os.IsNotExist(err) {
		t.Fatal("tasks.json not written")
	}
}

func TestPlanStage_RetryOnPlanValidationFailure(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// First plan is invalid (empty tasks), second is valid
	fp := &fakePlanner{plans: []planner.Plan{invalidPlan(), validPlan()}}
	stage := NewPlanStage(fp, store, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if fp.calls != 2 {
		t.Fatalf("expected 2 planner calls, got %d", fp.calls)
	}
	if len(rs.Tasks) != 2 {
		t.Fatalf("expected 2 tasks from retry, got %d", len(rs.Tasks))
	}
}

func TestPlanStage_BothRetriesFail_Blocked(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Both plans are invalid
	fp := &fakePlanner{plans: []planner.Plan{invalidPlan(), invalidPlan()}}
	stage := NewPlanStage(fp, store, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked, got %v", action.Kind)
	}
}

func TestPlanStage_Cycle1_TasksAreSet(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 1
	// Pre-populate tasks to verify they get replaced
	rs.Tasks = []runstore.Task{
		{TaskID: "old-task", Status: "done", Cycle: 0},
	}
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	// Cycle 1 should replace, not append
	if len(rs.Tasks) != 2 {
		t.Fatalf("expected 2 tasks (replaced), got %d", len(rs.Tasks))
	}
	if rs.Tasks[0].TaskID != "t-001" {
		t.Fatalf("expected first task t-001, got %q", rs.Tasks[0].TaskID)
	}
}

func TestPlanStage_FixCycle_TasksAreAppended(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.ReplanContext = []string{"test failure in pkg/foo"}
	// Pre-populate with cycle 1 tasks
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
		{TaskID: "t-002", Status: "done", Cycle: 1, Kind: "original"},
	}
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	fixPlan := planner.Plan{
		SpecID: "spec-001",
		Cycle:  2,
		Kind:   "fix",
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-003",
				Objective:           "Fix the failure",
				ExpectedTouchedArea: []string{"pkg/foo"},
				ProofChecks:         []string{"go test ./pkg/foo/..."},
				ParentCycle:         1,
				FailuresAddressed:   []string{"test failure in pkg/foo"},
			},
		},
	}

	ffp := &fakeFixPlanner{plans: []planner.Plan{fixPlan}}
	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)
	stage.SetFixPlanner(ffp)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	// Should have 3 tasks: 2 original + 1 fix appended
	if len(rs.Tasks) != 3 {
		t.Fatalf("expected 3 tasks (2 original + 1 fix), got %d", len(rs.Tasks))
	}
	if rs.Tasks[0].TaskID != "t-001" {
		t.Fatalf("expected first task t-001 (preserved), got %q", rs.Tasks[0].TaskID)
	}
	if rs.Tasks[2].TaskID != "t-003" {
		t.Fatalf("expected third task t-003 (appended), got %q", rs.Tasks[2].TaskID)
	}
	if rs.Tasks[2].Kind != "fix" {
		t.Fatalf("expected fix task kind 'fix', got %q", rs.Tasks[2].Kind)
	}
	if rs.Tasks[2].ParentCycle != 1 {
		t.Fatalf("expected parent_cycle 1, got %d", rs.Tasks[2].ParentCycle)
	}
	if len(rs.Tasks[2].FailuresAddressed) != 1 || rs.Tasks[2].FailuresAddressed[0] != "test failure in pkg/foo" {
		t.Fatalf("expected failures_addressed from fix plan, got %v", rs.Tasks[2].FailuresAddressed)
	}
	// Verify fix planner was called, not regular planner
	if ffp.calls != 1 {
		t.Fatalf("expected 1 fix planner call, got %d", ffp.calls)
	}
	if fp.calls != 0 {
		t.Fatalf("expected 0 regular planner calls, got %d", fp.calls)
	}
}

func TestPlanStage_SpecConstraintsCopiedToTasks(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.SpecConstraints = "## Out-of-Scope\n- Do NOT modify existing tests"
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if len(rs.Tasks) == 0 {
		t.Fatal("expected tasks to be created")
	}
	for _, task := range rs.Tasks {
		if task.SpecConstraints != rs.SpecConstraints {
			t.Fatalf("task %q: expected SpecConstraints %q, got %q",
				task.TaskID, rs.SpecConstraints, task.SpecConstraints)
		}
	}
}

func TestPlanStage_FixCycle_UsesCreatePlanWhenNoFixPlanner(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.ReplanContext = []string{"test failure"}
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
	}
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// No fix planner set, so regular planner is used
	// Even though it's cycle > 1 with replan context, tasks should still be appended
	// because isFixCycle is true based on cycle > 1 && replan context
	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)
	// No SetFixPlanner call -- falls back to CreatePlan

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	// isFixCycle is true (cycle > 1 && replan context), but fixPlanner is nil
	// so it falls through to CreatePlan path. isFixCycle is still true for append logic.
	if len(rs.Tasks) != 3 {
		t.Fatalf("expected 3 tasks (1 original + 2 appended), got %d", len(rs.Tasks))
	}
	if rs.Tasks[0].TaskID != "t-001" {
		t.Fatalf("expected first task preserved, got %q", rs.Tasks[0].TaskID)
	}
}

func TestFilterForbiddenFixTasks_RemovesTestFileTasks(t *testing.T) {
	tasks := []planner.TaskDef{
		{TaskID: "t-002", Objective: "fix test", ExpectedTouchedArea: []string{"calc/divide_test.go"}, ProofChecks: []string{"go test ./..."}},
		{TaskID: "t-003", Objective: "fix impl", ExpectedTouchedArea: []string{"calc/calc.go"}, ProofChecks: []string{"go test ./..."}},
	}
	constraints := "## Out-of-Scope\n- Do NOT modify any existing test files\n"
	result := filterForbiddenFixTasks(tasks, constraints)
	if len(result) != 1 {
		t.Fatalf("expected 1 task after filtering, got %d", len(result))
	}
	if result[0].TaskID != "t-003" {
		t.Fatalf("expected t-003 to survive, got %q", result[0].TaskID)
	}
}

func TestFilterForbiddenFixTasks_AllFilteredWhenAllTargetTestFiles(t *testing.T) {
	tasks := []planner.TaskDef{
		{TaskID: "t-002", Objective: "fix test", ExpectedTouchedArea: []string{"calc/divide_test.go"}, ProofChecks: []string{"go test ./..."}},
	}
	constraints := "## Out-of-Scope\n- Do NOT modify any existing test files\n"
	result := filterForbiddenFixTasks(tasks, constraints)
	if len(result) != 0 {
		t.Fatalf("expected 0 tasks after filtering, got %d", len(result))
	}
}

func TestFilterForbiddenFixTasks_NoConstraint_PassesThrough(t *testing.T) {
	tasks := []planner.TaskDef{
		{TaskID: "t-002", Objective: "fix test", ExpectedTouchedArea: []string{"calc/divide_test.go"}, ProofChecks: []string{"go test ./..."}},
	}
	result := filterForbiddenFixTasks(tasks, "")
	if len(result) != 1 {
		t.Fatalf("expected task to pass through when no constraints, got %d tasks", len(result))
	}
}

func TestPlanStage_FixCycle_CreateFixPlanErrorReturnsContinue(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	rs := &runstore.RunState{RunID: "run-x", Cycle: 2, ReplanContext: []string{"unit-tests: TestAdd failed"}}
	rs.SpecConstraints = "## Out-of-Scope\n- Do NOT modify any existing test files\n"
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	_ = os.MkdirAll(store.RunDir(rs.RunID), 0o755)
	_ = os.WriteFile(filepath.Join(store.RunDir(rs.RunID), "spec-packet.md"), []byte("spec"), 0o644)

	// Fix planner returns an error (e.g. LLM couldn't produce valid plan after retries)
	stage := NewPlanStage(nil, store, nil)
	stage.SetFixPlanner(&fakeFixPlanner{
		errs: []error{errors.New("fix plan generation failed after 2 attempts: plan validation failed: task t-002: missing expected_touched_area")},
	})

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue when CreateFixPlan errors, got %v", action.Kind)
	}
	if len(rs.Tasks) != 1 {
		t.Fatalf("expected 1 task (no new tasks added), got %d", len(rs.Tasks))
	}
}

func TestPlanStage_FixCycle_InvalidPlanAfterRetriesReturnsContinue(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	rs := &runstore.RunState{RunID: "run-x", Cycle: 2, ReplanContext: []string{"unit-tests: TestAdd failed"}}
	rs.SpecConstraints = "## Out-of-Scope\n- Do NOT modify any existing test files\n"
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	_ = os.MkdirAll(store.RunDir(rs.RunID), 0o755)
	_ = os.WriteFile(filepath.Join(store.RunDir(rs.RunID), "spec-packet.md"), []byte("spec"), 0o644)

	// Fix planner returns a structurally invalid plan (missing expected_touched_area) on both attempts
	invalidPlan := planner.Plan{
		Kind:  "fix",
		Cycle: 2,
		Tasks: []planner.TaskDef{
			{TaskID: "t-002", Objective: "fix something", ProofChecks: []string{"go test ./..."}},
			// ExpectedTouchedArea intentionally omitted — ValidatePlan will reject this
		},
	}
	stage := NewPlanStage(nil, store, nil)
	stage.SetFixPlanner(&fakeFixPlanner{plans: []planner.Plan{invalidPlan, invalidPlan}})

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue when fix plan tasks are invalid after retries, got %v", action.Kind)
	}
	if len(rs.Tasks) != 1 {
		t.Fatalf("expected 1 task (no new tasks added), got %d", len(rs.Tasks))
	}
}

func TestPlanStage_FixCycle_AllTasksFilteredReturnsContinue(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	rs := &runstore.RunState{RunID: "run-x", Cycle: 2, ReplanContext: []string{"unit-tests: format %d"}}
	rs.SpecConstraints = "## Out-of-Scope\n- Do NOT modify any existing test files\n"
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	_ = os.MkdirAll(store.RunDir(rs.RunID), 0o755)
	_ = os.WriteFile(filepath.Join(store.RunDir(rs.RunID), "spec-packet.md"), []byte("spec"), 0o644)

	// Fix planner returns a plan that ONLY targets test files (will be filtered out)
	fixPlan := planner.Plan{
		Kind:  "fix",
		Cycle: 2,
		Tasks: []planner.TaskDef{
			{TaskID: "t-002", Objective: "fix test file", ExpectedTouchedArea: []string{"calc/divide_test.go"}, ProofChecks: []string{"go test ./..."}},
		},
	}
	stage := NewPlanStage(nil, store, nil)
	stage.SetFixPlanner(&fakeFixPlanner{plans: []planner.Plan{fixPlan}})

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue when all fix tasks filtered, got %v", action.Kind)
	}
	// No new tasks should have been added
	if len(rs.Tasks) != 1 {
		t.Fatalf("expected 1 task (no new tasks added), got %d", len(rs.Tasks))
	}
}
