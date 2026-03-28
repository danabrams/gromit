package planner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_FixPlannerRewritesProofCheck(t *testing.T) {
	// Seed: runstore with a run state whose failure context contains suspect-proof-check output.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.RunID = "run-fix-proof-check"
	rs.Cycle = 1
	rs.Tasks = []runstore.Task{{
		TaskID:    "t-001",
		Status:    "failed",
		Objective: "register --title flag on foo subcommand",
	}}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save runstate: %v", err)
	}
	seeded, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get runstate: %v", err)
	}

	failure := "[suspect-proof-check] All build checks pass but pattern-matching checks failed... grep -q '--title' cmd/foo.go: exit status 1"

	// Invoke: build the fix-plan prompt and create a fix plan from a mocked agent response.
	fixReq := FixPlanRequest{
		OriginalPlan: Plan{
			SpecID: seeded.SpecID,
			Cycle:  seeded.Cycle,
			Tasks: []TaskDef{{
				TaskID:              "t-001",
				Objective:           "register --title flag on foo subcommand",
				ExpectedTouchedArea: []string{"cmd/foo.go"},
				ProofChecks:         []string{"go build ./...", "grep -q '--title' cmd/foo.go"},
			}},
		},
		Failures: []string{failure},
		Cycle:    2,
	}

	prompt := buildFixPlanPrompt(fixReq)

	fixJSON := `{"spec_id":"spec-001","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"proof-check rewrite for suspect-proof-check failure: replace source pattern grep with behavioral CLI help assertion","expected_touched_area":["scenario-contracts.yaml"],"proof_checks":["./binary subcommand --help | grep -q -- '--title'"],"parent_cycle":1,"failures_addressed":["[suspect-proof-check] All build checks pass but pattern-matching checks failed... grep -q '--title' cmd/foo.go: exit status 1"],"fixes":"t-001"}]}`
	agent := &fakeAgent{output: fixJSON}
	p := NewPlanner(agent, "high")

	plan, err := p.CreateFixPlan(context.Background(), fixReq)
	if err != nil {
		t.Fatalf("CreateFixPlan: %v", err)
	}

	// Assert: prompt contains the suspect-proof-check rewrite mechanism (AC10 prompt review).
	if !strings.Contains(prompt, "If a failure message starts with `[suspect-proof-check]`") {
		t.Fatal("expected suspect-proof-check rewrite instruction in fix-plan prompt")
	}
	if !strings.Contains(prompt, "do NOT create a code implementation task") {
		t.Fatal("expected prompt to forbid implementation task for suspect-proof-check failures")
	}
	if !strings.Contains(prompt, "./binary subcommand --help | grep -q -- '--flag-name'") {
		t.Fatal("expected runtime behavioral proof-check example in fix-plan prompt")
	}

	// Assert: generated fix plan contains a proof-check rewrite task with behavioral proof check.
	if len(plan.Tasks) == 0 {
		t.Fatal("expected at least one fix task")
	}

	foundRewrite := false
	for _, task := range plan.Tasks {
		objective := strings.ToLower(task.Objective)
		if strings.Contains(objective, "proof-check rewrite") {
			foundRewrite = true
		}
		for _, check := range task.ProofChecks {
			if check == "./binary subcommand --help | grep -q -- '--title'" {
				foundRewrite = true
			}
		}

		// No task should re-implement CLI flag registration.
		if strings.Contains(objective, "re-implement") || strings.Contains(objective, "register --title") {
			t.Fatalf("unexpected implementation task objective: %q", task.Objective)
		}
		for _, area := range task.ExpectedTouchedArea {
			if area == "cmd/foo.go" {
				t.Fatalf("unexpected implementation touched area in fix task: %q", area)
			}
		}
	}
	if !foundRewrite {
		t.Fatal("expected fix plan to include a proof-check rewrite task with behavioral CLI help check")
	}
}
