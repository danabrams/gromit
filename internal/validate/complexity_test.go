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

func TestScoreCandidate_TitleAndDescriptionScopeSignalsIncludeBothReasons(t *testing.T) {
	result := ScoreCandidate(BeadCandidate{
		Title:          "Update all auth handlers",
		Description:    "This work will refactor entire validation layer",
		EstimatedFiles: 2,
	})

	if result.Classification != "high" {
		t.Fatalf("Classification = %q, want %q", result.Classification, "high")
	}
	if len(result.Reasons) != 2 {
		t.Fatalf("Reasons len = %d, want 2", len(result.Reasons))
	}
	if result.Reasons[0] != "contains broad-scope language in title" {
		t.Fatalf("Reasons[0] = %q, want %q", result.Reasons[0], "contains broad-scope language in title")
	}
	if result.Reasons[1] != "contains broad-scope language in description" {
		t.Fatalf("Reasons[1] = %q, want %q", result.Reasons[1], "contains broad-scope language in description")
	}
}
