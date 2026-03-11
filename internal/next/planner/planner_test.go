package planner

import (
	"context"
	"testing"
)

func TestPlanner_InvokesAgentAndParsesPlan(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":1,"kind":"original","tasks":[{"task_id":"t-001","objective":"x","expected_touched_area":["a/"],"proof_checks":["true"]}]}`
	agent := &fakeAgent{output: validJSON}
	p := NewPlanner(agent, "high")

	plan, err := p.CreatePlan(context.Background(), PlanRequest{
		SpecPacket: "build a thing", Cycle: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(plan.Tasks))
	}
	if !agent.called {
		t.Fatal("agent not called")
	}
}

type fakeAgent struct {
	output string
	called bool
}

func (f *fakeAgent) Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error) {
	f.called = true
	return AgentResult{Output: f.output, TokensIn: 100, TokensOut: 50, Cost: 0.01, Model: "fake-model"}, nil
}

func TestPlanner_CreateFixPlan(t *testing.T) {
	fixJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix lint","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["lint failure"]}]}`
	agent := &fakeAgent{output: fixJSON}
	p := NewPlanner(agent, "high")

	plan, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		CompletedTasks: []CompletedTask{{
			TaskID:            "t-001",
			Attempts:          1,
			FilesChanged:      []string{"a/foo.go"},
			ValidationOutcome: "passed",
		}},
		Failures:    []string{"lint failure in a/"},
		CurrentDiff: "diff --git ...",
		Cycle:       2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Kind != "fix" {
		t.Fatalf("want fix, got %s", plan.Kind)
	}
	if plan.Tasks[0].ParentCycle != 1 {
		t.Fatal("expected parent_cycle=1")
	}
}
