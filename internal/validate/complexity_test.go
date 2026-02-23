package validate

import "testing"

func TestScoreCandidate_TitleScopeSignalAddsTitleReasonAndHighClassification(t *testing.T) {
	result := ScoreCandidate(BeadCandidate{
		Title:          "Refactor entire auth workflow",
		Description:    "Focused implementation",
		EstimatedFiles: 1,
	})

	if result.Classification != "high" {
		t.Fatalf("Classification = %q, want %q", result.Classification, "high")
	}
	if len(result.Reasons) != 1 {
		t.Fatalf("Reasons len = %d, want 1", len(result.Reasons))
	}
	want := "contains broad-scope language in title"
	if result.Reasons[0] != want {
		t.Fatalf("Reasons[0] = %q, want %q", result.Reasons[0], want)
	}
}
