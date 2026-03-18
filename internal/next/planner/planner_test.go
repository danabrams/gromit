package planner

import (
	"context"
	"strings"
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
	output  string
	called  bool
	outputs []string // if set, returns outputs[callCount-1] on each call
	calls   int
	prompts []string // captures each prompt passed to Invoke
}

func (f *fakeAgent) Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error) {
	f.called = true
	f.calls++
	f.prompts = append(f.prompts, prompt)
	out := f.output
	if len(f.outputs) > 0 && f.calls <= len(f.outputs) {
		out = f.outputs[f.calls-1]
	}
	return AgentResult{Output: out, TokensIn: 100, TokensOut: 50, Cost: 0.01, Model: "fake-model"}, nil
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

func TestPlanner_CreatePlan_RetryOnParseFailure(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":1,"kind":"original","tasks":[{"task_id":"t-001","objective":"x","expected_touched_area":["a/"],"proof_checks":["true"]}]}`
	agent := &fakeAgent{
		outputs: []string{"not valid json", validJSON},
	}
	p := NewPlanner(agent, "high")

	plan, err := p.CreatePlan(context.Background(), PlanRequest{SpecPacket: "spec", Cycle: 1})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(plan.Tasks))
	}
	if agent.calls != 2 {
		t.Fatalf("expected 2 agent calls, got %d", agent.calls)
	}
}

func TestPlanner_CreatePlan_RetryExhaustion(t *testing.T) {
	agent := &fakeAgent{
		outputs: []string{"bad json", "still bad"},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreatePlan(context.Background(), PlanRequest{SpecPacket: "spec", Cycle: 1})
	if err == nil {
		t.Fatal("expected error after retry exhaustion")
	}
	if agent.calls != 2 {
		t.Fatalf("expected 2 agent calls, got %d", agent.calls)
	}
}

func TestPlanner_CreateFixPlan_RetryOnParseFailure(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-005","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{
		outputs: []string{"garbage", validJSON},
	}
	p := NewPlanner(agent, "high")

	plan, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if plan.Tasks[0].TaskID != "t-005" {
		t.Fatalf("unexpected task ID: %s", plan.Tasks[0].TaskID)
	}
}

func TestPlanner_CreateFixPlan_ValidatePlanWithPrior(t *testing.T) {
	// Task ID t-002 is <= prior max t-004, should fail validation
	badIDJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{
		outputs: []string{badIDJSON, badIDJSON},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err == nil {
		t.Fatal("expected error: fix plan task IDs must be > prior max")
	}
	if agent.calls != 2 {
		t.Fatalf("expected 2 calls (retry), got %d", agent.calls)
	}
}

func TestPlanner_CreateFixPlan_PriorValidationSucceeds(t *testing.T) {
	goodJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-005","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{output: goodJSON}
	p := NewPlanner(agent, "high")

	plan, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Tasks[0].TaskID != "t-005" {
		t.Fatalf("unexpected task ID: %s", plan.Tasks[0].TaskID)
	}
}

func TestBuildFixPlanPrompt_ForbidsReplanningCompletedTasks(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"lint error"},
		Cycle:        2,
	})
	if !strings.Contains(prompt, "Do NOT replan") {
		t.Fatal("fix plan prompt must forbid replanning completed tasks")
	}
}

func TestBuildFixPlanPrompt_IncludesSpecConstraints(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan:    Plan{SpecID: "s1", Cycle: 1},
		Failures:        []string{"format error"},
		Cycle:           2,
		SpecConstraints: "## Out-of-Scope\n- Do NOT modify any existing test files",
	})
	if !strings.Contains(prompt, "Do NOT modify any existing test files") {
		t.Fatal("fix plan prompt must include spec constraints")
	}
	if !strings.Contains(prompt, "HARD REQUIREMENTS") {
		t.Fatal("fix plan prompt must label spec constraints as HARD REQUIREMENTS")
	}
	// Constraints must appear before failures so the LLM anchors on them first
	constraintsIdx := strings.Index(prompt, "HARD REQUIREMENTS")
	failuresIdx := strings.Index(prompt, "Review Findings")
	if failuresIdx < 0 {
		failuresIdx = strings.Index(prompt, "Validation Failures")
	}
	if failuresIdx >= 0 && constraintsIdx > failuresIdx {
		t.Fatal("spec constraints must appear before failures in the fix plan prompt")
	}
}

func TestBuildFixPlanPrompt_NoSpecConstraintsSection_WhenEmpty(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"lint error"},
		Cycle:        2,
	})
	if strings.Contains(prompt, "HARD REQUIREMENTS") {
		t.Fatal("fix plan prompt must not include HARD REQUIREMENTS section when spec constraints are empty")
	}
}

func TestPlanner_CreatePlan_RetryFeedsBackParseError(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":1,"kind":"original","tasks":[{"task_id":"t-001","objective":"x","expected_touched_area":["a/"],"proof_checks":["true"]}]}`
	agent := &fakeAgent{
		outputs: []string{"not valid json", validJSON},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreatePlan(context.Background(), PlanRequest{SpecPacket: "spec", Cycle: 1})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if len(agent.prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(agent.prompts))
	}
	// First prompt should NOT contain error feedback
	if strings.Contains(agent.prompts[0], "Your previous output was invalid") {
		t.Fatal("first prompt should not contain error feedback")
	}
	// Second prompt should contain the parse error
	if !strings.Contains(agent.prompts[1], "Your previous output was invalid") {
		t.Fatal("second prompt should contain error feedback")
	}
	if !strings.Contains(agent.prompts[1], "Please produce valid JSON output") {
		t.Fatal("second prompt should ask for valid JSON")
	}
}

func TestPlanner_CreateFixPlan_RetryFeedsBackParseError(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-005","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{
		outputs: []string{"garbage", validJSON},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if len(agent.prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(agent.prompts))
	}
	if strings.Contains(agent.prompts[0], "Your previous output was invalid") {
		t.Fatal("first prompt should not contain error feedback")
	}
	if !strings.Contains(agent.prompts[1], "Your previous output was invalid") {
		t.Fatal("second prompt should contain error feedback")
	}
}

func TestBuildPlanPrompt_RequiresTestFileProofChecks(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if !strings.Contains(prompt, "*_test.go") {
		t.Fatal("buildPlanPrompt must instruct LLM to require proof checks for *_test.go files")
	}
	if !strings.Contains(prompt, "Do NOT rely solely on `go test ./...`") {
		t.Fatal("buildPlanPrompt must warn that go test passes even without new test assertions")
	}
}

func TestBuildFixPlanPrompt_RequiresTestFileProofChecks(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"missing test coverage"},
		Cycle:        2,
	})
	if !strings.Contains(prompt, "*_test.go") {
		t.Fatal("buildFixPlanPrompt must instruct LLM to require proof checks for *_test.go files")
	}
	if !strings.Contains(prompt, "Do NOT rely solely on `go test ./...`") {
		t.Fatal("buildFixPlanPrompt must warn that go test passes even without new test assertions")
	}
}

func TestPlanner_CreateFixPlan_RetryFeedsBackValidationError(t *testing.T) {
	// First output: valid JSON but task ID t-002 <= prior max t-004
	badIDJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	// Second output: valid JSON with task ID t-005 > prior max t-004
	goodJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-005","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{
		outputs: []string{badIDJSON, goodJSON},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if len(agent.prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(agent.prompts))
	}
	if strings.Contains(agent.prompts[0], "Your previous output was invalid") {
		t.Fatal("first prompt should not contain error feedback")
	}
	if !strings.Contains(agent.prompts[1], "prior-plan validation failed") {
		t.Fatal("second prompt should contain prior-plan validation error")
	}
}

func TestBuildFixPlanPrompt_InstructsAboutFixesField(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"compilation error in main.go"},
		Cycle:        2,
	})
	if !strings.Contains(prompt, "fixes") {
		t.Fatal("buildFixPlanPrompt must mention the 'fixes' field")
	}
	if !strings.Contains(prompt, "failed task") {
		t.Fatal("buildFixPlanPrompt must reference 'failed task' when describing the fixes field")
	}
}
