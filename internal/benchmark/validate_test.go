package benchmark

import (
	stdstrings "strings"
	"testing"
)

func TestValidateSelectedCohort_EnforcesMinimumSize(t *testing.T) {
	_, err := ValidateSelectedCohort(nil, []string{"gromit-1"}, 2)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want minimum size error")
	}
	if !stdstrings.Contains(err.Error(), "selected cohort size 1 is below minimum 2") {
		t.Fatalf("ValidateSelectedCohort() error = %q, want minimum size message", err.Error())
	}
}
