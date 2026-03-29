package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/specloop/speclooptest"
	"github.com/danabrams/gromit/internal/provider"
)

// TestScenario_NoConventions_NoPromptNoise verifies that when a spec touches a single
// file with no cross-cutting patterns, the planner produces no ArchitectureDecisions,
// RunState.ArchitectureConstraints remains empty, and neither executor task prompts
// nor fix plan prompts contain an "Architecture Conventions" section.
func TestScenario_NoConventions_NoPromptNoise(t *testing.T) {
	// === Seed ===
	// A spec that touches exactly one file — no need for cross-cutting conventions.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-single-file", "proj-001")
	rs.Cycle = 1
	rs.ReplanContext = &runstore.ReplanContext{Failures: []string{}}
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("Add divide function to calc/calc.go"), 0o644); err != nil {
		t.Fatalf("write spec-packet.md: %v", err)
	}

	// Plan with no ArchitectureDecisions (single file, no cross-cutting patterns).
	singleFilePlan := planner.Plan{
		SpecID: "spec-single-file",
		Cycle:  1,
		Kind:   "original",
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-001",
				Objective:           "Implement divide function",
				ExpectedTouchedArea: []string{"calc/calc.go"},
				ProofChecks:         []string{"go test ./calc/..."},
			},
		},
		// ArchitectureDecisions intentionally absent.
	}

	// === Invoke: initial plan stage ===
	fp := &fakePlanner{plans: []planner.Plan{singleFilePlan}}
	stage := NewPlanStage(fp, store, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("PlanStage.Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// === Assert 0: rs.ArchitectureConstraints is empty ===
	if len(rs.ArchitectureConstraints) != 0 {
		t.Errorf("expected empty ArchitectureConstraints for no-convention plan, got %v", rs.ArchitectureConstraints)
	}

	// === Assert 1: plan.ArchitectureDecisions is empty ===
	if len(singleFilePlan.ArchitectureDecisions) != 0 {
		t.Errorf("plan.ArchitectureDecisions should be empty for single-file spec, got %v", singleFilePlan.ArchitectureDecisions)
	}

	// === Assert 2: executor task prompts contain no "Architecture Conventions" section ===
	if len(rs.Tasks) == 0 {
		t.Fatal("expected at least one task to be created")
	}
	inv := &speclooptest.MockInvoker{Result: &provider.Result{Success: true, Model: "sonnet", Duration: time.Second}}
	runner := specloop.NewProviderTaskRunner(inv, func() string { return "" })
	// Don't set a context provider (or set empty constraints) since no conventions were established
	if _, err := runner.RunTask(context.Background(), rs.Tasks[0]); err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if strings.Contains(inv.CapturedPrompt, "Architecture Conventions") {
		t.Errorf("executor task prompt must not contain 'Architecture Conventions' when no constraints are established;\ngot prompt:\n%s", inv.CapturedPrompt)
	}

	// === Assert 3: fix plan prompts contain no "Architecture Conventions" section ===
	// Seed a fix cycle run when no ArchitectureConstraints are provided.
	rsfix := runstore.NewRunState("spec-single-file", "proj-001")
	rsfix.Cycle = 2
	rsfix.ReplanContext = &runstore.ReplanContext{Failures: []string{"test failure in calc"}}
	rsfix.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
	}

	fixRunDir := store.RunDir(rsfix.RunID)
	if err := os.MkdirAll(fixRunDir, 0o755); err != nil {
		t.Fatalf("mkdir fixRunDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixRunDir, "spec-packet.md"), []byte("Add divide function to calc/calc.go"), 0o644); err != nil {
		t.Fatalf("write fix spec-packet.md: %v", err)
	}

	// Use a real planner.Planner backed by a capturing agent so we can inspect
	// the prompt string it builds from the FixPlanRequest.
	const validFixPlan = `{"spec_id":"spec-single-file","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix calc","expected_touched_area":["calc/calc.go"],"proof_checks":["go test ./calc/..."],"parent_cycle":1,"failures_addressed":["test failure"]}]}`
	agentCapture := &capturingPlannerAgent{response: validFixPlan}
	realFixPlanner := planner.NewPlanner(agentCapture, "sonnet")

	fixStage := NewPlanStage(fp, store, nil)
	fixStage.SetFixPlanner(realFixPlanner)

	if _, err := fixStage.Run(context.Background(), rsfix); err != nil {
		t.Fatalf("fix PlanStage.Run: %v", err)
	}

	if agentCapture.capturedPrompt == "" {
		t.Fatal("expected fix planner agent to be invoked and capture a prompt")
	}
	if strings.Contains(agentCapture.capturedPrompt, "Architecture Conventions") {
		t.Errorf("fix plan prompt must not contain 'Architecture Conventions' when rs has no architecture constraints;\ngot prompt:\n%s", agentCapture.capturedPrompt)
	}
}
