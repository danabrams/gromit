package specgate

import (
	"testing"
)

func TestParseVerdict_validMixedPassFailJSON(t *testing.T) {
	input := []byte(`{
		"passed": false,
		"results": [
			{"criterion": "No TODOs", "passed": true, "evidence": "none found"},
			{"criterion": "Tests pass", "passed": false, "evidence": "1 test failed"}
		]
	}`)

	verdict, err := ParseVerdict(input)
	if err != nil {
		t.Fatalf("ParseVerdict() error = %v", err)
	}
	if verdict.Passed {
		t.Errorf("verdict.Passed = true, want false")
	}
	if len(verdict.Results) != 2 {
		t.Fatalf("len(verdict.Results) = %d, want 2", len(verdict.Results))
	}

	failed := verdict.FailedCriteria()
	if len(failed) != 1 {
		t.Fatalf("len(FailedCriteria()) = %d, want 1", len(failed))
	}
	if failed[0].Criterion != "Tests pass" {
		t.Errorf("failed[0].Criterion = %q, want %q", failed[0].Criterion, "Tests pass")
	}
}

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

func TestFailedCriteria_nilVerdict_returnsEmpty(t *testing.T) {
	var verdict *GateVerdict

	failed := verdict.FailedCriteria()
	if failed == nil {
		t.Fatal("FailedCriteria() returned nil, want empty slice")
	}
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

func TestParseVerdict_missingFields_usesZeroValuesAndNormalizesSlices(t *testing.T) {
	verdict, err := ParseVerdict([]byte(`{"passed": true}`))
	if err != nil {
		t.Fatalf("ParseVerdict() error = %v", err)
	}
	if !verdict.Passed {
		t.Errorf("verdict.Passed = false, want true")
	}
	if verdict.Results == nil {
		t.Fatal("verdict.Results is nil, want empty slice")
	}
	if len(verdict.Results) != 0 {
		t.Errorf("len(verdict.Results) = %d, want 0", len(verdict.Results))
	}
}

func TestParseVerdict_emptyInput_returnsError(t *testing.T) {
	_, err := ParseVerdict([]byte{})
	if err == nil {
		t.Error("ParseVerdict() expected error for empty input, got nil")
	}
}
