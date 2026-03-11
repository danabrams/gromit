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
