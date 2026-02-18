package specgate

import (
	"testing"
)

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
