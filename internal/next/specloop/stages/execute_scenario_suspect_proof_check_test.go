package stages

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type scenarioPlannerAgent struct {
	output string
	prompt string
}

func (f *scenarioPlannerAgent) Invoke(ctx context.Context, prompt string, tier string) (planner.AgentResult, error) {
	f.prompt = prompt
	return planner.AgentResult{Output: f.output}, nil
}

func TestScenario_SuspectProofCheck_AnnotationFlowsToExecuteAndFixPlanner(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-suspect-proof-check", "proj-cli")
	rs.Cycle = 2
	rs.Tasks = []runstore.Task{
		{
			TaskID:      "t-001",
			Status:      "pending",
			Objective:   "wire --title flag",
			ProofChecks: []string{"go build ./...", "grep -q '--title' cmd/foo.go"},
		},
	}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save runstate: %v", err)
	}

	seeded, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get runstate: %v", err)
	}

	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{
			{Status: "done"},
		},
	}
	inspector := &failingInspector{
		failures: map[string][]string{
			"t-001": {"grep -q '--title' cmd/foo.go: exit status 1"},
		},
	}

	// Invoke
	stage := NewExecuteStage(runner, ExecuteStageConfig{
		MaxRetries: 1,
		Inspector:  inspector,
	})

	action, err := stage.Run(context.Background(), seeded)
	if err != nil {
		t.Fatalf("execute stage: %v", err)
	}

	// Assert (execute-stage failure context gets annotated suspect-proof-check string)
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil || len(action.Context.Failures) == 0 {
		t.Fatal("expected failure context with failures")
	}

	const expectedPrefix = "[suspect-proof-check] All build checks pass but pattern-matching checks failed. The implementation may be correct; proof checks may be testing source structure rather than behavior. "
	wantFailure := expectedPrefix + "grep -q '--title' cmd/foo.go: exit status 1"

	found := false
	for _, f := range action.Context.Failures {
		if f == wantFailure {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected annotated failure in context, got: %v", action.Context.Failures)
	}

	// Invoke fix planner with propagated failures and assert rewrite-style plan is produced.
	fixJSON := `{"spec_id":"spec-suspect-proof-check","cycle":3,"kind":"fix","tasks":[{"task_id":"t-002","objective":"rewrite suspect proof check to behavioral runtime check","expected_touched_area":["scenario-contracts.yaml"],"proof_checks":["go build ./...","./foo --help | grep -q -- '--title'"],"parent_cycle":2,"failures_addressed":["` + wantFailure + `"]}]}`
	agent := &scenarioPlannerAgent{output: fixJSON}
	p := planner.NewPlanner(agent, "high")

	plan, err := p.CreateFixPlan(context.Background(), planner.FixPlanRequest{
		OriginalPlan: planner.Plan{
			SpecID: "spec-suspect-proof-check",
			Cycle:  2,
			Kind:   "original",
		},
		Failures: action.Context.Failures,
		Cycle:    3,
	})
	if err != nil {
		t.Fatalf("create fix plan: %v", err)
	}

	// Assert (planner received suspect message and returns proof-check rewrite task)
	if !strings.Contains(agent.prompt, "[suspect-proof-check]") {
		t.Fatalf("expected fix planner prompt to include suspect-proof-check failure, got: %s", agent.prompt)
	}
	if plan.Kind != "fix" || len(plan.Tasks) != 1 {
		t.Fatalf("expected one fix task, got kind=%q tasks=%d", plan.Kind, len(plan.Tasks))
	}
	if !strings.Contains(plan.Tasks[0].Objective, "rewrite") {
		t.Fatalf("expected rewrite objective, got: %s", plan.Tasks[0].Objective)
	}
	if len(plan.Tasks[0].ProofChecks) < 2 || !strings.Contains(plan.Tasks[0].ProofChecks[1], "--help") {
		t.Fatalf("expected runtime behavioral proof check, got: %v", plan.Tasks[0].ProofChecks)
	}
}
