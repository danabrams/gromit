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

type promptCaptureAgent struct {
	prompt string
}

func (a *promptCaptureAgent) Invoke(ctx context.Context, prompt string, tier string) (planner.AgentResult, error) {
	a.prompt = prompt
	return planner.AgentResult{
		Output: `{"spec_id":"spec-001","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"Apply reviewer guidance","expected_touched_area":["internal/next/specloop/stages/plan.go"],"proof_checks":["go build ./..."],"parent_cycle":1,"failures_addressed":["review: follow reviewer guidance"]}]}`,
	}, nil
}

func TestScenario_VisionChangeResume_InjectsReviewerGuidance(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.RunID = "run-vision-change"
	rs.Cycle = 2
	rs.Resumed = true
	rs.ReplanContext = &runstore.ReplanContext{Failures: []string{"review: follow reviewer guidance"}}
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"}}

	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644); err != nil {
		t.Fatalf("write spec-packet.md: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review-outcome.json"), []byte(`{"outcome":"rework_vision_change","summary":"Shift implementation to the new product vision and tighten acceptance checks."}`), 0o644); err != nil {
		t.Fatalf("write review-outcome.json: %v", err)
	}

	agent := &promptCaptureAgent{}
	fixPlanner := planner.NewPlanner(agent, "high")
	stage := NewPlanStage(nil, store, nil)
	stage.SetFixPlanner(fixPlanner)

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("PlanStage.Run: %v", err)
	}

	// Assert
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if !strings.Contains(agent.prompt, "## Reviewer Instructions") {
		t.Fatal("expected fix plan prompt to include Reviewer Instructions section")
	}
	if !strings.Contains(agent.prompt, "Shift implementation to the new product vision and tighten acceptance checks.") {
		t.Fatal("expected fix plan prompt to include reviewer summary text")
	}
}
