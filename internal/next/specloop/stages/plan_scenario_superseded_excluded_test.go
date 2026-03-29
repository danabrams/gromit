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
)

func TestScenario_SupersededEntriesExcludedFromPrompts(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.ReplanContext = &runstore.ReplanContext{Failures: []string{}}
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("build a calculator"), 0o644); err != nil {
		t.Fatalf("write spec-packet.md: %v", err)
	}

	// Create a cell path with a playbook containing both active and superseded heuristics
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")
	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		t.Fatalf("mkdir playbookDir: %v", err)
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	entries := []playbook.Entry{
		{
			ID:               "pb-active01",
			Type:             "planner_heuristic",
			Title:            "Active heuristic stays visible",
			Content:          "Each task should touch at most 3 files for manageable reviews",
			Rationale:        "Smaller diffs are easier to review",
			Status:           "active",
			SourceProposalID: "prop-001",
			SourceRunID:      "run-001",
			SourceSpecID:     "spec-001",
			SupersededBy:     "",
		},
		{
			ID:               "pb-super01",
			Type:             "planner_heuristic",
			Title:            "Superseded heuristic must not appear",
			Content:          "Old guidance that was replaced by a better approach",
			Rationale:        "This rationale should be invisible in the prompt",
			Status:           "superseded",
			SourceProposalID: "prop-002",
			SourceRunID:      "run-002",
			SourceSpecID:     "spec-002",
			SupersededBy:     "prop-003",
		},
	}
	if err := pbStore.Save(entries); err != nil {
		t.Fatalf("save playbook entries: %v", err)
	}

	// Set up the PlanStage with a fake planner that captures requests
	validPlan := planner.Plan{
		SpecID: "spec-001",
		Cycle:  1,
		Kind:   "original",
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-001",
				Objective:           "implement calculator",
				ExpectedTouchedArea: []string{"calc/calc.go"},
				ProofChecks:         []string{"go build ./...", "go test ./calc/..."},
			},
		},
	}
	fp := &fakePlanner{plans: []planner.Plan{validPlan}}
	stage := NewPlanStage(fp, store, nil)
	stage.SetCellPathResolver(&fakeCellPathResolver{path: cellPath})

	// === Invoke ===
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("PlanStage.Run: %v", err)
	}

	// === Assert ===

	// 1. Stage completed successfully
	if action.Kind != 0 { // specloop.Continue == 0
		t.Fatalf("expected Continue action, got %v", action.Kind)
	}

	// 2. Planner was called exactly once
	if len(fp.reqs) != 1 {
		t.Fatalf("expected 1 plan request, got %d", len(fp.reqs))
	}
	req := fp.reqs[0]

	// 3. Active heuristic appears in the prompt
	if !strings.Contains(req.PlaybookHeuristics, "Active heuristic stays visible") {
		t.Errorf("PlaybookHeuristics should contain active heuristic title, got:\n%s", req.PlaybookHeuristics)
	}

	// 4. Active heuristic content appears in the prompt
	if !strings.Contains(req.PlaybookHeuristics, "Each task should touch at most 3 files") {
		t.Errorf("PlaybookHeuristics should contain active heuristic content, got:\n%s", req.PlaybookHeuristics)
	}

	// 5. Superseded heuristic title does NOT appear in the prompt
	if strings.Contains(req.PlaybookHeuristics, "Superseded heuristic must not appear") {
		t.Error("PlaybookHeuristics should NOT contain superseded heuristic title")
	}

	// 6. Superseded heuristic content does NOT appear in the prompt
	if strings.Contains(req.PlaybookHeuristics, "Old guidance that was replaced") {
		t.Error("PlaybookHeuristics should NOT contain superseded heuristic content")
	}

	// 7. Superseded heuristic rationale does NOT appear in the prompt
	if strings.Contains(req.PlaybookHeuristics, "This rationale should be invisible") {
		t.Error("PlaybookHeuristics should NOT contain superseded heuristic rationale")
	}

	// 8. Tasks were created from the plan
	if len(rs.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(rs.Tasks))
	}
	if rs.Tasks[0].TaskID != "t-001" {
		t.Errorf("expected task ID t-001, got %q", rs.Tasks[0].TaskID)
	}
}
