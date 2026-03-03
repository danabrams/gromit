package midreview_test

import (
	"encoding/json"
	"testing"

	"github.com/danabrams/gromit/internal/runner/pipeline/midreview"
)

func TestFinding_JSONMarshal(t *testing.T) {
	t.Parallel()

	f := midreview.Finding{
		Category: "performance",
		Message:  "Consider optimizing loop",
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}

	var decoded midreview.Finding
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, want nil", err)
	}

	if decoded.Category != f.Category {
		t.Fatalf("Category = %q, want %q", decoded.Category, f.Category)
	}
	if decoded.Message != f.Message {
		t.Fatalf("Message = %q, want %q", decoded.Message, f.Message)
	}
}

func TestMidBuildReviewResult_JSONMarshal(t *testing.T) {
	t.Parallel()

	result := midreview.MidBuildReviewResult{
		Findings: []midreview.Finding{
			{
				Category: "performance",
				Message:  "Consider optimizing loop",
			},
			{
				Category: "testing",
				Message:  "Add test coverage",
			},
		},
		Summary: "Found 2 issues",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}

	var decoded midreview.MidBuildReviewResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, want nil", err)
	}

	if len(decoded.Findings) != 2 {
		t.Fatalf("Findings length = %d, want 2", len(decoded.Findings))
	}
	if decoded.Findings[0].Category != "performance" {
		t.Fatalf("Findings[0].Category = %q, want %q", decoded.Findings[0].Category, "performance")
	}
	if decoded.Summary != "Found 2 issues" {
		t.Fatalf("Summary = %q, want %q", decoded.Summary, "Found 2 issues")
	}
}

func TestParseMidBuildReviewResult_ValidFullResult(t *testing.T) {
	t.Parallel()

	input := `{
		"findings": [
			{"category": "performance", "message": "Consider optimizing loop"},
			{"category": "testing", "message": "Add test coverage"}
		],
		"summary": "Found 2 issues"
	}`

	result, err := midreview.ParseMidBuildReviewResult(input)
	if err != nil {
		t.Fatalf("ParseMidBuildReviewResult() error = %v, want nil", err)
	}

	if len(result.Findings) != 2 {
		t.Fatalf("Findings length = %d, want 2", len(result.Findings))
	}
	if result.Summary != "Found 2 issues" {
		t.Fatalf("Summary = %q, want %q", result.Summary, "Found 2 issues")
	}
}

func TestParseMidBuildReviewResult_MalformedJSON(t *testing.T) {
	t.Parallel()

	input := `{invalid json}`

	_, err := midreview.ParseMidBuildReviewResult(input)
	if err == nil {
		t.Fatalf("ParseMidBuildReviewResult() error = nil, want non-nil")
	}
}

func TestParseMidBuildReviewResult_EmptyString(t *testing.T) {
	t.Parallel()

	input := ""

	_, err := midreview.ParseMidBuildReviewResult(input)
	if err == nil {
		t.Fatalf("ParseMidBuildReviewResult() error = nil, want non-nil")
	}
}
