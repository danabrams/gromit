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

func TestFailedCriteria(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		verdict         *GateVerdict
		wantCriteria    []string
		wantNilReturned bool
	}{
		{
			name: "returns only failed criteria",
			verdict: &GateVerdict{
				Passed: false,
				Results: []CriterionResult{
					{Criterion: "No TODOs", Passed: true, Evidence: "none found"},
					{Criterion: "Tests pass", Passed: false, Evidence: "test output"},
					{Criterion: "No lint errors", Passed: false, Evidence: "lint output"},
				},
			},
			wantCriteria: []string{"Tests pass", "No lint errors"},
		},
		{
			name: "all pass returns empty",
			verdict: &GateVerdict{
				Passed: true,
				Results: []CriterionResult{
					{Criterion: "No TODOs", Passed: true, Evidence: "none found"},
				},
			},
			wantCriteria: []string{},
		},
		{
			name:            "nil verdict returns empty",
			verdict:         nil,
			wantCriteria:    []string{},
			wantNilReturned: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			failed := tt.verdict.FailedCriteria()
			if (failed == nil) != tt.wantNilReturned {
				t.Fatalf("FailedCriteria() nil = %t, want %t", failed == nil, tt.wantNilReturned)
			}
			if len(failed) != len(tt.wantCriteria) {
				t.Fatalf("len(FailedCriteria()) = %d, want %d", len(failed), len(tt.wantCriteria))
			}
			for i, criterion := range tt.wantCriteria {
				if failed[i].Criterion != criterion {
					t.Errorf("failed[%d].Criterion = %q, want %q", i, failed[i].Criterion, criterion)
				}
			}
		})
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

func TestParseVerdict_returnsErrorForMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "invalid JSON",
			input: []byte(`not json`),
		},
		{
			name:  "empty input",
			input: []byte{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseVerdict(tt.input)
			if err == nil {
				t.Errorf("ParseVerdict() expected error for %s, got nil", tt.name)
			}
		})
	}
}
