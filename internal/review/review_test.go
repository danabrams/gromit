package review

import (
	"testing"
)

func TestParseReviewResult(t *testing.T) {
	input := `{
		"passed": true,
		"fixes_applied": ["added missing error check"],
		"beads_to_create": [
			{"title": "Add input validation", "description": "...", "priority": 1, "labels": ["from-review"]}
		],
		"backlog_items": [
			{"title": "Redesign auth flow", "description": "...", "reason": "needs product owner"}
		],
		"learnings": ["error handling pattern is consistent"],
		"summary": "Implementation matches spec"
	}`

	result, err := ParseReviewResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Error("expected passed=true")
	}
	if len(result.FixesApplied) != 1 {
		t.Errorf("expected 1 fix, got %d", len(result.FixesApplied))
	}
	if len(result.BeadsToCreate) != 1 {
		t.Errorf("expected 1 bead, got %d", len(result.BeadsToCreate))
	}
	if result.BeadsToCreate[0].Title != "Add input validation" {
		t.Errorf("unexpected bead title: %q", result.BeadsToCreate[0].Title)
	}
	if len(result.BacklogItems) != 1 {
		t.Errorf("expected 1 backlog item, got %d", len(result.BacklogItems))
	}
	if len(result.Learnings) != 1 {
		t.Errorf("expected 1 learning, got %d", len(result.Learnings))
	}
	if result.Summary != "Implementation matches spec" {
		t.Errorf("unexpected summary: %q", result.Summary)
	}
}

func TestParseReviewResultWithSurroundingText(t *testing.T) {
	input := `Here is my review:

{
	"passed": false,
	"fixes_applied": [],
	"beads_to_create": [],
	"backlog_items": [],
	"summary": "Major issues found"
}

That concludes my review.`

	result, err := ParseReviewResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed {
		t.Error("expected passed=false")
	}
	if result.Summary != "Major issues found" {
		t.Errorf("unexpected summary: %q", result.Summary)
	}
}

func TestParseReviewResultNilSlices(t *testing.T) {
	input := `{"passed": true, "summary": "looks good"}`

	result, err := ParseReviewResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.FixesApplied == nil {
		t.Error("FixesApplied should be normalized to empty slice, not nil")
	}
	if result.BeadsToCreate == nil {
		t.Error("BeadsToCreate should be normalized to empty slice, not nil")
	}
	if result.BacklogItems == nil {
		t.Error("BacklogItems should be normalized to empty slice, not nil")
	}
}

func TestParseReviewResultEmptyInput(t *testing.T) {
	_, err := ParseReviewResult("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseReviewResultNoJSON(t *testing.T) {
	input := "This is just plain text with no JSON"
	_, err := ParseReviewResult(input)
	if err == nil {
		t.Error("expected error for input with no JSON")
	}
}

func TestParseReviewResultInvalidJSON(t *testing.T) {
	input := `{"passed": true, "summary": "incomplete`
	_, err := ParseReviewResult(input)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseReviewResultNormalizeLabels(t *testing.T) {
	input := `{
		"passed": true,
		"fixes_applied": [],
		"beads_to_create": [
			{"title": "Test", "description": "test", "priority": 1}
		],
		"backlog_items": [],
		"summary": "ok"
	}`

	result, err := ParseReviewResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.BeadsToCreate) != 1 {
		t.Fatal("expected 1 bead proposal")
	}
	if result.BeadsToCreate[0].Labels == nil {
		t.Error("BeadProposal.Labels should be normalized to empty slice, not nil")
	}
}

func TestParseReviewResultNormalizeLearnings(t *testing.T) {
	input := `{
		"passed": true,
		"fixes_applied": [],
		"beads_to_create": [],
		"backlog_items": [],
		"summary": "ok"
	}`

	result, err := ParseReviewResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Learnings == nil {
		t.Error("Learnings should be normalized to empty slice, not nil")
	}
}
