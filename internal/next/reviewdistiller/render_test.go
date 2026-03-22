package reviewdistiller

import (
	"strings"
	"testing"
	"time"
)

// TestRenderMarkdownNilResult verifies RenderMarkdown handles nil input gracefully.
func TestRenderMarkdownNilResult(t *testing.T) {
	output := RenderMarkdown(nil)

	if output != "" {
		t.Errorf("RenderMarkdown(nil) = %q, want empty string", output)
	}
}

// TestRenderMarkdownBasicWithSingleProposal verifies RenderMarkdown includes run ID, outcome, and model tier.
func TestRenderMarkdownBasicWithSingleProposal(t *testing.T) {
	result := &DistillationResult{
		RunID:     "run-123",
		SpecID:    "spec-001",
		Outcome:   "accepted",
		ModelTier: TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 14, 30, 0, 0, time.UTC),
		Proposals: []Proposal{
			{
				ID:         "p1",
				Type:       "doctrine_rule",
				Title:      "Add validation check",
				Confidence: "high",
			},
		},
	}

	output := RenderMarkdown(result)

	// Verify essential metadata is present
	if !strings.Contains(output, "run-123") {
		t.Error("output missing run ID")
	}
	if !strings.Contains(output, "accepted") {
		t.Error("output missing outcome")
	}
	if !strings.Contains(output, "high") {
		t.Error("output missing model tier")
	}

	// Verify proposal content
	if !strings.Contains(output, "Add validation check") {
		t.Error("output missing proposal title")
	}
	if !strings.Contains(output, "doctrine_rule") {
		t.Error("output missing proposal type")
	}
	if !strings.Contains(output, "HIGH") {
		t.Error("output missing confidence (should be uppercase)")
	}
}

// TestRenderMarkdownMultipleProposals verifies RenderMarkdown handles multiple proposals correctly.
func TestRenderMarkdownMultipleProposals(t *testing.T) {
	result := &DistillationResult{
		RunID:     "run-456",
		SpecID:    "spec-002",
		Outcome:   "rejected",
		ModelTier: TierMedium,
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		Proposals: []Proposal{
			{
				ID:         "p1",
				Type:       "validation_gap",
				Title:      "Missing error handling",
				Confidence: "high",
			},
			{
				ID:         "p2",
				Type:       "planner_heuristic",
				Title:      "Improve retry logic",
				Confidence: "medium",
			},
			{
				ID:         "p3",
				Type:       "refinement_guidance",
				Title:      "Add documentation",
				Confidence: "low",
			},
		},
	}

	output := RenderMarkdown(result)

	// Verify proposal count
	if !strings.Contains(output, "Proposals (3)") {
		t.Error("output missing or incorrect proposal count")
	}

	// Verify all proposal titles present
	if !strings.Contains(output, "Missing error handling") {
		t.Error("output missing first proposal title")
	}
	if !strings.Contains(output, "Improve retry logic") {
		t.Error("output missing second proposal title")
	}
	if !strings.Contains(output, "Add documentation") {
		t.Error("output missing third proposal title")
	}

	// Verify proposal types are present
	if !strings.Contains(output, "validation_gap") {
		t.Error("output missing validation_gap type")
	}
	if !strings.Contains(output, "planner_heuristic") {
		t.Error("output missing planner_heuristic type")
	}
	if !strings.Contains(output, "refinement_guidance") {
		t.Error("output missing refinement_guidance type")
	}

	// Verify confidence levels are present (uppercased)
	if !strings.Contains(output, "HIGH") {
		t.Error("output missing HIGH confidence")
	}
	if !strings.Contains(output, "MEDIUM") {
		t.Error("output missing MEDIUM confidence")
	}
	if !strings.Contains(output, "LOW") {
		t.Error("output missing LOW confidence")
	}
}

// TestRenderMarkdownNoProposals verifies RenderMarkdown handles empty proposal list.
func TestRenderMarkdownNoProposals(t *testing.T) {
	result := &DistillationResult{
		RunID:     "run-789",
		SpecID:    "spec-003",
		Outcome:   "undecided",
		ModelTier: TierLow,
		CreatedAt: time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC),
		Proposals: []Proposal{},
	}

	output := RenderMarkdown(result)

	if !strings.Contains(output, "No proposals extracted") {
		t.Error("output missing message for no proposals")
	}
	if !strings.Contains(output, "run-789") {
		t.Error("output missing run ID with no proposals")
	}
}

// TestRenderMarkdownWithAllProposalFields verifies all proposal narrative fields and evidence references.
func TestRenderMarkdownWithAllProposalFields(t *testing.T) {
	result := &DistillationResult{
		RunID:     "run-full",
		SpecID:    "spec-004",
		Outcome:   "accepted",
		ModelTier: TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 15, 45, 0, 0, time.UTC),
		Proposals: []Proposal{
			{
				ID:                  "p1",
				Type:                "doctrine_rule",
				Title:               "Enforce input validation",
				Confidence:          "high",
				ConfidenceRationale: "Validation is critical for security",
				WhatHappened:        "Input was not validated before use",
				WhatWasMissing:      "Type checking and bounds validation",
				ProposedChange:      "Add validation middleware",
				Rationale:           "Prevents injection attacks",
				EvidenceReferences:  []string{"OWASP-A03", "CWE-20"},
			},
		},
	}

	output := RenderMarkdown(result)

	// Verify narrative fields
	if !strings.Contains(output, "What Happened:") {
		t.Error("output missing 'What Happened' section")
	}
	if !strings.Contains(output, "Input was not validated before use") {
		t.Error("output missing WhatHappened content")
	}

	if !strings.Contains(output, "What Was Missing:") {
		t.Error("output missing 'What Was Missing' section")
	}
	if !strings.Contains(output, "Type checking and bounds validation") {
		t.Error("output missing WhatWasMissing content")
	}

	if !strings.Contains(output, "Proposed Change:") {
		t.Error("output missing 'Proposed Change' section")
	}
	if !strings.Contains(output, "Add validation middleware") {
		t.Error("output missing ProposedChange content")
	}

	if !strings.Contains(output, "Rationale:") {
		t.Error("output missing Rationale section")
	}
	if !strings.Contains(output, "Prevents injection attacks") {
		t.Error("output missing Rationale content")
	}

	// Verify confidence rationale
	if !strings.Contains(output, "Confidence Rationale:") {
		t.Error("output missing confidence rationale section")
	}
	if !strings.Contains(output, "Validation is critical for security") {
		t.Error("output missing confidence rationale content")
	}

	// Verify evidence references
	if !strings.Contains(output, "Evidence References:") {
		t.Error("output missing Evidence References section")
	}
	if !strings.Contains(output, "OWASP-A03") {
		t.Error("output missing first evidence reference")
	}
	if !strings.Contains(output, "CWE-20") {
		t.Error("output missing second evidence reference")
	}
}

// TestRenderMarkdownWithPartialProposalFields verifies rendering with optional fields omitted.
func TestRenderMarkdownWithPartialProposalFields(t *testing.T) {
	result := &DistillationResult{
		RunID:     "run-partial",
		SpecID:    "spec-005",
		Outcome:   "rejected",
		ModelTier: TierMedium,
		CreatedAt: time.Date(2026, 3, 21, 9, 15, 0, 0, time.UTC),
		Proposals: []Proposal{
			{
				ID:         "p1",
				Type:       "validation_gap",
				Title:      "Missing edge case handling",
				Confidence: "medium",
				// Only Title, Type, Confidence set; other fields empty
			},
		},
	}

	output := RenderMarkdown(result)

	// Verify required content is present
	if !strings.Contains(output, "Missing edge case handling") {
		t.Error("output missing proposal title")
	}
	if !strings.Contains(output, "validation_gap") {
		t.Error("output missing type")
	}
	if !strings.Contains(output, "MEDIUM") {
		t.Error("output missing confidence")
	}

	// Verify optional sections are not included when empty
	if strings.Contains(output, "What Happened:") {
		t.Error("output includes 'What Happened' even though it's empty")
	}
	if strings.Contains(output, "Evidence References:") {
		t.Error("output includes 'Evidence References' even though list is empty")
	}
}

// TestRenderMarkdownMetadataStructure verifies the metadata section format.
func TestRenderMarkdownMetadataStructure(t *testing.T) {
	result := &DistillationResult{
		RunID:     "meta-test-123",
		SpecID:    "spec-meta-001",
		Outcome:   "accepted",
		ModelTier: TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 14, 30, 45, 0, time.UTC),
		Proposals: []Proposal{},
	}

	output := RenderMarkdown(result)

	// Verify header structure
	if !strings.Contains(output, "# Review Distillation: accepted") {
		t.Error("output missing or incorrect header")
	}

	// Verify metadata section labels
	if !strings.Contains(output, "## Metadata") {
		t.Error("output missing Metadata section")
	}
	if !strings.Contains(output, "- **Run ID:**") {
		t.Error("output missing Run ID label")
	}
	if !strings.Contains(output, "- **Spec ID:**") {
		t.Error("output missing Spec ID label")
	}
	if !strings.Contains(output, "- **Outcome:**") {
		t.Error("output missing Outcome label")
	}
	if !strings.Contains(output, "- **Model Tier:**") {
		t.Error("output missing Model Tier label")
	}
	if !strings.Contains(output, "- **Created At:**") {
		t.Error("output missing Created At label")
	}

	// Verify metadata values are formatted correctly (backticks for IDs)
	if !strings.Contains(output, "`meta-test-123`") {
		t.Error("Run ID not properly backtick-formatted")
	}
	if !strings.Contains(output, "`spec-meta-001`") {
		t.Error("Spec ID not properly backtick-formatted")
	}
}

// TestFormatOutcome verifies snake_case to title case conversion.
func TestFormatOutcome(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"accepted", "Accepted"},
		{"rejected", "Rejected"},
		{"undecided", "Undecided"},
		{"needs_more_review", "Needs More Review"},
		{"partially_accepted", "Partially Accepted"},
		{"UPPERCASE", "Uppercase"},
		{"", ""},
	}

	for _, tt := range tests {
		result := formatOutcome(tt.input)
		if result != tt.expected {
			t.Errorf("formatOutcome(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestRenderMarkdownProposalNumbering verifies proposals are numbered sequentially.
func TestRenderMarkdownProposalNumbering(t *testing.T) {
	result := &DistillationResult{
		RunID:     "run-numbering",
		SpecID:    "spec-006",
		Outcome:   "accepted",
		ModelTier: TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 11, 0, 0, 0, time.UTC),
		Proposals: []Proposal{
			{ID: "p1", Type: "doctrine_rule", Title: "First proposal", Confidence: "high"},
			{ID: "p2", Type: "validation_gap", Title: "Second proposal", Confidence: "medium"},
			{ID: "p3", Type: "planner_heuristic", Title: "Third proposal", Confidence: "low"},
		},
	}

	output := RenderMarkdown(result)

	// Verify proposal numbering
	if !strings.Contains(output, "### 1. First proposal") {
		t.Error("output missing or incorrectly numbered first proposal")
	}
	if !strings.Contains(output, "### 2. Second proposal") {
		t.Error("output missing or incorrectly numbered second proposal")
	}
	if !strings.Contains(output, "### 3. Third proposal") {
		t.Error("output missing or incorrectly numbered third proposal")
	}
}

// TestRenderMarkdownTypeAndConfidenceBadge verifies the inline type/confidence format.
func TestRenderMarkdownTypeAndConfidenceBadge(t *testing.T) {
	result := &DistillationResult{
		RunID:     "run-badge",
		SpecID:    "spec-007",
		Outcome:   "accepted",
		ModelTier: TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 13, 0, 0, 0, time.UTC),
		Proposals: []Proposal{
			{
				ID:         "p1",
				Type:       "refinement_guidance",
				Title:      "Update docs",
				Confidence: "low",
			},
		},
	}

	output := RenderMarkdown(result)

	// Verify the type/confidence badge format: **Type:** `type` | **Confidence:** CONFIDENCE
	if !strings.Contains(output, "**Type:**") {
		t.Error("output missing Type badge label")
	}
	if !strings.Contains(output, "`refinement_guidance`") {
		t.Error("output missing backtick-formatted type value")
	}
	if !strings.Contains(output, "**Confidence:**") {
		t.Error("output missing Confidence badge label")
	}
	if !strings.Contains(output, "LOW") {
		t.Error("output missing uppercase confidence value")
	}
}
