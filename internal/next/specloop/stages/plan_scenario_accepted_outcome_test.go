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

type acceptedScenarioPromptCaptureAgent struct {
	prompt string
}

func (a *acceptedScenarioPromptCaptureAgent) Invoke(ctx context.Context, prompt string, tier string) (planner.AgentResult, error) {
	a.prompt = prompt
	return planner.AgentResult{
		Output: `{"spec_id":"spec-accepted","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"Fix the failure","expected_touched_area":["pkg/foo"],"proof_checks":["go test ./pkg/foo/..."],"parent_cycle":1,"failures_addressed":["review: needs check"]}]}`,
	}, nil
}

type scenarioRealFixPlanner struct {
	agent *acceptedScenarioPromptCaptureAgent
}

func (p *scenarioRealFixPlanner) CreateFixPlan(ctx context.Context, req planner.FixPlanRequest) (planner.Plan, error) {
	return planner.NewPlanner(p.agent, "high").CreateFixPlan(ctx, req)
}

func TestScenario_AcceptedOutcomeProducesNoReviewerGuidance(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:   "run-accepted",
		SpecID:  "spec-accepted",
		Cycle:   2,
		Resumed: true,
		ReplanContext: &runstore.ReplanContext{
			Failures: []string{"review: needs check"},
		},
		Tasks: []runstore.Task{
			{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
		},
	}

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
	if err := os.WriteFile(
		filepath.Join(evidenceDir, "review-outcome.json"),
		[]byte(`{"outcome":"accepted","summary":"Looks good."}`),
		0o644,
	); err != nil {
		t.Fatalf("write review-outcome.json: %v", err)
	}

	agent := &acceptedScenarioPromptCaptureAgent{}
	fixPlanner := &scenarioRealFixPlanner{agent: agent}
	stage := NewPlanStage(nil, store, nil)
	stage.SetFixPlanner(fixPlanner)

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("stage.Run: %v", err)
	}

	// Assert
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if agent.prompt == "" {
		t.Fatal("expected fix-plan prompt to be captured")
	}
	if strings.Contains(agent.prompt, "## Reviewer Instructions") {
		t.Fatalf("did not expect Reviewer Instructions section for accepted outcome, prompt:\n%s", agent.prompt)
	}
	if !strings.Contains(agent.prompt, "## Review Findings to Fix") {
		t.Fatalf("expected review findings section in prompt, prompt:\n%s", agent.prompt)
	}
}
