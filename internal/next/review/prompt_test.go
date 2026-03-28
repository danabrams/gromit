package review

import (
	"strings"
	"testing"
)

func TestRenderReviewPrompt_ContainsFacetName(t *testing.T) {
	reg := NewRegistry()
	facet, _ := reg.Get("spec_alignment")

	input := ReviewPromptInput{
		FacetDef:    facet,
		DiffSummary: "Added refund handler in internal/handler/refund.go",
		SpecContent: "## Acceptance Criteria\n- refund endpoint returns 200",
	}

	prompt, err := RenderReviewPrompt(input)
	if err != nil {
		t.Fatalf("RenderReviewPrompt: %v", err)
	}
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	if !strings.Contains(prompt, "spec_alignment") {
		t.Error("prompt should contain facet name")
	}
	if !strings.Contains(prompt, "refund handler") {
		t.Error("prompt should contain diff summary")
	}
}

func TestRenderReviewPrompt_IncludesPriorFindings(t *testing.T) {
	reg := NewRegistry()
	facet, _ := reg.Get("code_quality")

	input := ReviewPromptInput{
		FacetDef:      facet,
		DiffSummary:   "Modified handler.go",
		PriorFindings: []Finding{{Severity: SeverityWarning, File: "handler.go", Description: "duplicate logic"}},
	}

	prompt, err := RenderReviewPrompt(input)
	if err != nil {
		t.Fatalf("RenderReviewPrompt: %v", err)
	}
	if !strings.Contains(prompt, "duplicate logic") {
		t.Error("prompt should include prior finding descriptions for disposition labeling")
	}
	if !strings.Contains(prompt, `disposition: "pre-existing"`) {
		t.Errorf("prompt should remind reviewers to tag pre-existing findings")
	}
	if !strings.Contains(prompt, `disposition: "new"`) {
		t.Errorf("prompt should remind reviewers to tag new findings")
	}
}

func TestRenderReviewPrompt_SkipsPriorTriageWhenNoPriorFindings(t *testing.T) {
	reg := NewRegistry()
	facet, _ := reg.Get("spec_alignment")

	input := ReviewPromptInput{
		FacetDef:    facet,
		DiffSummary: "Added refund handler in internal/handler/refund.go",
	}

	prompt, err := RenderReviewPrompt(input)
	if err != nil {
		t.Fatalf("RenderReviewPrompt: %v", err)
	}
	if strings.Contains(prompt, "## Prior Findings") {
		t.Error("prompt should not include prior findings block when there are no prior findings")
	}
	if strings.Contains(prompt, `disposition: "pre-existing"`) {
		t.Error("prompt should not mention disposition instructions when no prior findings exist")
	}
}
