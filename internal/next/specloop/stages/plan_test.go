package stages

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type fakePlanner struct {
	plans []planner.Plan
	errs  []error
	calls int
	reqs  []planner.PlanRequest
}

func (f *fakePlanner) CreatePlan(ctx context.Context, req planner.PlanRequest) (planner.Plan, error) {
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

func planWithArchitectureDecisions(decisions []string) planner.Plan {
	p := validPlan()
	p.ArchitectureDecisions = decisions
	return p
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

func TestPlanStage_FixCycle_FixesCopiedToTasks(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.ReplanContext = []string{"test failure in pkg/foo"}
	// Pre-populate with cycle 1 tasks
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
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
				TaskID:              "t-002",
				Objective:           "Fix the failure",
				ExpectedTouchedArea: []string{"pkg/foo"},
				ProofChecks:         []string{"go test ./pkg/foo/..."},
				ParentCycle:         1,
				FailuresAddressed:   []string{"test failure in pkg/foo"},
				Fixes:               "issue-001, issue-002",
			},
			{
				TaskID:              "t-003",
				Objective:           "Additional fix",
				ExpectedTouchedArea: []string{"pkg/bar"},
				ProofChecks:         []string{"go test ./pkg/bar/..."},
				ParentCycle:         1,
				FailuresAddressed:   []string{"test failure in pkg/foo"},
				Fixes:               "issue-003",
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
	// Should have 3 tasks: 1 original + 2 fix appended
	if len(rs.Tasks) != 3 {
		t.Fatalf("expected 3 tasks (1 original + 2 fix), got %d", len(rs.Tasks))
	}

	// Verify Fixes are copied to first fix task (t-002)
	if rs.Tasks[1].Fixes == "issue-001, issue-002" {
		// Verified: Fixes matches expected value
	} else {
		t.Fatalf("task t-002: expected Fixes 'issue-001, issue-002', got %q", rs.Tasks[1].Fixes)
	}

	// Verify Fixes are copied to second fix task (t-003)
	if rs.Tasks[2].Fixes == "issue-003" {
		// Verified: Fixes matches expected value
	} else {
		t.Fatalf("task t-003: expected Fixes 'issue-003', got %q", rs.Tasks[2].Fixes)
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

func TestPlanStage_FixesCopiedToTasks(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Create a plan with tasks that have Fixes populated
	planWithFixes := planner.Plan{
		SpecID: "spec-001",
		Cycle:  1,
		Kind:   "original",
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-001",
				Objective:           "Do something",
				ExpectedTouchedArea: []string{"pkg/foo"},
				ProofChecks:         []string{"go test ./pkg/foo/..."},
				Fixes:               "issue-123, issue-456",
			},
			{
				TaskID:              "t-002",
				Objective:           "Do another thing",
				ExpectedTouchedArea: []string{"pkg/bar"},
				ProofChecks:         []string{"go test ./pkg/bar/..."},
				Fixes:               "issue-789",
			},
		},
	}

	fp := &fakePlanner{plans: []planner.Plan{planWithFixes}}
	stage := NewPlanStage(fp, store, nil)

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

	// Verify Fixes are copied to first task
	if rs.Tasks[0].Fixes == "issue-123, issue-456" {
		// Verified: Fixes matches expected value
	} else {
		t.Fatalf("task t-001: expected Fixes 'issue-123, issue-456', got %q", rs.Tasks[0].Fixes)
	}

	// Verify Fixes are copied to second task
	if rs.Tasks[1].Fixes == "issue-789" {
		// Verified: Fixes matches expected value
	} else {
		t.Fatalf("task t-002: expected Fixes 'issue-789', got %q", rs.Tasks[1].Fixes)
	}
}

func TestPlanStage_FixCycle_PopulatesCurrentDiffAndCompletedTasks(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.ReplanContext = []string{"test failure in pkg/foo"}

	// Set up a git repo as the worktree so git diff HEAD works
	worktree := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = worktree
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	// Create a file and leave it as an unstaged change so git diff HEAD shows it
	os.WriteFile(filepath.Join(worktree, "new.go"), []byte("package main\n"), 0o644)
	cmd := exec.Command("git", "add", "new.go")
	cmd.Dir = worktree
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %s: %v", out, err)
	}

	rs.WorktreePath = worktree

	// Pre-populate with cycle 1 tasks — one done, one failed
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original", Objective: "Build foo", Attempts: 1, FilesChanged: []string{"pkg/foo/foo.go"}},
		{TaskID: "t-002", Status: "failed", Cycle: 1, Kind: "original", Objective: "Build bar", Attempts: 2},
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

	// Verify the fix planner received CompletedTasks
	if len(ffp.reqs) != 1 {
		t.Fatalf("expected 1 fix planner call, got %d", len(ffp.reqs))
	}
	req := ffp.reqs[0]

	// Only "done" task should appear
	if len(req.CompletedTasks) != 1 {
		t.Fatalf("expected 1 completed task, got %d", len(req.CompletedTasks))
	}
	ct := req.CompletedTasks[0]
	if ct.TaskID != "t-001" {
		t.Fatalf("expected completed task t-001, got %q", ct.TaskID)
	}
	if ct.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", ct.Attempts)
	}
	if len(ct.FilesChanged) != 1 || ct.FilesChanged[0] != "pkg/foo/foo.go" {
		t.Fatalf("expected FilesChanged [pkg/foo/foo.go], got %v", ct.FilesChanged)
	}
	if ct.ValidationOutcome != "done" {
		t.Fatalf("expected ValidationOutcome 'done', got %q", ct.ValidationOutcome)
	}

	// Verify CurrentDiff is populated (should contain our staged new.go)
	if req.CurrentDiff == "" {
		t.Fatal("expected CurrentDiff to be populated from worktree git diff")
	}
	if !strings.Contains(req.CurrentDiff, "new.go") {
		t.Fatalf("expected CurrentDiff to mention new.go, got: %s", req.CurrentDiff)
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

type fakeCellPathResolver struct {
	path string
}

func (f *fakeCellPathResolver) ResolveCellPath(projectID string) (string, error) {
	return f.path, nil
}

func TestPlanStage_PlaybookHeuristics_PopulatedWhenEntryExists(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Create a playbook store with planner_heuristic entries
	playbookDir := t.TempDir()
	pb := &playbook.Store{Dir: playbookDir}
	entries := []playbook.Entry{
		{
			ID:        "pb-001",
			Type:      "planner_heuristic",
			Title:     "Task Granularity",
			Content:   "Each task should touch at most 3-4 files",
			Rationale: "Smaller tasks reduce complexity",
			Status:    "active",
		},
		{
			ID:        "pb-002",
			Type:      "planner_heuristic",
			Title:     "Parallel Work",
			Content:   "Identify opportunities for parallel task execution",
			Rationale: "Improves planning efficiency",
			Status:    "active",
		},
	}
	if err := pb.Save(entries); err != nil {
		t.Fatalf("failed to save playbook: %v", err)
	}

	// Set up PlanStage with resolver that points to our playbook
	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)
	stage.SetCellPathResolver(&fakeCellPathResolver{path: t.TempDir()})

	// We need to actually put the playbook where the resolver points to
	cellPath := t.TempDir()
	playbookDirReal := filepath.Join(cellPath, "playbook")
	os.MkdirAll(playbookDirReal, 0o755)
	pbReal := &playbook.Store{Dir: playbookDirReal}
	if err := pbReal.Save(entries); err != nil {
		t.Fatalf("failed to save playbook in cell: %v", err)
	}
	stage.SetCellPathResolver(&fakeCellPathResolver{path: cellPath})

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify the PlanRequest was populated with heuristics
	if len(fp.reqs) != 1 {
		t.Fatalf("expected 1 plan request, got %d", len(fp.reqs))
	}
	req := fp.reqs[0]

	// The PlaybookHeuristics should contain formatted heuristics
	expectedHeuristics := playbook.FormatPlaybookForPrompt(entries)
	if req.PlaybookHeuristics != expectedHeuristics {
		t.Fatalf("expected PlaybookHeuristics %q, got %q", expectedHeuristics, req.PlaybookHeuristics)
	}

	// Verify the formatted heuristics include the titles and content
	if !strings.Contains(req.PlaybookHeuristics, "Task Granularity") {
		t.Fatalf("expected 'Task Granularity' in PlaybookHeuristics, got: %q", req.PlaybookHeuristics)
	}
	if !strings.Contains(req.PlaybookHeuristics, "Parallel Work") {
		t.Fatalf("expected 'Parallel Work' in PlaybookHeuristics, got: %q", req.PlaybookHeuristics)
	}
}

func TestPlanStage_PlaybookHeuristics_EmptyWhenNoEntries(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Create a playbook store with NO entries
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")
	os.MkdirAll(playbookDir, 0o755)
	pb := &playbook.Store{Dir: playbookDir}
	if err := pb.Save([]playbook.Entry{}); err != nil {
		t.Fatalf("failed to save empty playbook: %v", err)
	}

	// Set up PlanStage with resolver pointing to the empty playbook
	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)
	stage.SetCellPathResolver(&fakeCellPathResolver{path: cellPath})

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify the PlanRequest was populated with empty heuristics
	if len(fp.reqs) != 1 {
		t.Fatalf("expected 1 plan request, got %d", len(fp.reqs))
	}
	req := fp.reqs[0]

	if req.PlaybookHeuristics != "" {
		t.Fatalf("expected empty PlaybookHeuristics, got %q", req.PlaybookHeuristics)
	}
}

func TestPlanStage_MergedPlaybook_LocalWinsForOverlappingIDAndIncludesAllNonOverlapping(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Create global playbook entries: 3 heuristics with IDs pb-global-1, pb-global-2, pb-global-3
	// Global playbook lives at storeRootDir/global/playbook
	storeRootDir := t.TempDir()
	globalPlaybookDir := filepath.Join(storeRootDir, "global", "playbook")
	os.MkdirAll(globalPlaybookDir, 0o755)
	globalStore := &playbook.Store{Dir: globalPlaybookDir}
	globalEntries := []playbook.Entry{
		{
			ID:      "pb-global-1",
			Type:    "planner_heuristic",
			Title:   "Global Heuristic 1",
			Content: "Global content 1",
			Status:  "active",
		},
		{
			ID:      "pb-global-2",
			Type:    "planner_heuristic",
			Title:   "Global Heuristic 2",
			Content: "Global content 2",
			Status:  "active",
		},
		{
			ID:      "pb-global-3",
			Type:    "planner_heuristic",
			Title:   "Global Heuristic 3",
			Content: "Global content 3",
			Status:  "active",
		},
	}
	if err := globalStore.Save(globalEntries); err != nil {
		t.Fatalf("failed to save global playbook: %v", err)
	}

	// Create local playbook entries:
	// - pb-global-2 with different content (should override global)
	// - pb-local-4 new local entry (should be included)
	cellPath := t.TempDir()
	localPlaybookDir := filepath.Join(cellPath, "playbook")
	os.MkdirAll(localPlaybookDir, 0o755)
	localStore := &playbook.Store{Dir: localPlaybookDir}
	localEntries := []playbook.Entry{
		{
			ID:      "pb-global-2",
			Type:    "planner_heuristic",
			Title:   "Local Override for Heuristic 2",
			Content: "Local override content 2",
			Status:  "active",
		},
		{
			ID:      "pb-local-4",
			Type:    "planner_heuristic",
			Title:   "Local Heuristic 4",
			Content: "Local content 4",
			Status:  "active",
		},
	}
	if err := localStore.Save(localEntries); err != nil {
		t.Fatalf("failed to save local playbook: %v", err)
	}

	// Set up PlanStage with merged playbook loading
	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)
	stage.SetCellPathResolver(&fakeCellPathResolver{path: cellPath})
	stage.SetStoreRootDir(storeRootDir)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify the PlanRequest was populated with merged heuristics
	if len(fp.reqs) != 1 {
		t.Fatalf("expected 1 plan request, got %d", len(fp.reqs))
	}
	req := fp.reqs[0]

	// The PlaybookHeuristics should contain:
	// - pb-global-1 (non-overlapping global entry)
	// - pb-global-2 but with LOCAL content (local wins for overlapping IDs)
	// - pb-global-3 (non-overlapping global entry)
	// - pb-local-4 (new local entry)
	if !strings.Contains(req.PlaybookHeuristics, "Global Heuristic 1") {
		t.Fatalf("expected 'Global Heuristic 1' (non-overlapping global) in merged heuristics, got: %q", req.PlaybookHeuristics)
	}
	if !strings.Contains(req.PlaybookHeuristics, "Local Override for Heuristic 2") {
		t.Fatalf("expected 'Local Override for Heuristic 2' (local version of overlapping entry) in merged heuristics, got: %q", req.PlaybookHeuristics)
	}
	if strings.Contains(req.PlaybookHeuristics, "Global Heuristic 2") && !strings.Contains(req.PlaybookHeuristics, "Local Override for Heuristic 2") {
		t.Fatalf("expected LOCAL version of Heuristic 2 to override GLOBAL version in merged heuristics, got: %q", req.PlaybookHeuristics)
	}
	if !strings.Contains(req.PlaybookHeuristics, "Global Heuristic 3") {
		t.Fatalf("expected 'Global Heuristic 3' (non-overlapping global) in merged heuristics, got: %q", req.PlaybookHeuristics)
	}
	if !strings.Contains(req.PlaybookHeuristics, "Local Heuristic 4") {
		t.Fatalf("expected 'Local Heuristic 4' (new local entry) in merged heuristics, got: %q", req.PlaybookHeuristics)
	}
}

func TestPlanStage_InitialPlan_RetryPopulatesArchitectureConstraints(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// First plan is invalid (empty tasks), second is valid with ArchitectureDecisions
	decisions := []string{"Config.Tier always receives a tier label"}
	fp := &fakePlanner{
		plans: []planner.Plan{
			invalidPlan(),
			planWithArchitectureDecisions(decisions),
		},
	}
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

	// Verify rs.ArchitectureConstraints matches the valid plan's ArchitectureDecisions
	if len(rs.ArchitectureConstraints) != len(decisions) {
		t.Fatalf("expected %d architecture constraints, got %d", len(decisions), len(rs.ArchitectureConstraints))
	}
	for i, expected := range decisions {
		if rs.ArchitectureConstraints[i] != expected {
			t.Fatalf("expected constraint[%d] %q, got %q", i, expected, rs.ArchitectureConstraints[i])
		}
	}
}

func TestPlanStage_InitialPlan_NilDecisions_SetsEmptyConstraints(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Plan with nil ArchitectureDecisions (LLM omitted the field)
	fp := &fakePlanner{
		plans: []planner.Plan{validPlan()},
	}
	stage := NewPlanStage(fp, store, nil)

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// nil ArchitectureDecisions must result in []string{} not nil
	if rs.ArchitectureConstraints == nil {
		t.Fatal("expected rs.ArchitectureConstraints to be []string{}, got nil")
	}
	if len(rs.ArchitectureConstraints) != 0 {
		t.Fatalf("expected 0 architecture constraints, got %d: %v", len(rs.ArchitectureConstraints), rs.ArchitectureConstraints)
	}
}

func TestPlanStage_FixCycle_DedupArchitectureDecisions(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.ReplanContext = []string{"test failure in pkg/foo"}
	// Pre-populate with cycle 1 tasks
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
	}
	// Pre-populate with an existing constraint
	rs.ArchitectureConstraints = []string{"Config.Tier always receives a tier label"}
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Fix planner returns a plan with the SAME constraint plus a new one
	fixPlan := planner.Plan{
		SpecID: "spec-001",
		Cycle:  2,
		Kind:   "fix",
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-002",
				Objective:           "Fix the failure",
				ExpectedTouchedArea: []string{"pkg/foo"},
				ProofChecks:         []string{"go test ./pkg/foo/..."},
				ParentCycle:         1,
				FailuresAddressed:   []string{"test failure in pkg/foo"},
			},
		},
		// ArchitectureDecisions includes the same constraint + one new constraint
		ArchitectureDecisions: []string{
			"Config.Tier always receives a tier label",                          // Already exists
			"LLMCompleter.Complete must use context.Context as first parameter", // New
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

	// Verify the fix planner request received the pre-existing ArchitectureConstraints
	if len(ffp.reqs) < 1 {
		t.Fatalf("expected at least 1 fix planner request, got %d", len(ffp.reqs))
	}
	if len(ffp.reqs[0].ArchitectureConstraints) != 1 {
		t.Fatalf("expected fix planner request to have 1 ArchitectureConstraint, got %d: %v", len(ffp.reqs[0].ArchitectureConstraints), ffp.reqs[0].ArchitectureConstraints)
	}
	if ffp.reqs[0].ArchitectureConstraints[0] != "Config.Tier always receives a tier label" {
		t.Fatalf("expected fix planner request constraint 'Config.Tier always receives a tier label', got %q", ffp.reqs[0].ArchitectureConstraints[0])
	}

	// Verify rs.ArchitectureConstraints has exactly 2 entries (deduped: existing + new)
	if len(rs.ArchitectureConstraints) != 2 {
		t.Fatalf("expected 2 architecture constraints after dedup, got %d: %v", len(rs.ArchitectureConstraints), rs.ArchitectureConstraints)
	}

	// Verify the first constraint is the original one
	if rs.ArchitectureConstraints[0] != "Config.Tier always receives a tier label" {
		t.Fatalf("expected first constraint 'Config.Tier always receives a tier label', got %q", rs.ArchitectureConstraints[0])
	}

	// Verify the second constraint is the new one
	if rs.ArchitectureConstraints[1] != "LLMCompleter.Complete must use context.Context as first parameter" {
		t.Fatalf("expected second constraint 'LLMCompleter.Complete must use context.Context as first parameter', got %q", rs.ArchitectureConstraints[1])
	}
}

func TestPlanStage_FixCycle_AllFiltered_ArchitectureDecisions(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.ReplanContext = []string{"test failure in pkg/foo"}
	rs.SpecConstraints = "## Out-of-Scope\n- Do NOT modify any existing test files\n"
	// Pre-populate with cycle 1 tasks
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
	}
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Fix planner returns a plan with a task that will be filtered and ArchitectureDecisions
	fixPlan := planner.Plan{
		SpecID: "spec-001",
		Cycle:  2,
		Kind:   "fix",
		Tasks:  []planner.TaskDef{{TaskID: "t-002", Objective: "fix test", ExpectedTouchedArea: []string{"internal/pkg/foo_test.go"}, ProofChecks: []string{"go build ./..."}}},
		ArchitectureDecisions: []string{
			"NewConstraint: all fix tasks were filtered but the convention still applies",
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

	// Verify the fix planner request received empty ArchitectureConstraints (not pre-populated in this test)
	if len(ffp.reqs) < 1 {
		t.Fatalf("expected at least 1 fix planner request, got %d", len(ffp.reqs))
	}
	if len(ffp.reqs[0].ArchitectureConstraints) != 0 {
		t.Fatalf("expected fix planner request to have 0 ArchitectureConstraints, got %d: %v", len(ffp.reqs[0].ArchitectureConstraints), ffp.reqs[0].ArchitectureConstraints)
	}

	// Verify rs.ArchitectureConstraints contains the new constraint even though allFiltered=true
	if len(rs.ArchitectureConstraints) != 1 {
		t.Fatalf("expected 1 architecture constraint (allFiltered case), got %d: %v", len(rs.ArchitectureConstraints), rs.ArchitectureConstraints)
	}
	if rs.ArchitectureConstraints[0] != "NewConstraint: all fix tasks were filtered but the convention still applies" {
		t.Fatalf("expected constraint 'NewConstraint: all fix tasks were filtered but the convention still applies', got %q", rs.ArchitectureConstraints[0])
	}

	// Verify no new tasks were added (all were filtered)
	if len(rs.Tasks) != 1 {
		t.Fatalf("expected 1 task (no new tasks added), got %d", len(rs.Tasks))
	}
}

func TestPlanStage_FixCycle_LLMError_NoArchConstraintsAccumulated(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.ReplanContext = []string{"test failure in pkg/foo"}
	// Pre-populate with cycle 1 tasks
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
	}
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	ffp := &fakeFixPlanner{errs: []error{errors.New("LLM API error")}}
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
	if len(rs.ArchitectureConstraints) != 0 {
		t.Fatalf("expected 0 architecture constraints (LLM error case), got %d: %v", len(rs.ArchitectureConstraints), rs.ArchitectureConstraints)
	}
}

func TestPlanStage_InitialPlan_ValidationFailure_ConstraintsNotSet(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 1
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Planner returns invalid plans both times (validation will fail)
	fp := &fakePlanner{plans: []planner.Plan{invalidPlan(), invalidPlan()}}
	stage := NewPlanStage(fp, store, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked when plan validation fails after retries, got %v", action.Kind)
	}

	// Verify rs.ArchitectureConstraints is nil or empty (not set on validation failure)
	if len(rs.ArchitectureConstraints) != 0 {
		t.Fatalf("expected empty ArchitectureConstraints on validation failure, got %d entries: %v", len(rs.ArchitectureConstraints), rs.ArchitectureConstraints)
	}
}

func TestPlanStage_FixCycle_ValidationFailure_ArchitectureDecisionsNotAccumulated(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.ReplanContext = []string{"test failure in pkg/foo"}
	// Pre-populate with cycle 1 tasks
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
	}
	// Pre-populate with an existing constraint
	rs.ArchitectureConstraints = []string{"Config.Tier always receives a tier label"}
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Fix planner returns plans with tasks but missing required fields (Objective)
	// This will fail ValidatePlan but pass the empty task check
	invalidFixPlan := planner.Plan{
		SpecID: "spec-001",
		Cycle:  2,
		Kind:   "fix",
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-002",
				Objective:           "", // Missing objective - will fail validation
				ExpectedTouchedArea: []string{"pkg/foo"},
				ProofChecks:         []string{"go test ./pkg/foo/..."},
			},
		},
		ArchitectureDecisions: []string{
			"InvalidPlanConstraint: should NOT be accumulated",
		},
	}
	ffp := &fakeFixPlanner{plans: []planner.Plan{invalidFixPlan, invalidFixPlan}}
	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)
	stage.SetFixPlanner(ffp)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue (no viable fix this cycle), got %v", action.Kind)
	}

	// Verify the fix planner was called twice (both retries failed validation)
	if ffp.calls != 2 {
		t.Fatalf("expected 2 fix planner calls (initial + 1 retry), got %d", ffp.calls)
	}

	// Verify the retry request (ffp.reqs[1]) contains the validation error from attempt 0 in Failures
	if len(ffp.reqs) < 2 {
		t.Fatalf("expected at least 2 fix planner requests, got %d", len(ffp.reqs))
	}
	// ffp.reqs[1].Failures should contain the original ReplanContext plus the validation error from attempt 0
	if len(ffp.reqs[1].Failures) < 2 {
		t.Fatalf("expected retry request to contain validation error from attempt 0 in Failures, got %d entries: %v", len(ffp.reqs[1].Failures), ffp.reqs[1].Failures)
	}
	// The validation error should be the last entry (appended after the original ReplanContext)
	validationErrorMsg := ffp.reqs[1].Failures[len(ffp.reqs[1].Failures)-1]
	if !strings.Contains(validationErrorMsg, "t-002") || !strings.Contains(validationErrorMsg, "objective") {
		t.Fatalf("expected retry request to contain validation error mentioning 't-002' and 'objective', got %q", validationErrorMsg)
	}

	// Verify rs.ArchitectureConstraints still has ONLY the original constraint
	// The invalid plan's ArchitectureDecisions must NOT be accumulated
	if len(rs.ArchitectureConstraints) != 1 {
		t.Fatalf("expected 1 architecture constraint (original only), got %d: %v", len(rs.ArchitectureConstraints), rs.ArchitectureConstraints)
	}
	if rs.ArchitectureConstraints[0] != "Config.Tier always receives a tier label" {
		t.Fatalf("expected original constraint 'Config.Tier always receives a tier label', got %q", rs.ArchitectureConstraints[0])
	}
}
