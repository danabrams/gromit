package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_NewProjectInheritsGlobalEntries(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-new")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Create 3 global playbook entries: 2 planner_heuristic + 1 validation_gap
	storeRootDir := t.TempDir()
	globalPlaybookDir := filepath.Join(storeRootDir, "global", "playbook")
	os.MkdirAll(globalPlaybookDir, 0o755)
	globalStore := &playbook.Store{Dir: globalPlaybookDir}
	globalEntries := []playbook.Entry{
		{
			ID:      "pb-global-h1",
			Type:    "planner_heuristic",
			Title:   "Task Size Limit",
			Content: "Keep each task to at most 3-4 files",
			Status:  "active",
		},
		{
			ID:      "pb-global-h2",
			Type:    "planner_heuristic",
			Title:   "Parallel Decomposition",
			Content: "Identify independent subtasks for parallel execution",
			Status:  "active",
		},
		{
			ID:      "pb-global-v1",
			Type:    "validation_gap",
			Title:   "Missing Boundary Check",
			Content: "Validate array bounds before access",
			Status:  "active",
		},
	}
	if err := globalStore.Save(globalEntries); err != nil {
		t.Fatalf("save global playbook: %v", err)
	}

	// New project: local playbook directory exists but has no entries
	cellPath := t.TempDir()
	localPlaybookDir := filepath.Join(cellPath, "playbook")
	os.MkdirAll(localPlaybookDir, 0o755)

	// === Invoke ===
	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)
	stage.SetCellPathResolver(&fakeCellPathResolver{path: cellPath})
	stage.SetStoreRootDir(storeRootDir)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("PlanStage.Run: %v", err)
	}

	// === Assert ===
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	if len(fp.reqs) != 1 {
		t.Fatalf("expected 1 plan request, got %d", len(fp.reqs))
	}
	req := fp.reqs[0]

	// Both planner_heuristic entries should appear in PlaybookHeuristics
	if !strings.Contains(req.PlaybookHeuristics, "Task Size Limit") {
		t.Errorf("expected 'Task Size Limit' in PlaybookHeuristics, got: %q", req.PlaybookHeuristics)
	}
	if !strings.Contains(req.PlaybookHeuristics, "Parallel Decomposition") {
		t.Errorf("expected 'Parallel Decomposition' in PlaybookHeuristics, got: %q", req.PlaybookHeuristics)
	}

	// Verify the content is also rendered (not just titles)
	if !strings.Contains(req.PlaybookHeuristics, "Keep each task to at most 3-4 files") {
		t.Errorf("expected heuristic content in PlaybookHeuristics, got: %q", req.PlaybookHeuristics)
	}
	if !strings.Contains(req.PlaybookHeuristics, "Identify independent subtasks for parallel execution") {
		t.Errorf("expected heuristic content in PlaybookHeuristics, got: %q", req.PlaybookHeuristics)
	}

	// validation_gap entries are loaded by MergedPlaybook but filtered by type
	// in loadPlaybookAndDoctrine — they do not appear in PlaybookHeuristics or RefinementGuidance
	if strings.Contains(req.PlaybookHeuristics, "Missing Boundary Check") {
		t.Errorf("validation_gap entry should not appear in PlaybookHeuristics")
	}
	if req.RefinementGuidance != "" {
		t.Errorf("expected empty RefinementGuidance (no refinement_guidance entries), got: %q", req.RefinementGuidance)
	}

	// Verify that the merge itself succeeded (no error, plan was created)
	if len(rs.Tasks) != 2 {
		t.Fatalf("expected 2 tasks from plan, got %d", len(rs.Tasks))
	}
}
