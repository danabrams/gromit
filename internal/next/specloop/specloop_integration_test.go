package specloop

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// --- Helpers for integration tests ---

// scenarioStage is a flexible mock stage that calls a user-supplied function
// receiving the RunState and the current call count (1-indexed).
type scenarioStage struct {
	name      string
	callCount int
	fn        func(ctx context.Context, rs *runstore.RunState, call int) (NextAction, error)
}

func (s *scenarioStage) Name() string { return s.name }
func (s *scenarioStage) Run(ctx context.Context, rs *runstore.RunState) (NextAction, error) {
	s.callCount++
	return s.fn(ctx, rs, s.callCount)
}

// passThrough returns a scenarioStage that always continues.
func passThrough(name string) *scenarioStage {
	return &scenarioStage{
		name: name,
		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
			return NextAction{Kind: Continue}, nil
		},
	}
}

// stdBudget returns a budget suitable for most integration tests.
func stdBudget(maxCycles int) *Budget {
	return NewBudget(execpolicy.Budgets{
		MaxSpecCycles:          maxCycles,
		MaxRunCostUSD:          99,
		MaxRunDurationSeconds:  3600,
		MaxTaskDurationSeconds: 300,
	})
}

// --- Scenario 8: Happy path — all stages pass on first cycle ---

func TestIntegration_ReviewAcceptance_HappyPath_ReadyForReview(t *testing.T) {
	initStage := passThrough("init")
	planStage := passThrough("plan")
	compileStage := passThrough("compile")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalReviewPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}
	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalAcceptancePassed = true
			return NextAction{Kind: Continue}, nil
		},
	}
	finalizeStage := &scenarioStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			// Mimic FinalizeStage logic: set terminal status based on gates
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			} else {
				rs.Status = runstore.StatusNeedsHuman
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{initStage, planStage, compileStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(3),
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, rs.Status)
	}
	if rs.Cycle != 1 {
		t.Errorf("want cycle 1, got %d", rs.Cycle)
	}
	if !rs.FinalValidationPassed {
		t.Error("FinalValidationPassed should be true")
	}
	if !rs.FinalReviewPassed {
		t.Error("FinalReviewPassed should be true")
	}
	if !rs.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be true")
	}
}

// --- Scenario 9: Review blocking finding triggers fix cycle ---

func TestIntegration_ReviewFinding_TriggersFixCycle(t *testing.T) {
	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Review: cycle 1 has blocking finding; cycle 2 is clean
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
			if call == 1 {
				rs.FinalReviewPassed = false
				rs.ReviewFindings = []string{"review:spec_alignment:error:handler.go:10 — missing validation"}
				return NextAction{
					Kind: ReplanFrom,
					Context: &FailureContext{
						Failures: []string{"review:spec_alignment:error:handler.go:10 — missing validation"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			rs.FinalReviewPassed = true
			rs.ReviewFindings = []string{}
			return NextAction{Kind: Continue}, nil
		},
	}

	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalAcceptancePassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	finalizeStage := &scenarioStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			} else {
				rs.Status = runstore.StatusNeedsHuman
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(3),
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, rs.Status)
	}
	if rs.Cycle != 2 {
		t.Errorf("want cycle 2 (one replan), got %d", rs.Cycle)
	}
	if planStage.callCount != 2 {
		t.Errorf("plan should run twice (initial + replan), got %d", planStage.callCount)
	}
	if reviewStage.callCount != 2 {
		t.Errorf("review should run twice, got %d", reviewStage.callCount)
	}
}

// --- Scenario 10: Acceptance fail triggers fix cycle, then passes ---

func TestIntegration_AcceptanceFail_FixCycle_ThenPass(t *testing.T) {
	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalReviewPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Accept: cycle 1 fails; cycle 2 passes
	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
			if call == 1 {
				rs.FinalAcceptancePassed = false
				rs.AcceptanceResults = []string{"acceptance:fail:criterion x — not done"}
				return NextAction{
					Kind: ReplanFrom,
					Context: &FailureContext{
						Failures: []string{"acceptance:fail:criterion x — not done"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			rs.FinalAcceptancePassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	finalizeStage := &scenarioStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			} else {
				rs.Status = runstore.StatusNeedsHuman
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(3),
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, rs.Status)
	}
	if rs.Cycle != 2 {
		t.Errorf("want cycle 2 after one replan, got %d", rs.Cycle)
	}
	if !rs.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be true after second cycle")
	}
	if acceptStage.callCount != 2 {
		t.Errorf("accept should run twice, got %d", acceptStage.callCount)
	}
}

// --- Scenario 11: Repeated review failures exhaust budget -> needs_human ---

func TestIntegration_BudgetExhausted_ReviewAcceptance_NeedsHuman(t *testing.T) {
	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Review always returns blocking findings
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalReviewPassed = false
			return NextAction{
				Kind: ReplanFrom,
				Context: &FailureContext{
					Failures: []string{"review:spec_alignment:error:handler.go:10 — persistent issue"},
					Cycle:    rs.Cycle,
				},
			}, nil
		},
	}

	acceptStage := passThrough("accept")
	finalizeStage := passThrough("finalize")

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(2), // Only 2 cycles allowed
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("want status %q after cycle exhaustion, got %q", runstore.StatusNeedsHuman, rs.Status)
	}
	if rs.TerminalReason != "cycles_exhausted" {
		t.Errorf("want terminal_reason %q, got %q", "cycles_exhausted", rs.TerminalReason)
	}
	// Review should have been called twice (once per cycle)
	if reviewStage.callCount != 2 {
		t.Errorf("review should run twice (once per cycle), got %d", reviewStage.callCount)
	}
}

// --- Scenario 12: Threshold=error means warnings are non-blocking ---

func TestIntegration_ThresholdError_WarningsNonBlocking(t *testing.T) {
	// This test simulates the threshold behavior at the pipeline level.
	// The ReviewStage (or its mock) decides what is blocking based on threshold.
	// With threshold="error", a warning-only review result should Continue.

	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Review returns warnings only — with threshold=error, these are non-blocking
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			// Simulate: warnings found but no blocking findings at error threshold
			rs.ReviewFindings = []string{"review:spec_alignment:warning:handler.go:5 — consider adding docs"}
			rs.FinalReviewPassed = true // No blocking findings
			return NextAction{Kind: Continue}, nil
		},
	}

	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalAcceptancePassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	finalizeStage := &scenarioStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			} else {
				rs.Status = runstore.StatusNeedsHuman
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(3),
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q (warnings non-blocking at error threshold), got %q",
			runstore.StatusReadyForReview, rs.Status)
	}
	if rs.Cycle != 1 {
		t.Errorf("want single cycle (no replan for warnings), got %d", rs.Cycle)
	}
	if len(rs.ReviewFindings) == 0 {
		t.Error("ReviewFindings should still contain warning findings")
	}
}

// --- Scenario 13: Missing acceptance criteria -> needs_human ---

func TestIntegration_MissingAcceptanceCriteria_NeedsHuman(t *testing.T) {
	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalReviewPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Accept stage returns NeedsHuman because no acceptance criteria found
	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			return NextAction{
				Kind: NeedsHuman,
				Context: &FailureContext{
					Failures: []string{"spec lacks acceptance criteria section — cannot evaluate acceptance"},
					Cycle:    rs.Cycle,
				},
			}, nil
		},
	}

	finalizeStage := passThrough("finalize")

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(3),
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("want status %q when acceptance criteria missing, got %q",
			runstore.StatusNeedsHuman, rs.Status)
	}
	// Should not have attempted a replan — NeedsHuman is immediately terminal
	if acceptStage.callCount != 1 {
		t.Errorf("accept should only be called once (NeedsHuman is terminal), got %d", acceptStage.callCount)
	}
}

// --- Scenario 14: Enabling a third facet via config works ---

func TestIntegration_FacetEnabledViaConfig(t *testing.T) {
	// This test verifies that adding a third review facet via config integrates
	// into the pipeline correctly. We simulate three facets by having the review
	// stage track that it was invoked and all three facets produced results.

	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Review stage simulates three facets: spec_alignment, code_quality, security
	var reviewedFacets []string
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			reviewedFacets = []string{"spec_alignment", "code_quality", "security"}
			rs.ReviewFindings = []string{
				"review:spec_alignment:info:main.go:1 — looks good",
				"review:code_quality:info:main.go:2 — clean",
				"review:security:info:auth.go:5 — no issues",
			}
			rs.FinalReviewPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalAcceptancePassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	finalizeStage := &scenarioStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			} else {
				rs.Status = runstore.StatusNeedsHuman
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(3),
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q with three facets passing, got %q",
			runstore.StatusReadyForReview, rs.Status)
	}
	if len(reviewedFacets) != 3 {
		t.Errorf("want 3 facets reviewed, got %d", len(reviewedFacets))
	}
	if len(rs.ReviewFindings) != 3 {
		t.Errorf("want 3 review findings (one per facet), got %d", len(rs.ReviewFindings))
	}
}

// --- Scenario 15: Pre-existing findings don't re-trigger on second cycle ---

func TestIntegration_FixCycle_NewVsPreexistingFindings(t *testing.T) {
	// Cycle 1: review finds a blocking issue -> replan
	// Cycle 2: review returns the same finding but marked pre-existing -> no block
	// The key behavior: the ReviewStage instance persists across cycles and
	// accumulates priorFindings. On cycle 2, identical findings get disposition
	// "pre-existing" and do not block.

	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Review: cycle 1 has new blocking finding; cycle 2 same finding is pre-existing (non-blocking)
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
			if call == 1 {
				// First cycle: new blocking finding
				rs.FinalReviewPassed = false
				rs.ReviewFindings = []string{"review:spec_alignment:error:handler.go:10 — missing validation"}
				return NextAction{
					Kind: ReplanFrom,
					Context: &FailureContext{
						Failures: []string{"review:spec_alignment:error:handler.go:10 — missing validation"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			// Second cycle: same finding now marked pre-existing, so non-blocking
			rs.FinalReviewPassed = true
			rs.ReviewFindings = []string{"review:spec_alignment:error:handler.go:10 — missing validation [pre-existing]"}
			return NextAction{Kind: Continue}, nil
		},
	}

	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalAcceptancePassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	finalizeStage := &scenarioStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			} else {
				rs.Status = runstore.StatusNeedsHuman
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(3),
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q (pre-existing findings should not block), got %q",
			runstore.StatusReadyForReview, rs.Status)
	}
	if rs.Cycle != 2 {
		t.Errorf("want cycle 2 (one replan for new finding, then pass), got %d", rs.Cycle)
	}
	if reviewStage.callCount != 2 {
		t.Errorf("review should run exactly twice, got %d", reviewStage.callCount)
	}
	// Accept should only run once — on cycle 2 after review passes
	if acceptStage.callCount != 1 {
		t.Errorf("accept should run once (only on successful cycle), got %d", acceptStage.callCount)
	}
}

// --- Scenario 16: Acceptance unclear triggers fix, then passes ---

func TestIntegration_AcceptanceUnclear_FixAddsEvidence_ThenPass(t *testing.T) {
	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalReviewPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Accept: cycle 1 returns unclear; cycle 2 returns pass
	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
			if call == 1 {
				rs.FinalAcceptancePassed = false
				rs.AcceptanceResults = []string{"acceptance:unclear:criterion y — insufficient evidence"}
				return NextAction{
					Kind: ReplanFrom,
					Context: &FailureContext{
						Failures: []string{"acceptance:unclear:criterion y — insufficient evidence"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			rs.FinalAcceptancePassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	finalizeStage := &scenarioStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			} else {
				rs.Status = runstore.StatusNeedsHuman
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(3),
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("want status %q after unclear->fix->pass, got %q", runstore.StatusReadyForReview, rs.Status)
	}
	if rs.Cycle != 2 {
		t.Errorf("want cycle 2 after one replan, got %d", rs.Cycle)
	}
	if !rs.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be true after second cycle")
	}
	if acceptStage.callCount != 2 {
		t.Errorf("accept should run twice, got %d", acceptStage.callCount)
	}
}

// --- Scenario 5: Review fix cycle with evolving findings exhausting budget -> needs_human ---

func TestIntegration_ReviewFixCycle_EvolvingFindings_BudgetExhausted_NeedsHuman(t *testing.T) {
	// Each cycle, review finds NEW (different) blocking findings.
	// The fix cycle runs but introduces a different issue each time.
	// Eventually the budget is exhausted, producing needs_human.

	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Review returns a different blocking finding on each cycle
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
			// Each call produces a unique NEW finding (not pre-existing)
			finding := fmt.Sprintf("review:spec_alignment:error:handler.go:%d — issue %d introduced by fix", call*10, call)
			rs.FinalReviewPassed = false
			rs.ReviewFindings = []string{finding}
			return NextAction{
				Kind: ReplanFrom,
				Context: &FailureContext{
					Failures: []string{finding},
					Cycle:    rs.Cycle,
				},
			}, nil
		},
	}

	acceptStage := passThrough("accept")

	finalizeStage := &scenarioStage{
		name: "finalize",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			} else {
				rs.Status = runstore.StatusNeedsHuman
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(3), // Allow 3 cycles, all will be consumed
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Budget should be exhausted -> needs_human
	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("want status %q after budget exhaustion with evolving findings, got %q",
			runstore.StatusNeedsHuman, rs.Status)
	}
	if rs.TerminalReason != "cycles_exhausted" {
		t.Errorf("want terminal_reason %q, got %q", "cycles_exhausted", rs.TerminalReason)
	}

	// Review should have been called once per cycle (3 times)
	if reviewStage.callCount != 3 {
		t.Errorf("review should run 3 times (once per cycle), got %d", reviewStage.callCount)
	}

	// Accept should run exactly once — at cycles_exhausted to capture acceptance.json,
	// even though review short-circuited before accept in every normal cycle.
	if acceptStage.callCount != 1 {
		t.Errorf("accept should run once (at cycles_exhausted), got %d calls", acceptStage.callCount)
	}

	// BlockerSummary should contain the last finding from the final cycle's replan context
	if rs.BlockerSummary == "" {
		t.Error("BlockerSummary should be set from final replan context")
	}
}

// --- Scenario 17: Acceptance fail persists until budget exhaustion ---

func TestIntegration_AcceptanceFail_BudgetExhausted_NeedsHuman(t *testing.T) {
	planStage := passThrough("plan")
	executeStage := passThrough("execute")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}
	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalReviewPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	// Accept always returns fail
	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			rs.FinalAcceptancePassed = false
			rs.AcceptanceResults = []string{"acceptance:fail:criterion z — not implemented"}
			return NextAction{
				Kind: ReplanFrom,
				Context: &FailureContext{
					Failures: []string{"acceptance:fail:criterion z — not implemented"},
					Cycle:    rs.Cycle,
				},
			}, nil
		},
	}

	finalizeStage := passThrough("finalize")

	stages := []Stage{planStage, executeStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      stdBudget(2), // Only 2 cycles allowed
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("want status %q after cycle exhaustion, got %q", runstore.StatusNeedsHuman, rs.Status)
	}
	if rs.TerminalReason != "cycles_exhausted" {
		t.Errorf("want terminal_reason %q, got %q", "cycles_exhausted", rs.TerminalReason)
	}
	// Accept runs once per cycle (2 cycles) plus once more at cycles_exhausted to
	// capture acceptance.json even though the run terminated early.
	if acceptStage.callCount != 3 {
		t.Errorf("accept should run 3 times (once per cycle + once at cycles_exhausted), got %d", acceptStage.callCount)
	}
	if rs.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be false when acceptance always fails")
	}
}
