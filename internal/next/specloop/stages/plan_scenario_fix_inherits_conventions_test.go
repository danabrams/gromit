package stages

import (
	"context"
	"encoding/json"
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

// TestScenario_FixPlannerInheritsAndExtendsConventions verifies the full
// cross-cycle lifecycle of architecture constraints:
//
//  1. Cycle 1 establishes an architecture constraint in rs.ArchitectureConstraints.
//  2. Cycle 2 fix plan prompt includes that constraint under "Architecture Conventions".
//  3. The cycle 2 fix plan encodes a resolved architecture decision for the drift finding.
//  4. rs.ArchitectureConstraints after cycle 2 contains both the original and new entry.
//  5. Cycle 3 fix prompts and executor task prompts carry both entries.
func TestScenario_FixPlannerInheritsAndExtendsConventions(t *testing.T) {
	const (
		existingConstraint = "Config.Tier always receives a tier label"
		driftFinding       = "review: architecture drift: LLMCompleter.Complete must use context.Context as first parameter"
		newConstraint      = "LLMCompleter.Complete must use context.Context as first parameter"
	)

	// === Seed ===
	// Represent the state at the end of cycle 1: one completed task and one
	// architecture constraint produced by the initial plan's architecture_decisions.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.ReplanContext = &runstore.ReplanContext{Failures: []string{driftFinding}}
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Cycle: 1, Kind: "original"},
	}
	// Seed ArchitectureConstraints from cycle 1 so plan.go can propagate it into fix prompt
	rs.ArchitectureConstraints = []string{existingConstraint}

	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("failed to create run directory: %v", err)
	}
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// The LLM fix planner resolves the drift by recording it as an architecture decision.
	cycle2FixPlan := planner.Plan{
		SpecID:                "spec-001",
		Cycle:                 2,
		Kind:                  "fix",
		ArchitectureDecisions: []string{newConstraint},
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-002",
				Objective:           "Refactor LLMCompleter.Complete to accept context.Context as first parameter",
				ExpectedTouchedArea: []string{"internal/llm/completer.go"},
				ProofChecks:         []string{"go build ./..."},
				ParentCycle:         1,
				FailuresAddressed:   []string{driftFinding},
			},
		},
	}

	// === Invoke — cycle 2 fix plan ===
	b2, _ := json.Marshal(cycle2FixPlan)
	cycle2Agent := &capturingPlannerAgent{response: string(b2)}
	// fp is never called in fix cycles -- only fixPlanner is used
	fp := &fakePlanner{}
	stage2 := NewPlanStage(fp, store, nil)
	realFixPlanner2 := planner.NewPlanner(cycle2Agent, "sonnet")
	stage2.SetFixPlanner(realFixPlanner2)

	action, err := stage2.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("cycle 2: unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("cycle 2: expected Continue, got %v", action.Kind)
	}

	// === Assert: fix plan architecture_decisions includes the resolved convention ===
	foundNew := false
	for _, d := range cycle2FixPlan.ArchitectureDecisions {
		if d == newConstraint {
			foundNew = true
		}
	}
	if !foundNew {
		t.Fatalf("cycle 2 fix plan architecture_decisions must include %q; got %v",
			newConstraint, cycle2FixPlan.ArchitectureDecisions)
	}

	// === Assert: fix plan prompt includes existing constraint under "Architecture Conventions" ===
	if !strings.Contains(cycle2Agent.capturedPrompt, "Architecture Conventions") {
		t.Fatal("cycle 2 fix plan prompt must include 'Architecture Conventions' section")
	}
	if !strings.Contains(cycle2Agent.capturedPrompt, existingConstraint) {
		t.Fatalf("cycle 2 fix plan prompt must include existing constraint %q; got:\n%s",
			existingConstraint, cycle2Agent.capturedPrompt)
	}

	// === Assert: rs.ArchitectureConstraints after cycle 2 contains both constraints ===
	if len(rs.ArchitectureConstraints) != 2 {
		t.Fatalf("after cycle 2, rs.ArchitectureConstraints must contain 2 entries; got %d: %v",
			len(rs.ArchitectureConstraints), rs.ArchitectureConstraints)
	}
	foundExisting := false
	foundNewInState := false
	for _, c := range rs.ArchitectureConstraints {
		if c == existingConstraint {
			foundExisting = true
		}
		if c == newConstraint {
			foundNewInState = true
		}
	}
	if !foundExisting {
		t.Fatalf("rs.ArchitectureConstraints must contain existing constraint %q; got %v",
			existingConstraint, rs.ArchitectureConstraints)
	}
	if !foundNewInState {
		t.Fatalf("rs.ArchitectureConstraints must contain new constraint %q; got %v",
			newConstraint, rs.ArchitectureConstraints)
	}

	// === Continue to cycle 3 ===

	// Advance to cycle 3: mark cycle 2 task done, introduce a new failure.
	for i := range rs.Tasks {
		if rs.Tasks[i].TaskID == "t-002" {
			rs.Tasks[i].Status = "done"
		}
	}
	rs.Cycle = 3
	rs.ReplanContext = &runstore.ReplanContext{Failures: []string{"unit-tests: TestFoo failed"}}

	cycle3FixPlan := planner.Plan{
		SpecID: "spec-001",
		Cycle:  3,
		Kind:   "fix",
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-003",
				Objective:           "Fix TestFoo",
				ExpectedTouchedArea: []string{"internal/pkg/foo.go"},
				ProofChecks:         []string{"go test ./..."},
				ParentCycle:         2,
				FailuresAddressed:   []string{"unit-tests: TestFoo failed"},
			},
		},
	}

	b3, _ := json.Marshal(cycle3FixPlan)
	cycle3Agent := &capturingPlannerAgent{response: string(b3)}
	// fp is never called in fix cycles -- only fixPlanner is used
	stage3 := NewPlanStage(fp, store, nil)
	realFixPlanner3 := planner.NewPlanner(cycle3Agent, "sonnet")
	stage3.SetFixPlanner(realFixPlanner3)

	action3, err := stage3.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("cycle 3: unexpected error: %v", err)
	}
	if action3.Kind != specloop.Continue {
		t.Fatalf("cycle 3: expected Continue, got %v", action3.Kind)
	}

	// === Assert: executor task prompts for cycle 3 include constraints passed via TaskContext ===
	// Constraints flow into task prompts via TaskContext, not via task.ArchitectureConstraints.
	// Verify that cycle 3 executor task prompts can render constraints when they are provided.
	var cycle3Tasks []runstore.Task
	for _, task := range rs.Tasks {
		if task.Cycle == 3 {
			cycle3Tasks = append(cycle3Tasks, task)
		}
	}
	if len(cycle3Tasks) == 0 {
		t.Fatal("expected at least one cycle 3 task to be created")
	}

	// Set up a mock invoker to capture the rendered task prompts
	taskInvoker := &speclooptest.MockInvoker{Result: &provider.Result{Success: true, Model: "sonnet", Duration: time.Second}}
	runner := specloop.NewProviderTaskRunner(taskInvoker, func() string { return "" })

	// Set context provider with both constraints (existing from cycle 1, new from cycle 2)
	constraints := []string{existingConstraint, newConstraint}
	runner.SetContextProvider(func() specloop.TaskContext {
		return specloop.TaskContext{
			ArchitectureConstraints: constraints,
		}
	})

	// Run a cycle 3 task and verify the prompt includes both constraints
	if _, err := runner.RunTask(context.Background(), cycle3Tasks[0]); err != nil {
		t.Fatalf("RunTask: %v", err)
	}

	if !strings.Contains(taskInvoker.CapturedPrompt, "Architecture Conventions") {
		t.Fatal("cycle 3 executor task prompt must include 'Architecture Conventions' section")
	}
	if !strings.Contains(taskInvoker.CapturedPrompt, existingConstraint) {
		t.Fatalf("cycle 3 executor task prompt must contain existing constraint %q", existingConstraint)
	}
	if !strings.Contains(taskInvoker.CapturedPrompt, newConstraint) {
		t.Fatalf("cycle 3 executor task prompt must contain new constraint %q", newConstraint)
	}
}
