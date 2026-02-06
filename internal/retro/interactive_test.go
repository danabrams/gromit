package retro

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestReviewProposals_AllAccepted(t *testing.T) {
	proposals := &Proposals{
		Consolidations: []ConsolidationProposal{
			{
				LearningHashes:   []string{"abc123", "def456"},
				ConsolidatedText: "Always verify state before acting",
				Rationale:        "Both address the same principle",
			},
		},
		Promotions: []PromotionProposal{
			{
				LearningHash: "xyz789",
				ProposedRule: "Check actual state before diagnosing",
				Section:      "Process",
				Rationale:    "Seen across multiple beads",
			},
		},
		Archives: []ArchiveProposal{
			{
				LearningHash: "old123",
				Rationale:    "Already captured in rules",
			},
		},
		RuleChanges: []RuleChangeProposal{
			{
				CurrentRule:  "Old rule text",
				ProposedRule: "New rule text",
				Rationale:    "More specific guidance needed",
			},
		},
	}

	// Simulate user accepting everything
	input := "y\ny\ny\ny\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	accepted, err := ReviewProposals(proposals, reader, &output)
	if err != nil {
		t.Fatalf("ReviewProposals() error = %v", err)
	}

	// Check all were accepted
	if len(accepted.Consolidations) != 1 {
		t.Errorf("Expected 1 accepted consolidation, got %d", len(accepted.Consolidations))
	}
	if len(accepted.Promotions) != 1 {
		t.Errorf("Expected 1 accepted promotion, got %d", len(accepted.Promotions))
	}
	if len(accepted.Archives) != 1 {
		t.Errorf("Expected 1 accepted archive, got %d", len(accepted.Archives))
	}
	if len(accepted.RuleChanges) != 1 {
		t.Errorf("Expected 1 accepted rule change, got %d", len(accepted.RuleChanges))
	}

	// Verify output contains section headers
	outputStr := output.String()
	if !strings.Contains(outputStr, "=== CONSOLIDATIONS") {
		t.Error("Output should contain CONSOLIDATIONS header")
	}
	if !strings.Contains(outputStr, "=== PROMOTIONS") {
		t.Error("Output should contain PROMOTIONS header")
	}
	if !strings.Contains(outputStr, "=== ARCHIVES") {
		t.Error("Output should contain ARCHIVES header")
	}
	if !strings.Contains(outputStr, "=== RULE CHANGES") {
		t.Error("Output should contain RULE CHANGES header")
	}
}

func TestReviewProposals_AllRejected(t *testing.T) {
	proposals := &Proposals{
		Consolidations: []ConsolidationProposal{
			{
				LearningHashes:   []string{"abc123", "def456"},
				ConsolidatedText: "Test consolidation",
				Rationale:        "Test",
			},
		},
		Promotions: []PromotionProposal{
			{
				LearningHash: "xyz789",
				ProposedRule: "Test rule",
				Section:      "Process",
				Rationale:    "Test",
			},
		},
	}

	// Simulate user rejecting everything
	input := "n\nn\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	accepted, err := ReviewProposals(proposals, reader, &output)
	if err != nil {
		t.Fatalf("ReviewProposals() error = %v", err)
	}

	// Check none were accepted
	if len(accepted.Consolidations) != 0 {
		t.Errorf("Expected 0 accepted consolidations, got %d", len(accepted.Consolidations))
	}
	if len(accepted.Promotions) != 0 {
		t.Errorf("Expected 0 accepted promotions, got %d", len(accepted.Promotions))
	}
}

func TestReviewProposals_MixedResponses(t *testing.T) {
	proposals := &Proposals{
		Consolidations: []ConsolidationProposal{
			{
				LearningHashes:   []string{"a", "b"},
				ConsolidatedText: "First",
				Rationale:        "Test 1",
			},
			{
				LearningHashes:   []string{"c", "d"},
				ConsolidatedText: "Second",
				Rationale:        "Test 2",
			},
		},
		Promotions: []PromotionProposal{
			{
				LearningHash: "e",
				ProposedRule: "Rule 1",
				Section:      "Process",
				Rationale:    "Test",
			},
		},
	}

	// Accept first consolidation, reject second, accept promotion
	input := "y\nn\ny\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	accepted, err := ReviewProposals(proposals, reader, &output)
	if err != nil {
		t.Fatalf("ReviewProposals() error = %v", err)
	}

	// Check selective acceptance
	if len(accepted.Consolidations) != 1 {
		t.Errorf("Expected 1 accepted consolidation, got %d", len(accepted.Consolidations))
	}
	if len(accepted.Consolidations) > 0 && accepted.Consolidations[0].ConsolidatedText != "First" {
		t.Errorf("Expected 'First' consolidation, got %s", accepted.Consolidations[0].ConsolidatedText)
	}
	if len(accepted.Promotions) != 1 {
		t.Errorf("Expected 1 accepted promotion, got %d", len(accepted.Promotions))
	}
}

func TestReviewProposals_EmptyProposals(t *testing.T) {
	proposals := &Proposals{}

	reader := strings.NewReader("")
	var output bytes.Buffer

	accepted, err := ReviewProposals(proposals, reader, &output)
	if err != nil {
		t.Fatalf("ReviewProposals() error = %v", err)
	}

	// Should return empty accepted lists (not nil)
	if accepted.Consolidations == nil {
		t.Error("Expected non-nil Consolidations for empty proposals")
	}
	if accepted.Promotions == nil {
		t.Error("Expected non-nil Promotions for empty proposals")
	}
	if accepted.Archives == nil {
		t.Error("Expected non-nil Archives for empty proposals")
	}
	if accepted.RuleChanges == nil {
		t.Error("Expected non-nil RuleChanges for empty proposals")
	}
	if len(accepted.Consolidations) != 0 {
		t.Error("Expected no consolidations for empty proposals")
	}
	if len(accepted.Promotions) != 0 {
		t.Error("Expected no promotions for empty proposals")
	}
	if len(accepted.Archives) != 0 {
		t.Error("Expected no archives for empty proposals")
	}
	if len(accepted.RuleChanges) != 0 {
		t.Error("Expected no rule changes for empty proposals")
	}

	// Output should be minimal
	outputStr := output.String()
	if strings.Contains(outputStr, "===") {
		t.Error("Empty proposals should not show section headers")
	}
}

func TestReviewProposals_NilProposals(t *testing.T) {
	reader := strings.NewReader("")
	var output bytes.Buffer

	_, err := ReviewProposals(nil, reader, &output)
	if err == nil {
		t.Error("Expected error for nil proposals, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("Expected 'nil' in error message, got: %v", err)
	}
}

func TestReviewProposals_YesVariations(t *testing.T) {
	proposals := &Proposals{
		Promotions: []PromotionProposal{
			{
				LearningHash: "test1",
				ProposedRule: "Rule 1",
				Section:      "Process",
				Rationale:    "Test",
			},
			{
				LearningHash: "test2",
				ProposedRule: "Rule 2",
				Section:      "Process",
				Rationale:    "Test",
			},
		},
	}

	// Test different variations of yes
	input := "yes\nY\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	accepted, err := ReviewProposals(proposals, reader, &output)
	if err != nil {
		t.Fatalf("ReviewProposals() error = %v", err)
	}

	// Both should be accepted
	if len(accepted.Promotions) != 2 {
		t.Errorf("Expected 2 accepted promotions (yes/Y), got %d", len(accepted.Promotions))
	}
}

func TestReviewProposals_InvalidAnswer(t *testing.T) {
	proposals := &Proposals{
		Archives: []ArchiveProposal{
			{
				LearningHash: "test",
				Rationale:    "Test",
			},
		},
	}

	// Invalid answer should be treated as "no"
	input := "maybe\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	accepted, err := ReviewProposals(proposals, reader, &output)
	if err != nil {
		t.Fatalf("ReviewProposals() error = %v", err)
	}

	// Invalid answer should be treated as rejection
	if len(accepted.Archives) != 0 {
		t.Errorf("Expected 0 accepted archives for invalid answer, got %d", len(accepted.Archives))
	}
}

func TestReviewProposals_DisplaysDetails(t *testing.T) {
	proposals := &Proposals{
		Consolidations: []ConsolidationProposal{
			{
				LearningHashes:   []string{"hash1", "hash2", "hash3"},
				ConsolidatedText: "Unique consolidated text for testing",
				Rationale:        "Unique rationale for testing",
			},
		},
	}

	input := "n\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	_, err := ReviewProposals(proposals, reader, &output)
	if err != nil {
		t.Fatalf("ReviewProposals() error = %v", err)
	}

	outputStr := output.String()

	// Verify all details are displayed
	if !strings.Contains(outputStr, "hash1, hash2, hash3") {
		t.Error("Output should contain learning hashes")
	}
	if !strings.Contains(outputStr, "Unique consolidated text for testing") {
		t.Error("Output should contain consolidated text")
	}
	if !strings.Contains(outputStr, "Unique rationale for testing") {
		t.Error("Output should contain rationale")
	}
}

func TestAcceptedProposalsNormalizeNilFields(t *testing.T) {
	ap := &AcceptedProposals{}
	if ap.Consolidations != nil || ap.Promotions != nil || ap.Archives != nil || ap.RuleChanges != nil {
		t.Error("expected all fields to start as nil")
	}

	ap.normalizeNilFields()
	if ap.Consolidations == nil {
		t.Error("expected Consolidations to be non-nil after normalization")
	}
	if ap.Promotions == nil {
		t.Error("expected Promotions to be non-nil after normalization")
	}
	if ap.Archives == nil {
		t.Error("expected Archives to be non-nil after normalization")
	}
	if ap.RuleChanges == nil {
		t.Error("expected RuleChanges to be non-nil after normalization")
	}
}

func TestAcceptedProposalsNormalizeNilFieldsNilReceiver(t *testing.T) {
	var ap *AcceptedProposals
	ap.normalizeNilFields() // Should not panic
}

func TestAcceptedProposalsNormalizeNilFieldsPreservesExisting(t *testing.T) {
	ap := &AcceptedProposals{
		Consolidations: []ConsolidationProposal{{ConsolidatedText: "test"}},
		Promotions:     []PromotionProposal{{ProposedRule: "rule"}},
	}

	ap.normalizeNilFields()
	if len(ap.Consolidations) != 1 {
		t.Errorf("expected 1 consolidation preserved, got %d", len(ap.Consolidations))
	}
	if len(ap.Promotions) != 1 {
		t.Errorf("expected 1 promotion preserved, got %d", len(ap.Promotions))
	}
	if ap.Archives == nil {
		t.Error("expected Archives to be non-nil after normalization")
	}
	if ap.RuleChanges == nil {
		t.Error("expected RuleChanges to be non-nil after normalization")
	}
}

func TestAskYesNo(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"lowercase y", "y\n", true},
		{"uppercase Y", "Y\n", true},
		{"lowercase yes", "yes\n", true},
		{"uppercase YES", "YES\n", true},
		{"mixed Yes", "Yes\n", true},
		{"lowercase n", "n\n", false},
		{"uppercase N", "N\n", false},
		{"lowercase no", "no\n", false},
		{"anything else", "maybe\n", false},
		{"empty", "\n", false},
		{"whitespace yes", "  yes  \n", true},
		{"whitespace no", "  n  \n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			bufReader := bufio.NewReader(reader)
			var output bytes.Buffer

			result, err := askYesNo(bufReader, &output, "Test prompt")
			if err != nil {
				t.Fatalf("askYesNo() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("askYesNo(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
