package acceptor

import "testing"

func TestBuildFailureContext_FailAndUnclear(t *testing.T) {
	results := AcceptanceResult{
		Results: []CriterionResult{
			{Criterion: "returns 200", Status: StatusPass, Rationale: "test proves it"},
			{Criterion: "multi-currency", Status: StatusFail, Rationale: "only USD"},
			{Criterion: "audit log", Status: StatusUnclear, Rationale: "no test"},
		},
	}

	failures := BuildFailureContext(results, 2)
	if len(failures) != 2 {
		t.Fatalf("expected 2 failure contexts, got %d", len(failures))
	}
	if failures[0].Criterion != "multi-currency" {
		t.Errorf("first failure should be multi-currency, got %q", failures[0].Criterion)
	}
	if failures[0].Status != StatusFail {
		t.Errorf("first failure status = %q, want %q", failures[0].Status, StatusFail)
	}
}

func TestBuildFailureContext_AllPass_Empty(t *testing.T) {
	results := AcceptanceResult{
		Results: []CriterionResult{
			{Criterion: "returns 200", Status: StatusPass},
		},
	}

	failures := BuildFailureContext(results, 1)
	if len(failures) != 0 {
		t.Errorf("expected 0 failures for all-pass, got %d", len(failures))
	}
}

func TestAcceptanceFailuresToStrings_FailAndUnclear(t *testing.T) {
	results := []CriterionResult{
		{Criterion: "multi-currency", Status: StatusFail, Rationale: "only USD"},
		{Criterion: "audit log", Status: StatusUnclear, Rationale: "no test"},
		{Criterion: "returns 200", Status: StatusPass, Rationale: "test proves it"},
	}
	strs := AcceptanceFailuresToStrings(results)
	if len(strs) != 2 {
		t.Fatalf("expected 2 strings (skip pass), got %d", len(strs))
	}
	// fail format: "acceptance:fail: <criterion> — implement missing behavior"
	if !containsSubstring(strs[0], "acceptance:fail:") || !containsSubstring(strs[0], "multi-currency") {
		t.Errorf("fail string format wrong: %q", strs[0])
	}
	// unclear format: "acceptance:unclear: <criterion> — add tests or evidence to prove/disprove"
	if !containsSubstring(strs[1], "acceptance:unclear:") || !containsSubstring(strs[1], "audit log") {
		t.Errorf("unclear string format wrong: %q", strs[1])
	}
}
