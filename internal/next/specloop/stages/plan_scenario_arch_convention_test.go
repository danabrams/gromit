package stages

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/specloop/speclooptest"
	"github.com/danabrams/gromit/internal/provider"
)

// TestScenario_PlannerCapturesConventionOnInitialPlan verifies that when the
// planner returns architecture decisions on a cycle-1 plan, the full pipeline
// propagates them: rs.ArchitectureConstraints is populated, and the executor
// prompt renderer receives them via TaskContext so it will emit the
// "Architecture Conventions" section.
//
// Given:  a spec that introduces a shared type used across multiple files
//
//	(Config.Tier used in review_distill.go, review_guided.go, review_packet.go)
//
// When:   the planner generates the cycle 1 plan
// Then:   plan.ArchitectureDecisions contains the tier-label convention
// And:    rs.ArchitectureConstraints contains that entry after planning completes
// And:    the executor task prompt receives those constraints via TaskContext
//
//	(the task runner's renderTaskBody reads TaskContext.ArchitectureConstraints)
func TestScenario_PlannerCapturesConventionOnInitialPlan(t *testing.T) {
	// ── Seed ────────────────────────────────────────────────────────────────

	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-0042", "proj-review")
	rs.Cycle = 1
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}

	// The spec describes a shared Config.Tier type used across three files.
	specPacket := `# Spec 0042 — Config.Tier propagation

## Vision
Config.Tier selects the model tier label (e.g. "medium") that callers pass to the
model-resolver. It must never hold a resolved model name such as "claude-sonnet-4-6".

## In-Scope
- review_distill.go: read Config.Tier and forward to resolver
- review_guided.go:  read Config.Tier and forward to resolver
- review_packet.go:  read Config.Tier and forward to resolver
`
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte(specPacket), 0o644); err != nil {
		t.Fatalf("write spec-packet.md: %v", err)
	}

	// The convention the planner is expected to emit.
	tierConvention := `Config.Tier always receives a tier label such as "medium", never a resolved model name such as "claude-sonnet-4-6"`

	// The planner returns a plan whose ArchitectureDecisions documents the convention.
	initialPlan := planner.Plan{
		SpecID: "spec-0042",
		Cycle:  1,
		Kind:   "original",
		ArchitectureDecisions: []string{
			tierConvention,
		},
		Tasks: []planner.TaskDef{
			{
				TaskID:              "t-001",
				Objective:           "Add Config.Tier field and propagate to review_distill.go",
				ExpectedTouchedArea: []string{"internal/review/review_distill.go", "internal/config/config.go"},
				ProofChecks:         []string{"go build ./...", "go test -run TestDistill ./internal/review/..."},
			},
			{
				TaskID:              "t-002",
				Objective:           "Propagate Config.Tier to review_guided.go and review_packet.go",
				ExpectedTouchedArea: []string{"internal/review/review_guided.go", "internal/review/review_packet.go"},
				ProofChecks:         []string{"go build ./...", "go test -run TestGuided ./internal/review/..."},
			},
		},
	}

	fp := &fakePlanner{plans: []planner.Plan{initialPlan}}
	stage := NewPlanStage(fp, store, nil)

	// ── Invoke ───────────────────────────────────────────────────────────────

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("PlanStage.Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// ── Assert: plan.ArchitectureDecisions contained the convention ───────────
	// (verified via the planner stub — the plan we injected carried the decision)
	if fp.calls != 1 {
		t.Fatalf("expected exactly 1 planner call, got %d", fp.calls)
	}
	if len(initialPlan.ArchitectureDecisions) == 0 {
		t.Fatal("test precondition: initialPlan.ArchitectureDecisions must be non-empty")
	}
	if initialPlan.ArchitectureDecisions[0] != tierConvention {
		t.Fatalf("plan.ArchitectureDecisions[0] = %q; want %q",
			initialPlan.ArchitectureDecisions[0], tierConvention)
	}

	// ── Assert: rs.ArchitectureConstraints is populated from plan decisions ──
	// AC3 requires that RunState.ArchitectureConstraints is populated from
	// plan.ArchitectureDecisions after planning completes.
	if len(rs.ArchitectureConstraints) == 0 {
		t.Fatal("rs.ArchitectureConstraints is empty; AC3 requires it to be populated from plan.ArchitectureDecisions")
	}
	if !slices.Contains(rs.ArchitectureConstraints, tierConvention) {
		t.Fatalf("rs.ArchitectureConstraints does not contain tier convention: %q\nGot: %v",
			tierConvention, rs.ArchitectureConstraints)
	}

	// ── Assert: task runner renders conventions into executor prompt ─────────
	// The task runner's renderTaskBody reads TaskContext.ArchitectureConstraints and,
	// when non-empty, emits a "### Architecture Conventions" section listing each
	// entry with a "- " prefix (see provider_taskrunner.go renderTaskBody).
	// Constraints are stored on RunState and passed to TaskContext.
	if len(rs.Tasks) == 0 {
		t.Fatal("no tasks were created by PlanStage")
	}
	for _, task := range rs.Tasks {
		t.Run(task.TaskID, func(t *testing.T) {
			// Create a mock invoker to capture the rendered prompt
			inv := &speclooptest.MockInvoker{
				Result: &provider.Result{
					Success:  true,
					Model:    "sonnet",
					Duration: 1 * time.Second,
				},
			}
			runner := specloop.NewProviderTaskRunner(inv, func() string { return "" })
			// Set context provider with the architecture constraints from RunState
			runner.SetContextProvider(func() specloop.TaskContext {
				return specloop.TaskContext{
					ArchitectureConstraints: rs.ArchitectureConstraints,
				}
			})
			_, err := runner.RunTask(context.Background(), task)
			if err != nil {
				t.Fatalf("RunTask failed: %v", err)
			}

			prompt := inv.CapturedPrompt

			// Assert: prompt contains the Architecture Conventions section header
			if !strings.Contains(prompt, "### Architecture Conventions") {
				t.Fatal("prompt does not contain '### Architecture Conventions' section header")
			}

			// Assert: prompt contains the constraint text
			if !strings.Contains(prompt, tierConvention) {
				t.Fatalf("prompt does not contain tier convention: %q", tierConvention)
			}

			// Assert: convention appears after the section header
			headerIdx := strings.Index(prompt, "### Architecture Conventions")
			constraintIdx := strings.Index(prompt, tierConvention)
			if headerIdx >= constraintIdx {
				t.Errorf("Architecture Conventions header (pos %d) must appear before constraint (pos %d)",
					headerIdx, constraintIdx)
			}

			// Assert: constraint is rendered as a list item with "- " prefix
			if !strings.Contains(prompt, "- "+tierConvention) {
				t.Errorf("constraint should be rendered as a list item with '- ' prefix")
			}
		})
	}
}
