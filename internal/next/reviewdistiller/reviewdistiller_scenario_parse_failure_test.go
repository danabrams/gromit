package reviewdistiller

import (
	"strings"
	"testing"
)

func TestScenario_ParseFailureErrorIncludesExtractedJSON(t *testing.T) {
	// Seed
	input := `[{"evidence_references":"not-an-array"}]`

	// Invoke
	proposals, err := parseProposalsFromJSON(input)

	// Assert
	if err == nil {
		t.Fatal("parseProposalsFromJSON() expected error, got nil")
	}
	if proposals != nil {
		t.Fatalf("parseProposalsFromJSON() proposals = %v, want nil", proposals)
	}
	if !strings.Contains(err.Error(), input) {
		t.Fatalf("error message should include extracted input JSON; got: %q", err.Error())
	}
}
