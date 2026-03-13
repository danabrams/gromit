package acceptor

import (
	"context"
	"testing"
)

type mockAcceptAgent struct {
	results map[string]CriterionResult
}

func (m *mockAcceptAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	// Extract criterion from prompt (simplified: return based on first match)
	for criterion, result := range m.results {
		if containsSubstring(prompt, criterion) {
			return result, nil
		}
	}
	return CriterionResult{Status: StatusPass, Rationale: "default pass"}, nil
}

func TestEvaluator_AllPass(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"returns 200":    {Criterion: "returns 200", Status: StatusPass, Rationale: "test proves it"},
			"handles errors": {Criterion: "handles errors", Status: StatusPass, Rationale: "error tests exist"},
		},
	}

	eval := NewEvaluator(agent)
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"returns 200", "handles errors"},
		DiffSummary: "Added handler",
		TaskResults: "all passed",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.AllPass {
		t.Error("expected AllPass=true")
	}
	if result.HasFailOrUnclear {
		t.Error("expected HasFailOrUnclear=false")
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
}

func TestEvaluator_FailTriggersReplan(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"multi-currency": {Criterion: "multi-currency", Status: StatusFail, Rationale: "only USD implemented"},
		},
	}

	eval := NewEvaluator(agent)
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"multi-currency"},
		DiffSummary: "Added handler",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.AllPass {
		t.Error("expected AllPass=false")
	}
	if !result.HasFailOrUnclear {
		t.Error("expected HasFailOrUnclear=true")
	}
}

func TestEvaluator_UnclearTriggersReplan(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"audit log": {Criterion: "audit log", Status: StatusUnclear, Rationale: "no test verifies audit call"},
		},
	}

	eval := NewEvaluator(agent)
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"audit log"},
		DiffSummary: "Added handler",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.HasFailOrUnclear {
		t.Error("unclear should set HasFailOrUnclear=true")
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
