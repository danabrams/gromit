package planner

import (
	"context"
	"strings"
	"testing"
)

func TestScenario_NoBuildCheckInTask_NoAnnotation(t *testing.T) {
	// Directly construct failures that would result from a task with no build check.
	// When no build check exists in ProofChecks, RunTaskLoop produces failures without
	// [suspect-proof-check] annotation. This test verifies that planner receives and
	// renders those raw failure strings correctly in the prompt.
	grepFailure := "grep -q '--title' cmd/foo.go: exit status 1"
	awkFailure := "awk '/stepA/' foo.go: exit status 1"
	failures := []string{grepFailure, awkFailure}

	fixJSON := `{"spec_id":"spec-001","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix proof checks","expected_touched_area":["scenario-contracts.yaml"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["grep -q '--title' cmd/foo.go: exit status 1"]}]}`
	agent := &fakeAgent{output: fixJSON}
	p := NewPlanner(agent, "high")

	_, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan: Plan{SpecID: "spec-001", Cycle: 1},
		Failures:     failures,
		Cycle:        2,
	})
	if err != nil {
		t.Fatalf("CreateFixPlan: %v", err)
	}
	if len(agent.prompts) != 1 {
		t.Fatalf("expected 1 planner prompt, got %d", len(agent.prompts))
	}

	prompt := agent.prompts[0]
	if !strings.Contains(prompt, "- "+grepFailure) {
		t.Fatalf("expected raw grep failure in planner prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "- "+awkFailure) {
		t.Fatalf("expected raw awk failure in planner prompt, got: %s", prompt)
	}
	if strings.Contains(prompt, "- [suspect-proof-check]") {
		t.Fatalf("did not expect suspect-proof-check failure item in planner prompt: %s", prompt)
	}
}
