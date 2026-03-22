package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// reqCapturingPlanner captures PlanRequests for inspection in tests.
type reqCapturingPlanner struct {
	reqs  []planner.PlanRequest
	plans []planner.Plan
	calls int
}

func (f *reqCapturingPlanner) CreatePlan(_ context.Context, req planner.PlanRequest) (planner.Plan, error) {
	f.reqs = append(f.reqs, req)
	idx := f.calls
	f.calls++
	if idx < len(f.plans) {
		return f.plans[idx], nil
	}
	return planner.Plan{}, nil
}

// TestScenario_PromotedRefinementGuidanceAppearsInRefinementPrompt verifies that when
// a refinement_guidance entry is active in the project's playbook, the refinement
// prompt passed to the planner includes the guidance title and content.
//
// Given: a refinement_guidance entry with title "Ask about deployment constraints
//
//	before designing infrastructure tasks" is active in the project's playbook
//
// When:  a spec enters the refinement stage (plan stage for now) in this project
// Then:  the refinement prompt includes the guidance title and content
func TestScenario_PromotedRefinementGuidanceAppearsInRefinementPrompt(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("# Spec: Infrastructure Setup\n\nDesign and implement the deployment pipeline.\n"), 0o644); err != nil {
		t.Fatalf("write spec-packet.md: %v", err)
	}

	// Create a playbook directory with an active refinement_guidance entry.
	// This simulates a previously promoted refinement_guidance proposal
	// that should now influence spec refinement.
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")
	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		t.Fatalf("mkdir playbook: %v", err)
	}
	guidanceTitle := "Ask about deployment constraints before designing infrastructure tasks"
	guidanceContent := "When a spec involves infrastructure changes, always clarify deployment constraints (region, compliance, downtime windows) before decomposing into tasks. This prevents rework from discovering constraints mid-implementation."
	playbookJSON := `[
  {
    "id": "pb-guidance01",
    "type": "refinement_guidance",
    "title": "` + guidanceTitle + `",
    "content": "` + guidanceContent + `",
    "rationale": "Two prior specs required full replanning after discovering deployment constraints late",
    "status": "active",
    "source_proposal_id": "proposal-rg-001",
    "source_run_id": "run-prior-001",
    "source_spec_id": "spec-prior-infra",
    "created_at": "2026-03-15T10:00:00Z",
    "superseded_by": ""
  }
]`
	if err := os.WriteFile(filepath.Join(playbookDir, "entries.json"), []byte(playbookJSON), 0o644); err != nil {
		t.Fatalf("write entries.json: %v", err)
	}

	// Create a capturing planner that records the PlanRequest so we can
	// inspect what context was passed to the planner.
	validPlanResult := planner.Plan{
		SpecID: "spec-001",
		Cycle:  1,
		Kind:   "original",
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-001",
				Objective:           "Set up deployment pipeline",
				ExpectedTouchedArea: []string{"infra/"},
				ProofChecks:         []string{"true"},
			},
		},
	}
	cp := &reqCapturingPlanner{plans: []planner.Plan{validPlanResult}}

	// === Invoke ===
	stage := NewPlanStage(cp, store, nil)
	stage.SetCellPathResolver(&fakeCellPathResolver{path: cellPath})

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("PlanStage.Run failed: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// === Assert ===
	// The planner should have been called with refinement guidance from the
	// active refinement_guidance playbook entry.
	if len(cp.reqs) != 1 {
		t.Fatalf("expected 1 plan request, got %d", len(cp.reqs))
	}
	req := cp.reqs[0]

	// The refinement guidance title and content must appear somewhere in
	// the data passed to the planner — either in the SpecPacket (if injected
	// there) or in a dedicated field on PlanRequest.
	//
	// Currently the plan stage does not load playbook entries, so this
	// assertion will fail (RED). The implementation should:
	// 1. Load active refinement_guidance entries from the project's playbook
	// 2. Format them and pass them to the planner (e.g. via a RefinementGuidance
	//    field on PlanRequest)
	// 3. The planner prompt builder should include the guidance in the prompt
	combinedContext := req.SpecPacket + strings.Join(req.Failures, " ") + strings.Join(req.CompletedTasks, " ") + req.CurrentDiff + req.RefinementGuidance

	if !strings.Contains(combinedContext, guidanceTitle) {
		t.Fatalf("refinement guidance title %q not found in planner request context;\nSpecPacket: %s", guidanceTitle, req.SpecPacket)
	}
	if !strings.Contains(combinedContext, guidanceContent) {
		t.Fatalf("refinement guidance content not found in planner request context;\nexpected content: %s", guidanceContent)
	}
}
