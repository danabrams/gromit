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

func TestScenario_LocalEntryOverridesGlobalEntry(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Global playbook with entry pb-12345678
	storeRootDir := t.TempDir()
	globalPlaybookDir := filepath.Join(storeRootDir, "global", "playbook")
	os.MkdirAll(globalPlaybookDir, 0o755)
	globalStore := &playbook.Store{Dir: globalPlaybookDir}
	if err := globalStore.Save([]playbook.Entry{
		{
			ID:      "pb-12345678",
			Type:    "planner_heuristic",
			Title:   "UI Task Splitting",
			Content: "Always split UI tasks by component",
			Status:  "active",
		},
	}); err != nil {
		t.Fatalf("save global playbook: %v", err)
	}

	// Local playbook with same ID but different content
	cellPath := t.TempDir()
	localPlaybookDir := filepath.Join(cellPath, "playbook")
	os.MkdirAll(localPlaybookDir, 0o755)
	localStore := &playbook.Store{Dir: localPlaybookDir}
	if err := localStore.Save([]playbook.Entry{
		{
			ID:      "pb-12345678",
			Type:    "planner_heuristic",
			Title:   "UI Task Splitting",
			Content: "Split UI tasks by user flow, not component",
			Status:  "active",
		},
	}); err != nil {
		t.Fatalf("save local playbook: %v", err)
	}

	// Invoke
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

	// Assert
	if len(fp.reqs) != 1 {
		t.Fatalf("expected 1 plan request, got %d", len(fp.reqs))
	}
	heuristics := fp.reqs[0].PlaybookHeuristics

	if !strings.Contains(heuristics, "Split UI tasks by user flow, not component") {
		t.Fatalf("expected local content in planner prompt, got: %q", heuristics)
	}
	if strings.Contains(heuristics, "Always split UI tasks by component") {
		t.Fatalf("global content should NOT appear in planner prompt, got: %q", heuristics)
	}
}
