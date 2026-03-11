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
	stage := NewPlanStage(fp, store)

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
	stage := NewPlanStage(fp, store)

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
	stage := NewPlanStage(fp, store)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked, got %v", action.Kind)
	}
}
