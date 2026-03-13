package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestIntegration_AcceptFailTriggersReplan(t *testing.T) {
	planCallCount := 0
	planStage := &callbackStageInteg{
		name: "plan",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			planCallCount++
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}

	validateStage := &callbackStageInteg{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			rs.FinalValidationPassed = true
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}
	reviewStage := &callbackStageInteg{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			rs.FinalReviewPassed = true
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}

	// First call: fail. Second call: pass.
	acceptCallCount := 0
	acceptEval := &callbackAcceptEvaluator{
		fn: func(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
			acceptCallCount++
			if acceptCallCount == 1 {
				return acceptor.AcceptanceResult{
					Results:          []acceptor.CriterionResult{{Criterion: "x", Status: acceptor.StatusFail, Rationale: "not done"}},
					HasFailOrUnclear: true,
				}, nil
			}
			return acceptor.AcceptanceResult{
				Results: []acceptor.CriterionResult{{Criterion: "x", Status: acceptor.StatusPass}},
				AllPass: true,
			}, nil
		},
	}

	acceptStage := NewAcceptStage(acceptEval, AcceptStageConfig{Criteria: []string{"x"}}, nil)

	stages := []specloop.Stage{planStage, validateStage, reviewStage, acceptStage}
	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 3, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0})
	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("loop.Run: %v", err)
	}

	if planCallCount < 2 {
		t.Errorf("expected plan to be called at least 2 times (initial + replan), got %d", planCallCount)
	}
	if !rs.FinalAcceptancePassed {
		t.Error("expected FinalAcceptancePassed == true after successful acceptance on second cycle")
	}
}

func TestIntegration_AcceptUnclear_BudgetExhaustion_NeedsHuman(t *testing.T) {
	planStage := &callbackStageInteg{
		name: "plan",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}
	validateStage := &callbackStageInteg{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			rs.FinalValidationPassed = true
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}
	reviewStage := &callbackStageInteg{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			rs.FinalReviewPassed = true
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}

	// Always returns unclear
	acceptEval := &callbackAcceptEvaluator{
		fn: func(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
			return acceptor.AcceptanceResult{
				Results: []acceptor.CriterionResult{
					{Criterion: "audit log", Status: acceptor.StatusUnclear, Rationale: "no test verifies it"},
				},
				HasFailOrUnclear: true,
			}, nil
		},
	}

	acceptStage := NewAcceptStage(acceptEval, AcceptStageConfig{Criteria: []string{"audit log"}}, nil)

	stages := []specloop.Stage{planStage, validateStage, reviewStage, acceptStage}
	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 2, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0})
	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("loop.Run: %v", err)
	}

	if rs.FinalAcceptancePassed {
		t.Error("expected FinalAcceptancePassed == false when acceptance remains unclear")
	}
	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("expected needs_human after budget exhaustion with unclear criteria, got %q", rs.Status)
	}
}

func TestIntegration_AcceptStage_EmptyCriteriaSection_NeedsHuman(t *testing.T) {
	planStage := &callbackStageInteg{
		name: "plan",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}
	validateStage := &callbackStageInteg{
		name: "validate",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			rs.FinalValidationPassed = true
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}
	reviewStage := &callbackStageInteg{
		name: "review",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			rs.FinalReviewPassed = true
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}

	// AcceptStage with spec content that has heading but no bullet points
	acceptEval := &callbackAcceptEvaluator{
		fn: func(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
			// Should not be called — empty criteria triggers NeedsHuman before evaluation
			t.Error("evaluator should not be called when criteria section is empty")
			return acceptor.AcceptanceResult{}, nil
		},
	}

	acceptStage := NewAcceptStage(acceptEval, AcceptStageConfig{
		SpecContent: "# Spec\n\n## Acceptance Criteria\n\n## Notes\nSome notes here.\n",
	}, nil)

	stages := []specloop.Stage{planStage, validateStage, reviewStage, acceptStage}
	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 2, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0})
	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("loop.Run: %v", err)
	}

	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("expected needs_human when acceptance criteria section is empty, got %q", rs.Status)
	}
}

type callbackStageInteg struct {
	name string
	fn   func(context.Context, *runstore.RunState) (specloop.NextAction, error)
}

func (s *callbackStageInteg) Name() string { return s.name }
func (s *callbackStageInteg) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	return s.fn(ctx, rs)
}

type callbackAcceptEvaluator struct {
	fn func(context.Context, acceptor.EvaluateInput) (acceptor.AcceptanceResult, error)
}

func (c *callbackAcceptEvaluator) Evaluate(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
	return c.fn(ctx, input)
}
