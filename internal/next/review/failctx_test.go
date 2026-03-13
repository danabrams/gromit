package review

import (
	"strings"
	"testing"
)

func TestBuildReviewFailureContext_BlockingOnly(t *testing.T) {
	result := RunResult{
		AllFindings: []Finding{
			{Facet: "spec_alignment", Severity: SeverityError, File: "handler.go", Description: "missing validation", SuggestedFix: "add check"},
			{Facet: "code_quality", Severity: SeverityInfo, File: "handler.go", Description: "consider extracting helper"},
		},
		BlockingFindings: []Finding{
			{Facet: "spec_alignment", Severity: SeverityError, File: "handler.go", Description: "missing validation", SuggestedFix: "add check"},
		},
	}

	strs := BuildFailureStrings(result)
	if len(strs) != 1 {
		t.Fatalf("expected 1 failure string (blocking only), got %d", len(strs))
	}
	if !containsAny(strs[0], "missing validation") {
		t.Errorf("failure string should contain description, got %q", strs[0])
	}
	if !containsAny(strs[0], "handler.go") {
		t.Errorf("failure string should contain file, got %q", strs[0])
	}
	if !strings.HasPrefix(strs[0], "review:") {
		t.Errorf("failure string should have review: prefix, got %q", strs[0])
	}
}

func TestBuildReviewFailureContext_Empty(t *testing.T) {
	result := RunResult{
		AllFindings:      []Finding{},
		BlockingFindings: []Finding{},
	}

	strs := BuildFailureStrings(result)
	if len(strs) != 0 {
		t.Errorf("expected 0 failure strings, got %d", len(strs))
	}
}

func TestBuildReviewFailureContext_IncludesSuggestedFix(t *testing.T) {
	result := RunResult{
		BlockingFindings: []Finding{
			{Facet: "spec_alignment", Severity: SeverityError, File: "handler.go", Description: "missing check", SuggestedFix: "add nil guard"},
		},
	}

	strs := BuildFailureStrings(result)
	if len(strs) != 1 {
		t.Fatalf("expected 1 string, got %d", len(strs))
	}
	if !containsAny(strs[0], "add nil guard") {
		t.Error("failure string should include suggested fix")
	}
}
