package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
)

type capturePromptAgent struct {
	output  string
	prompts []string
}

func (a *capturePromptAgent) Invoke(ctx context.Context, prompt string, tier string) (planner.AgentResult, error) {
	a.prompts = append(a.prompts, prompt)
	return planner.AgentResult{Output: a.output}, nil
}

func TestScenario_ReworkResumeInjectsReviewerGuidanceIntoFixPlan(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.RunID = "run-rework-reviewer-guidance"
	rs.Cycle = 2
	rs.Resumed = true
	rs.ReplanContext = &runstore.ReplanContext{
		Failures: []string{
			"review: bypass test failure",
			"unit: another fix-cycle failure",
		},
	}
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
	}

	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec packet"), 0o644); err != nil {
		t.Fatalf("write spec-packet.md: %v", err)
	}

	summary := "Fix the bypass test at exec_scenario_escalation_fails_run_blocked_test.go:173 — remove the manual Blocked override and drive through the real count>=3 path"
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence dir: %v", err)
	}
	reviewOutcomeJSON := `{"outcome":"rework_implementation_gap","summary":"` + summary + `"}`
	if err := os.WriteFile(filepath.Join(evidenceDir, "review-outcome.json"), []byte(reviewOutcomeJSON), 0o644); err != nil {
		t.Fatalf("write review-outcome.json: %v", err)
	}

	agent := &capturePromptAgent{
		output: `{"spec_id":"spec-001","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix review issue","expected_touched_area":["cmd/gromit-next/"],"proof_checks":["go test ./cmd/gromit-next/..."],"parent_cycle":1,"failures_addressed":["review: bypass test failure"]}]}`,
	}
	fixPlanner := planner.NewPlanner(agent, "high")
	stage := NewPlanStage(nil, store, nil)
	stage.SetFixPlanner(fixPlanner)

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("stage run failed: %v", err)
	}
	if action.Kind != 0 { // specloop.Continue
		t.Fatalf("expected Continue action, got %v", action.Kind)
	}
	if len(agent.prompts) == 0 {
		t.Fatal("expected fix planner prompt to be captured")
	}
	prompt := agent.prompts[0]

	// Assert
	if !strings.Contains(prompt, "## Reviewer Instructions") {
		t.Fatal("expected Reviewer Instructions section in fix plan prompt")
	}
	if !strings.Contains(prompt, summary) {
		t.Fatalf("expected exact reviewer summary in prompt, got: %q", prompt)
	}
	reviewerIdx := strings.Index(prompt, "## Reviewer Instructions")
	findingsIdx := strings.Index(prompt, "## Review Findings to Fix")
	if reviewerIdx < 0 || findingsIdx < 0 {
		t.Fatalf("expected both Reviewer Instructions and Review Findings sections, got prompt: %q", prompt)
	}
	if reviewerIdx > findingsIdx {
		t.Fatal("expected Reviewer Instructions to appear before Review Findings to Fix")
	}
}
