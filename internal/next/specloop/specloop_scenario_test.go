package specloop

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// --- Test 1: WriteScenarioTests in pipeline stage order ---

func TestIntegration_WriteScenarioTestsInPipeline(t *testing.T) {
	// Verify that WriteScenarioTests runs after Execute and before Validate.
	// We'll track the order of stage calls using a call sequence.
	var stageCallSequence []string

	initStage := &scenarioStage{
		name: "init",
		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
			stageCallSequence = append(stageCallSequence, "init")
			return NextAction{Kind: Continue}, nil
		},
	}

	executeStage := &scenarioStage{
		name: "execute",
		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
			stageCallSequence = append(stageCallSequence, "execute")
			return NextAction{Kind: Continue}, nil
		},
	}

	writeScenarioTestsStage := &scenarioStage{
		name: "write_scenario_tests",
		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
			stageCallSequence = append(stageCallSequence, "write_scenario_tests")
			return NextAction{Kind: Continue}, nil
		},
	}

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			stageCallSequence = append(stageCallSequence, "validate")
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	reviewStage := &scenarioStage{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			stageCallSequence = append(stageCallSequence, "review")
			rs.FinalReviewPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	acceptStage := &scenarioStage{
		name: "accept",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			stageCallSequence = append(stageCallSequence, "accept")
			rs.FinalAcceptancePassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{initStage, executeStage, writeScenarioTestsStage, validateStage, reviewStage, acceptStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget: NewBudget(execpolicy.Budgets{
			MaxSpecCycles:          1,
			MaxRunCostUSD:          99,
			MaxRunDurationSeconds:  3600,
			MaxTaskDurationSeconds: 300,
		}),
		ReplanStage: "execute",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify that write_scenario_tests appears after execute and before validate.
	executeIdx := -1
	writeIdx := -1
	validateIdx := -1
	for i, stage := range stageCallSequence {
		if stage == "execute" {
			executeIdx = i
		}
		if stage == "write_scenario_tests" {
			writeIdx = i
		}
		if stage == "validate" {
			validateIdx = i
		}
	}

	if executeIdx < 0 {
		t.Error("execute stage was not called")
	}
	if writeIdx < 0 {
		t.Error("write_scenario_tests stage was not called")
	}
	if validateIdx < 0 {
		t.Error("validate stage was not called")
	}

	if executeIdx >= writeIdx {
		t.Errorf("execute should run before write_scenario_tests: execute_idx=%d, write_idx=%d", executeIdx, writeIdx)
	}
	if writeIdx >= validateIdx {
		t.Errorf("write_scenario_tests should run before validate: write_idx=%d, validate_idx=%d", writeIdx, validateIdx)
	}
}

// --- Test 2: ScenarioTestReplan preserves tests (no-op on second cycle) ---

func TestIntegration_ScenarioTestReplanPreservesTests(t *testing.T) {
	// Verify that on a replan cycle, WriteScenarioTests is a no-op when
	// ScenarioTestsWritten is already true. We track how many times the
	// write_scenario_tests stage actually performs work.
	writeWorkCount := 0

	executeStage := &scenarioStage{
		name: "execute",
		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
			return NextAction{Kind: Continue}, nil
		},
	}

	writeScenarioTestsStage := &scenarioStage{
		name: "write_scenario_tests",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			// If not yet written, mark it and count the work
			if !rs.ScenarioTestsWritten {
				writeWorkCount++
				rs.ScenarioTestsWritten = true
				return NextAction{Kind: Continue}, nil
			}
			// Already written: idempotent no-op
			return NextAction{Kind: Continue}, nil
		},
	}

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
			rs.FinalValidationPassed = true
			// On first cycle, trigger replan to test that write_scenario_tests is idempotent
			if call == 1 {
				return NextAction{
					Kind: ReplanFrom,
					Context: &FailureContext{
						Failures: []string{"--- FAIL: TestSomeTest"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			// On second cycle, validation passes
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
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{executeStage, writeScenarioTestsStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget: NewBudget(execpolicy.Budgets{
			MaxSpecCycles:          3,
			MaxRunCostUSD:          99,
			MaxRunDurationSeconds:  3600,
			MaxTaskDurationSeconds: 300,
		}),
		ReplanStage: "execute",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// ScenarioTestsWritten should be set after first cycle
	if !rs.ScenarioTestsWritten {
		t.Error("ScenarioTestsWritten should be true after first cycle")
	}

	// writeWorkCount should be 1 (only performed work once, on first cycle)
	// Cycle 2 should see ScenarioTestsWritten=true and skip work
	if writeWorkCount != 1 {
		t.Errorf("write_scenario_tests should only perform work once (idempotent), got count %d", writeWorkCount)
	}

	// Second cycle should have run (verified by rs.Cycle)
	if rs.Cycle < 2 {
		t.Errorf("expected at least 2 cycles (replan cycle), got %d", rs.Cycle)
	}
}

// --- Test 3: PersistentFailureHint after two consecutive cycles ---

func TestIntegration_PersistentFailureHintAfterTwoCycles(t *testing.T) {
	// Verify that FailureHistory accumulates across cycles and annotates
	// failures with persistent-failure hints after 2 consecutive cycles.
	// Uses contract: failure format which is properly extracted by ExtractContractFailureKeys.

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
		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
			// All cycles: review finds a blocking failure using contract: format
			// Contract failures are properly extracted for FailureHistory tracking
			rs.FinalReviewPassed = false
			rs.ReviewFindings = []string{"contract:UserCreation — missing validation"}
			return NextAction{
				Kind: ReplanFrom,
				Context: &FailureContext{
					Failures: []string{"contract:UserCreation — missing validation"},
					Cycle:    rs.Cycle,
				},
			}, nil
		},
	}

	acceptStage := passThrough("accept")

	stages := []Stage{executeStage, validateStage, reviewStage, acceptStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget: NewBudget(execpolicy.Budgets{
			MaxSpecCycles:          3,
			MaxRunCostUSD:          99,
			MaxRunDurationSeconds:  3600,
			MaxTaskDurationSeconds: 300,
		}),
		ReplanStage: "execute",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// After 2+ cycles with the same contract failure, the replan context should have been
	// annotated with a persistent-failure hint (threshold is 2).
	// The hint should be appended to the annotated failures.

	foundPersistentFailureHint := false
	if rs.ReplanContext != nil {
		for _, failure := range rs.ReplanContext.Failures {
			if contains(failure, "persistent-failure:") {
				foundPersistentFailureHint = true
				// Verify the hint mentions the failure key and has "failed" and "consecutive cycles"
				if !contains(failure, "contract:UserCreation") {
					t.Errorf("persistent-failure hint should mention contract:UserCreation, got: %s", failure)
				}
				if !contains(failure, "consecutive cycles") {
					t.Errorf("persistent-failure hint should mention 'consecutive cycles', got: %s", failure)
				}
				break
			}
		}
	}

	if !foundPersistentFailureHint {
		replanContextFailures := []string{}
		if rs.ReplanContext != nil {
			replanContextFailures = rs.ReplanContext.Failures
		}
		t.Errorf("ReplanContext should contain persistent-failure hint after 2 consecutive cycles. Got: %v", replanContextFailures)
	}

	// FailureHistory should be tracking the contract key
	if len(rs.FailureHistory) == 0 {
		t.Error("FailureHistory should be populated with contract:UserCreation key")
	}

	// Check that the extracted key is in history with a count >= 2
	if count, found := rs.FailureHistory["contract:UserCreation"]; !found || count < 2 {
		t.Errorf("contract:UserCreation should have count >= 2 in FailureHistory, got: %d (found: %v)", count, found)
	}

	// Verify that budget exhaustion occurred after 3 cycles (as expected with persistent failures)
	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("expected status %q after cycles exhausted, got %q", runstore.StatusNeedsHuman, rs.Status)
	}
}

// --- Test 4: ScenarioTestsWritten not reset per cycle ---

func TestIntegration_ScenarioTestsWrittenNotResetPerCycle(t *testing.T) {
	// Verify that ResetForNewCycle does not reset ScenarioTestsWritten or FailureHistory.
	// These fields should persist across cycles.

	executeStage := &scenarioStage{
		name: "execute",
		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
			return NextAction{Kind: Continue}, nil
		},
	}

	writeScenarioTestsStage := &scenarioStage{
		name: "write_scenario_tests",
		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
			if !rs.ScenarioTestsWritten {
				rs.ScenarioTestsWritten = true
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
			rs.FinalValidationPassed = true
			// Cycles 1-2: trigger replan to test persistence
			if call <= 2 {
				return NextAction{
					Kind: ReplanFrom,
					Context: &FailureContext{
						Failures: []string{"--- FAIL: TestValidation"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			// Cycle 3: pass to allow completion
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

	stages := []Stage{executeStage, writeScenarioTestsStage, validateStage, reviewStage, acceptStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget: NewBudget(execpolicy.Budgets{
			MaxSpecCycles:          3,
			MaxRunCostUSD:          99,
			MaxRunDurationSeconds:  3600,
			MaxTaskDurationSeconds: 300,
		}),
		ReplanStage: "execute",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// After multiple cycles, ScenarioTestsWritten should still be true
	if !rs.ScenarioTestsWritten {
		t.Error("ScenarioTestsWritten should remain true across replan cycles (not reset by ResetForNewCycle)")
	}

	// FailureHistory should accumulate across cycles with the same failure key
	if len(rs.FailureHistory) == 0 {
		t.Error("FailureHistory should be non-empty after multiple cycles with failures")
	}

	// Verify that the failure key "TestValidation" is in the history
	testValidationCount := rs.FailureHistory["TestValidation"]
	if testValidationCount == 0 {
		t.Errorf("TestValidation failure should be tracked in FailureHistory, got count %d", testValidationCount)
	}

	// Cycle count should show we had replans
	if rs.Cycle < 3 {
		t.Errorf("expected at least 3 cycles with 2 replan cycles, got %d", rs.Cycle)
	}
}

// --- Test 5: ScenarioTestFailureTriggersReplan ---

func TestIntegration_ScenarioTestFailureTriggersReplan(t *testing.T) {
	// Track how many times execute ran (to verify replan happened)
	executeCount := 0

	executeStage := &scenarioStage{
		name: "execute",
		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
			executeCount++
			return NextAction{Kind: Continue}, nil
		},
	}

	writeScenarioTestsStage := passThrough("write_scenario_tests")

	validateStage := &scenarioStage{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
			rs.FinalValidationPassed = true
			// First call: scenario test fails → replan
			if call == 1 {
				return NextAction{
					Kind: ReplanFrom,
					Context: &FailureContext{
						Failures: []string{"--- FAIL: TestScenario_Divide_ReturnsFloat64 (0.00s)"},
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			// Second call: test passes
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
			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
				rs.Status = runstore.StatusReadyForReview
			}
			return NextAction{Kind: Continue}, nil
		},
	}

	stages := []Stage{executeStage, writeScenarioTestsStage, validateStage, reviewStage, acceptStage, finalizeStage}
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget: NewBudget(execpolicy.Budgets{
			MaxSpecCycles:          3,
			MaxRunCostUSD:          99,
			MaxRunDurationSeconds:  3600,
			MaxTaskDurationSeconds: 300,
		}),
		ReplanStage: "execute",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify replan happened: execute ran twice (once per cycle)
	if executeCount != 2 {
		t.Errorf("expected execute to run 2 times (initial + replan), got %d", executeCount)
	}

	// Verify final status is ready_for_review (pipeline completed successfully)
	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("expected status %q, got %q", runstore.StatusReadyForReview, rs.Status)
	}

	// Verify FailureHistory tracked the test failure key
	if _, found := rs.FailureHistory["TestScenario_Divide_ReturnsFloat64"]; !found {
		t.Error("expected TestScenario_Divide_ReturnsFloat64 to be in FailureHistory")
	}
}

// --- Helper function ---

// contains checks if a substring exists in a string
func contains(s, substr string) bool {
	// Simple string containment check
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
