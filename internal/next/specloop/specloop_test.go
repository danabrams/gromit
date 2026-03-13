package specloop

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestSpecLoop_RunsStagesInOrder(t *testing.T) {
	var order []string
	stages := []Stage{
		&recordStage{name: "init", order: &order},
		&recordStage{name: "compile", order: &order},
		&recordStage{name: "plan", order: &order},
		&recordStage{name: "finalize", order: &order},
	}
	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"init", "compile", "plan", "finalize"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("want %v, got %v", want, order)
	}
}

func TestSpecLoop_StageError_SetsBlockedAndRunsEvidence(t *testing.T) {
	evidenceRan := false
	stages := []Stage{
		&recordStage{name: "init", order: new([]string)},
		&errorStage{name: "plan", err: fmt.Errorf("infra failure")},
		&recordStage{name: "execute", order: new([]string)},
		&callbackStage{name: "evidence", fn: func() { evidenceRan = true }},
		&recordStage{name: "finalize", order: new([]string)},
	}
	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("SpecLoop should not propagate stage errors: %v", err)
	}
	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("want blocked, got %s", rs.Status)
	}
	if !evidenceRan {
		t.Fatal("evidence stage should still run on error")
	}
}

type recordStage struct {
	name  string
	order *[]string
}

func (s *recordStage) Name() string { return s.name }
func (s *recordStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	*s.order = append(*s.order, s.name)
	return NextAction{Kind: Continue}, nil
}

type errorStage struct {
	name string
	err  error
}

func (s *errorStage) Name() string { return s.name }
func (s *errorStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	return NextAction{}, s.err
}

type callbackStage struct {
	name string
	fn   func()
}

func (s *callbackStage) Name() string { return s.name }
func (s *callbackStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	s.fn()
	return NextAction{Kind: Continue}, nil
}

type countStage struct {
	name   string
	counts map[string]int
}

func (s *countStage) Name() string { return s.name }
func (s *countStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	s.counts[s.name]++
	return NextAction{Kind: Continue}, nil
}

type actionStage struct {
	name     string
	actionFn func() NextAction
}

func (s *actionStage) Name() string { return s.name }
func (s *actionStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	return s.actionFn(), nil
}

func TestSpecLoop_ReplanFromLoopsBack(t *testing.T) {
	callCounts := map[string]int{}
	validate := &actionStage{name: "validate", actionFn: func() NextAction {
		callCounts["validate"]++
		if callCounts["validate"] == 1 {
			return NextAction{Kind: ReplanFrom, Context: &FailureContext{Failures: []string{"lint fail"}}}
		}
		return NextAction{Kind: Continue}
	}}
	stages := []Stage{
		&countStage{name: "init", counts: callCounts},
		&countStage{name: "plan", counts: callCounts},
		&countStage{name: "execute", counts: callCounts},
		validate,
		&countStage{name: "finalize", counts: callCounts},
	}
	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 3, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, ReplanStage: "plan"})
	rs := runstore.NewRunState("s1", "p1")
	loop.Run(context.Background(), rs)

	if callCounts["plan"] != 2 {
		t.Fatalf("plan should run twice, got %d", callCounts["plan"])
	}
	if callCounts["finalize"] != 1 {
		t.Fatal("finalize should run once after second pass succeeds")
	}
	if callCounts["init"] != 1 {
		t.Fatalf("init should run once (only on cycle 1), got %d", callCounts["init"])
	}
}

func TestSpecLoop_BudgetBlocksBetweenStages(t *testing.T) {
	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})
	budget.AddCost(2.0) // exceed cost budget

	var order []string
	stages := []Stage{
		&recordStage{name: "init", order: &order},
		&recordStage{name: "plan", order: &order},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget})
	rs := runstore.NewRunState("s1", "p1")
	loop.Run(context.Background(), rs)

	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("want blocked, got %s", rs.Status)
	}
	if rs.TerminalReason != "budget_exceeded" {
		t.Fatalf("want budget_exceeded, got %s", rs.TerminalReason)
	}
	if len(order) != 0 {
		t.Fatalf("no stages should run when budget already exceeded, got %v", order)
	}
}

func TestSpecLoop_CycleExhaustion_SetsNeedsHuman(t *testing.T) {
	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99})

	stages := []Stage{
		&actionStage{name: "validate", actionFn: func() NextAction {
			return NextAction{Kind: ReplanFrom}
		}},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, ReplanStage: "validate"})
	rs := runstore.NewRunState("s1", "p1")
	loop.Run(context.Background(), rs)

	if rs.Status != runstore.StatusNeedsHuman {
		t.Fatalf("want needs_human, got %s", rs.Status)
	}
	if rs.TerminalReason != "cycles_exhausted" {
		t.Fatalf("want cycles_exhausted, got %s", rs.TerminalReason)
	}
}

func TestSpecLoop_BudgetExceeded_StillRunsEvidence(t *testing.T) {
	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})
	budget.AddCost(2.0) // exceed cost budget before first stage

	evidenceRan := false
	stages := []Stage{
		&recordStage{name: "init", order: new([]string)},
		&callbackStage{name: "evidence", fn: func() { evidenceRan = true }},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget})
	rs := runstore.NewRunState("s1", "p1")
	loop.Run(context.Background(), rs)

	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("want blocked, got %s", rs.Status)
	}
	if rs.TerminalReason != "budget_exceeded" {
		t.Fatalf("want budget_exceeded, got %s", rs.TerminalReason)
	}
	if !evidenceRan {
		t.Fatal("evidence stage should run even when budget is exceeded")
	}
}

type mockStage struct {
	name  string
	runFn func(ctx context.Context, rs *runstore.RunState) (NextAction, error)
}

func (s *mockStage) Name() string { return s.name }
func (s *mockStage) Run(ctx context.Context, rs *runstore.RunState) (NextAction, error) {
	return s.runFn(ctx, rs)
}

func TestSpecLoop_CycleResetsGateFields(t *testing.T) {
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.FinalValidationPassed = true
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = true
	rs.ReviewFindings = []string{"prior finding 1"}
	rs.AcceptanceResults = []string{"prior result 1"}

	var capturedRS *runstore.RunState
	captureStage := &mockStage{
		name: "capture",
		runFn: func(ctx context.Context, rs *runstore.RunState) (NextAction, error) {
			capturedRS = rs
			return NextAction{Kind: Continue}, nil
		},
	}

	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0})
	loop := NewSpecLoop([]Stage{captureStage}, SpecLoopConfig{
		Budget: budget,
	})

	loop.Run(context.Background(), rs)

	if capturedRS.FinalValidationPassed {
		t.Error("FinalValidationPassed should be reset to false at cycle start")
	}
	if capturedRS.FinalReviewPassed {
		t.Error("FinalReviewPassed should be reset to false at cycle start")
	}
	if capturedRS.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be reset to false at cycle start")
	}
	if len(capturedRS.ReviewFindings) != 0 {
		t.Errorf("ReviewFindings should be empty at cycle start, got %v", capturedRS.ReviewFindings)
	}
	if len(capturedRS.AcceptanceResults) != 0 {
		t.Errorf("AcceptanceResults should be empty at cycle start, got %v", capturedRS.AcceptanceResults)
	}
}

func TestSpecLoop_CycleReset_ValidateStageResetsGateAfterReset(t *testing.T) {
	validateStub := &mockStage{
		name: "validate",
		runFn: func(ctx context.Context, rs *runstore.RunState) (NextAction, error) {
			rs.FinalValidationPassed = true
			return NextAction{Kind: Continue}, nil
		},
	}

	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0})
	loop := NewSpecLoop([]Stage{validateStub}, SpecLoopConfig{
		Budget: budget,
	})
	rs := &runstore.RunState{FinalValidationPassed: true}

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rs.FinalValidationPassed {
		t.Error("ValidateStage should re-set FinalValidationPassed after cycle reset")
	}
}

// callCountStage is a mock stage that tracks invocation count on the struct,
// providing a reliable way to verify the same instance is reused across cycles.
type callCountStage struct {
	name      string
	callCount int
	runFn     func(ctx context.Context, rs *runstore.RunState) (NextAction, error)
}

func (s *callCountStage) Name() string { return s.name }
func (s *callCountStage) Run(ctx context.Context, rs *runstore.RunState) (NextAction, error) {
	return s.runFn(ctx, rs)
}

func TestSpecLoop_ReusesStageInstances(t *testing.T) {
	trackingStage := &callCountStage{name: "tracker"}
	trackingStage.runFn = func(ctx context.Context, rs *runstore.RunState) (NextAction, error) {
		trackingStage.callCount++
		if trackingStage.callCount < 2 {
			return NextAction{Kind: ReplanFrom}, nil
		}
		return NextAction{Kind: Continue}, nil
	}

	var capturedAddr uintptr
	origRunFn := trackingStage.runFn
	trackingStage.runFn = func(ctx context.Context, rs *runstore.RunState) (NextAction, error) {
		thisAddr := reflect.ValueOf(trackingStage).Pointer()
		if trackingStage.callCount == 0 {
			capturedAddr = thisAddr
		} else if thisAddr != capturedAddr {
			t.Error("stage instance changed between cycles — must reuse same instance")
		}
		return origRunFn(ctx, rs)
	}

	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 3, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0})
	loop := NewSpecLoop([]Stage{trackingStage}, SpecLoopConfig{
		Budget: budget,
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	loop.Run(context.Background(), rs)

	if trackingStage.callCount < 2 {
		t.Fatalf("expected at least 2 calls on same instance, got %d", trackingStage.callCount)
	}
}

func TestSpecLoop_CycleExhaustion_RunsEvidence(t *testing.T) {
	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99})

	evidenceRan := false
	stages := []Stage{
		&actionStage{name: "validate", actionFn: func() NextAction {
			return NextAction{Kind: ReplanFrom}
		}},
		&callbackStage{name: "evidence", fn: func() { evidenceRan = true }},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, ReplanStage: "validate"})
	rs := runstore.NewRunState("s1", "p1")
	loop.Run(context.Background(), rs)

	if rs.Status != runstore.StatusNeedsHuman {
		t.Fatalf("want needs_human, got %s", rs.Status)
	}
	if !evidenceRan {
		t.Fatal("evidence stage should run on cycle exhaustion")
	}
}
