package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_PlanStage_MissingReviewOutcomeIsSilent(t *testing.T) {
	// Seed
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	rs := &runstore.RunState{
		RunID:   "run-missing-review-outcome",
		Cycle:   2,
		Resumed: true,
		ReplanContext: &runstore.ReplanContext{
			Failures: []string{"validation failed"},
		},
		Tasks: []runstore.Task{{TaskID: "t-001", Status: "done"}},
	}
	if err := os.MkdirAll(store.RunDir(rs.RunID), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.RunDir(rs.RunID), "spec-packet.md"), []byte("spec"), 0o644); err != nil {
		t.Fatalf("write spec-packet.md: %v", err)
	}
	if err := os.MkdirAll(store.RunEvidenceDir(rs.RunID), 0o755); err != nil {
		t.Fatalf("mkdir evidence dir: %v", err)
	}
	// Intentionally do not create review-outcome.json.

	fixPlan := planner.Plan{
		Kind:  "fix",
		Cycle: 2,
		Tasks: []planner.TaskDef{{
			TaskID:              "t-002",
			Objective:           "Apply fix",
			ExpectedTouchedArea: []string{"pkg/foo"},
			ProofChecks:         []string{"go test ./pkg/foo/..."},
			ParentCycle:         1,
			FailuresAddressed:   []string{"validation failed"},
		}},
	}

	ffp := &fakeFixPlanner{plans: []planner.Plan{fixPlan}}
	stage := NewPlanStage(nil, store, nil)
	stage.SetFixPlanner(ffp)

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if len(ffp.reqs) != 1 {
		t.Fatalf("expected 1 fix planner request, got %d", len(ffp.reqs))
	}
}
