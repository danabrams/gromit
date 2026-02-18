package specgate

import (
	"testing"
)

func TestFailedCriteria_returnsOnlyFailedCriteria(t *testing.T) {
	verdict := &GateVerdict{
		Passed: false,
		Results: []CriterionResult{
			{Criterion: "No TODOs", Passed: true, Evidence: "none found"},
			{Criterion: "Tests pass", Passed: false, Evidence: "test output"},
			{Criterion: "No lint errors", Passed: false, Evidence: "lint output"},
		},
	}

	failed := verdict.FailedCriteria()
	if len(failed) != 2 {
		t.Fatalf("FailedCriteria() returned %d items, want 2", len(failed))
	}
	if failed[0].Criterion != "Tests pass" {
		t.Errorf("failed[0].Criterion = %q, want %q", failed[0].Criterion, "Tests pass")
	}
	if failed[1].Criterion != "No lint errors" {
		t.Errorf("failed[1].Criterion = %q, want %q", failed[1].Criterion, "No lint errors")
	}
}

func TestFailedCriteria_allPass_returnsEmpty(t *testing.T) {
	verdict := &GateVerdict{
		Passed: true,
		Results: []CriterionResult{
			{Criterion: "No TODOs", Passed: true, Evidence: "none found"},
		},
	}

	failed := verdict.FailedCriteria()
	if len(failed) != 0 {
		t.Errorf("FailedCriteria() returned %d items, want 0", len(failed))
	}
}

func TestParseVerdict_invalidJSON_returnsError(t *testing.T) {
	_, err := ParseVerdict([]byte(`not json`))
	if err == nil {
		t.Error("ParseVerdict() expected error for invalid JSON, got nil")
	}
}

func TestParseVerdict_validJSON(t *testing.T) {
	input := []byte(`{"passed": true, "results": [{"criterion": "No TODOs", "passed": true, "evidence": "grep found nothing"}]}`)

	verdict, err := ParseVerdict(input)
	if err != nil {
		t.Fatalf("ParseVerdict() error = %v", err)
	}
	if !verdict.Passed {
		t.Errorf("verdict.Passed = false, want true")
	}
	if len(verdict.Results) != 1 {
		t.Fatalf("len(verdict.Results) = %d, want 1", len(verdict.Results))
	}
	if verdict.Results[0].Criterion != "No TODOs" {
		t.Errorf("Results[0].Criterion = %q, want %q", verdict.Results[0].Criterion, "No TODOs")
	}
	if !verdict.Results[0].Passed {
		t.Errorf("Results[0].Passed = false, want true")
	}
	if verdict.Results[0].Evidence != "grep found nothing" {
		t.Errorf("Results[0].Evidence = %q, want %q", verdict.Results[0].Evidence, "grep found nothing")
	}
}
