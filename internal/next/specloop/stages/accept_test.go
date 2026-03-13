package stages

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type mockAcceptEvaluator struct {
	results []acceptor.AcceptanceResult
	errs    []error
	calls   int
}

func (m *mockAcceptEvaluator) Evaluate(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
	idx := m.calls
	m.calls++
	if idx < len(m.errs) && m.errs[idx] != nil {
		return acceptor.AcceptanceResult{}, m.errs[idx]
	}
	if idx < len(m.results) {
		return m.results[idx], nil
	}
	return m.results[len(m.results)-1], nil
}

func TestAcceptStage_Name(t *testing.T) {
	s := NewAcceptStage(nil, AcceptStageConfig{}, nil)
	if s.Name() != "accept" {
		t.Errorf("Name() = %q, want %q", s.Name(), "accept")
	}
}

func TestAcceptStage_AllPass_Continue(t *testing.T) {
	eval := &mockAcceptEvaluator{
		results: []acceptor.AcceptanceResult{{
			Results: []acceptor.CriterionResult{
				{Criterion: "returns 200", Status: acceptor.StatusPass},
			},
			AllPass:          true,
			HasFailOrUnclear: false,
		}},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"returns 200"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalAcceptancePassed {
		t.Error("expected FinalAcceptancePassed = true")
	}
}

func TestAcceptStage_Fail_ReplanFrom(t *testing.T) {
	eval := &mockAcceptEvaluator{
		results: []acceptor.AcceptanceResult{{
			Results: []acceptor.CriterionResult{
				{Criterion: "multi-currency", Status: acceptor.StatusFail, Rationale: "only USD"},
			},
			AllPass:          false,
			HasFailOrUnclear: true,
		}},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"multi-currency"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Errorf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext")
	}
	if rs.FinalAcceptancePassed {
		t.Error("expected FinalAcceptancePassed = false")
	}
}

func TestAcceptStage_Unclear_ReplanFrom(t *testing.T) {
	eval := &mockAcceptEvaluator{
		results: []acceptor.AcceptanceResult{{
			Results: []acceptor.CriterionResult{
				{Criterion: "audit log", Status: acceptor.StatusUnclear, Rationale: "no test verifies it"},
			},
			AllPass:          false,
			HasFailOrUnclear: true,
		}},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"audit log"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Errorf("unclear should trigger ReplanFrom, got %v", action.Kind)
	}
}

func TestAcceptStage_RetriesOnAPIFailure(t *testing.T) {
	eval := &mockAcceptEvaluator{
		results: []acceptor.AcceptanceResult{
			{}, // placeholder for error call
			{
				Results: []acceptor.CriterionResult{
					{Criterion: "test", Status: acceptor.StatusPass},
				},
				AllPass:          true,
				HasFailOrUnclear: false,
			},
		},
		errs: []error{
			errors.New("API timeout"),
			nil,
		},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"test"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run should succeed on retry: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue after retry, got %v", action.Kind)
	}
	if eval.calls != 2 {
		t.Errorf("expected 2 evaluator calls, got %d", eval.calls)
	}
}

func TestAcceptStage_StoresResultsInRunState(t *testing.T) {
	eval := &mockAcceptEvaluator{
		results: []acceptor.AcceptanceResult{{
			Results: []acceptor.CriterionResult{
				{Criterion: "returns 200", Status: acceptor.StatusPass, Rationale: "test proves it"},
			},
			AllPass:          true,
			HasFailOrUnclear: false,
		}},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"returns 200"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !rs.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be true when all pass")
	}
	// AcceptanceResults may be empty when all pass (no failures to report)
}

func TestAcceptStage_StoresFailuresInRunState(t *testing.T) {
	eval := &mockAcceptEvaluator{
		results: []acceptor.AcceptanceResult{{
			Results: []acceptor.CriterionResult{
				{Criterion: "multi-currency", Status: acceptor.StatusFail, Rationale: "only USD"},
			},
			HasFailOrUnclear: true,
		}},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"multi-currency"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be false on failure")
	}
	if len(rs.AcceptanceResults) == 0 {
		t.Error("AcceptanceResults should contain failure strings")
	}
}

func TestAcceptStage_EmitsEvent(t *testing.T) {
	eval := &mockAcceptEvaluator{
		results: []acceptor.AcceptanceResult{{
			Results: []acceptor.CriterionResult{
				{Criterion: "x", Status: acceptor.StatusPass},
			},
			AllPass: true,
		}},
	}

	eventLog := runstore.NewEventLog(filepath.Join(t.TempDir(), "events.jsonl"))
	stage := NewAcceptStage(eval, AcceptStageConfig{Criteria: []string{"x"}}, eventLog)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].EventType() != "acceptance_result" {
		t.Errorf("event type = %q, want acceptance_result", events[0].EventType())
	}
}

func TestAcceptStage_NoCriteria_ParsesFromSpec(t *testing.T) {
	eval := &mockAcceptEvaluator{
		results: []acceptor.AcceptanceResult{{
			Results: []acceptor.CriterionResult{
				{Criterion: "Endpoint returns 200", Status: acceptor.StatusPass},
			},
			AllPass:          true,
			HasFailOrUnclear: false,
		}},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		// No Criteria set — should parse from SpecContent
		SpecContent: "# Spec\n\n## Acceptance Criteria\n- Endpoint returns 200\n\n## Notes\n",
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}
}

func TestAcceptStage_NoCriteria_NoSection_NeedsHuman(t *testing.T) {
	stage := NewAcceptStage(&mockAcceptEvaluator{
		results: []acceptor.AcceptanceResult{{}},
	}, AcceptStageConfig{
		SpecContent: "# Spec\n\nNo acceptance criteria here.\n",
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.NeedsHuman {
		t.Errorf("expected NeedsHuman when spec lacks criteria, got %v", action.Kind)
	}
}
