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

type guidanceScenarioPromptCaptureAgent struct {
	output  string
	prompts []string
}

func (a *guidanceScenarioPromptCaptureAgent) Invoke(ctx context.Context, prompt string, tier string) (planner.AgentResult, error) {
	a.prompts = append(a.prompts, prompt)
	return planner.AgentResult{
		Output:    a.output,
		TokensIn:  1,
		TokensOut: 1,
		Model:     "test",
	}, nil
}

func TestScenario_GuidancePersistsAcrossMultipleReworkCycles(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 3
	rs.Resumed = true
	rs.ReplanContext = &runstore.ReplanContext{
		Failures: []string{"review: cycle 2 fix did not resolve bypass behavior"},
	}
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
		{TaskID: "t-002", Status: "failed", Cycle: 2, Kind: "fix"},
	}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run state: %v", err)
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
		[]byte(`{"outcome":"rework_implementation_gap","summary":"Fix the bypass test and keep existing auth checks intact."}`),
		0o644,
	); err != nil {
		t.Fatalf("write review-outcome.json: %v", err)
	}

	agent := &guidanceScenarioPromptCaptureAgent{
		output: `{
			"spec_id":"spec-001",
			"cycle":3,
			"kind":"fix",
			"tasks":[
				{
					"task_id":"t-003",
					"objective":"Address bypass review finding",
					"expected_touched_area":["internal/next/specloop/stages/plan.go"],
					"proof_checks":["go test ./internal/next/specloop/stages/..."],
					"parent_cycle":2,
					"failures_addressed":["review: cycle 2 fix did not resolve bypass behavior"]
				}
			]
		}`,
	}
	fixPlanner := planner.NewPlanner(agent, "high")

	stage := NewPlanStage(nil, store, nil)
	stage.SetFixPlanner(fixPlanner)

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("plan stage run: %v", err)
	}

	// Assert
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if len(agent.prompts) == 0 {
		t.Fatal("expected fix planner to be invoked at least once")
	}
	prompt := agent.prompts[0]
	if !strings.Contains(prompt, "## Reviewer Instructions") {
		t.Fatalf("expected fix plan prompt to include Reviewer Instructions section, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Fix the bypass test and keep existing auth checks intact.") {
		t.Fatalf("expected fix plan prompt to include reviewer guidance text, got:\n%s", prompt)
	}
}
