package planner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type scenarioNoBuildRunner struct{}

func (r *scenarioNoBuildRunner) RunTask(_ context.Context, _ runstore.Task) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

func (r *scenarioNoBuildRunner) RepairTask(_ context.Context, _ runstore.Task, _ []string) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

type scenarioNoBuildInspector struct {
	failures map[string][]string
}

func (i *scenarioNoBuildInspector) Inspect(_ context.Context, task runstore.Task) specloop.InspectResult {
	return specloop.InspectResult{
		Pass:     false,
		Failures: append([]string(nil), i.failures[task.TaskID]...),
	}
}

func (i *scenarioNoBuildInspector) SetKnownGaps(string) {}

func TestScenario_NoBuildCheckInTask_NoAnnotation(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.RunID = "run-no-build-check"
	rs.Tasks = []runstore.Task{{
		TaskID:      "t-001",
		Status:      "pending",
		Objective:   "wire title option",
		ProofChecks: []string{"grep -q '--title' cmd/foo.go", "awk '/stepA/' foo.go"},
	}}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save runstate: %v", err)
	}
	seeded, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get runstate: %v", err)
	}

	grepFailure := "grep -q '--title' cmd/foo.go: exit status 1"
	awkFailure := "awk '/stepA/' foo.go: exit status 1"

	runner := &scenarioNoBuildRunner{}
	inspector := &scenarioNoBuildInspector{
		failures: map[string][]string{
			"t-001": {grepFailure, awkFailure},
		},
	}

	// Invoke
	results, err := specloop.RunTaskLoop(context.Background(), seeded.Tasks, runner, specloop.TaskLoopConfig{
		MaxRetries: 0,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatalf("RunTaskLoop: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	fixJSON := `{"spec_id":"spec-001","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix proof checks","expected_touched_area":["scenario-contracts.yaml"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["grep -q '--title' cmd/foo.go: exit status 1"]}]}`
	agent := &fakeAgent{output: fixJSON}
	p := NewPlanner(agent, "high")

	_, err = p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan: Plan{SpecID: "spec-001", Cycle: 1},
		Failures:     results[0].Failures,
		Cycle:        2,
	})
	if err != nil {
		t.Fatalf("CreateFixPlan: %v", err)
	}
	if len(agent.prompts) != 1 {
		t.Fatalf("expected 1 planner prompt, got %d", len(agent.prompts))
	}

	// Assert
	if results[0].Status != "failed" {
		t.Fatalf("expected failed status, got %q", results[0].Status)
	}
	if len(results[0].Failures) != 2 {
		t.Fatalf("expected 2 failures, got %d: %v", len(results[0].Failures), results[0].Failures)
	}
	for _, f := range results[0].Failures {
		if strings.Contains(f, "[suspect-proof-check]") {
			t.Fatalf("did not expect suspect-proof-check annotation, got: %v", results[0].Failures)
		}
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
