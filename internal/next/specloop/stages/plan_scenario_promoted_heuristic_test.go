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

func TestScenario_PromotedHeuristicAppearsInPlannerPrompt(t *testing.T) {
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

	// Create a cell path with a playbook containing the promoted heuristic
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")
	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		t.Fatalf("mkdir playbookDir: %v", err)
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	entries := []playbook.Entry{
		{
			ID:               "pb-heur0001",
			Type:             "planner_heuristic",
			Title:            "Prefer compile checks before full test suite",
			Content:          "Run go build ./... as the first proof check before any go test invocation to catch syntax and type errors early",
			Rationale:        "Compile errors are cheaper to detect than test failures and provide clearer diagnostics",
			Status:           "active",
			SourceProposalID: "run-099-proposal-heur1",
			SourceRunID:      "run-099",
			SourceSpecID:     "spec-compile-first",
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

	// 3. PlaybookHeuristics field is non-empty
	if req.PlaybookHeuristics == "" {
		t.Fatal("expected non-empty PlaybookHeuristics in plan request")
	}

	// 4. Heuristic title appears in the formatted heuristics
	if !strings.Contains(req.PlaybookHeuristics, "Prefer compile checks before full test suite") {
		t.Errorf("PlaybookHeuristics should contain heuristic title, got:\n%s", req.PlaybookHeuristics)
	}

	// 5. Heuristic content appears in the formatted heuristics
	if !strings.Contains(req.PlaybookHeuristics, "Run go build ./... as the first proof check") {
		t.Errorf("PlaybookHeuristics should contain heuristic content, got:\n%s", req.PlaybookHeuristics)
	}

	// 6. Heuristic rationale appears in the formatted heuristics
	if !strings.Contains(req.PlaybookHeuristics, "Compile errors are cheaper to detect") {
		t.Errorf("PlaybookHeuristics should contain heuristic rationale, got:\n%s", req.PlaybookHeuristics)
	}

	// 7. Verify the formatted output matches FormatPlaybookForPrompt expectations (bold title, content)
	if !strings.Contains(req.PlaybookHeuristics, "**Prefer compile checks before full test suite**") {
		t.Errorf("PlaybookHeuristics should format title in bold markdown, got:\n%s", req.PlaybookHeuristics)
	}

	// 8. Tasks were created from the plan
	if len(rs.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(rs.Tasks))
	}
	if rs.Tasks[0].TaskID != "t-001" {
		t.Errorf("expected task ID t-001, got %q", rs.Tasks[0].TaskID)
	}
}
