package retro

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseProposals_ValidJSON(t *testing.T) {
	output := `
## Analysis

Here are my recommendations:

` + "```json" + `
{
  "consolidations": [
    {
      "learning_hashes": ["abc123", "def456"],
      "consolidated_text": "Always verify state before acting",
      "rationale": "Both address the same principle"
    }
  ],
  "promotions": [
    {
      "learning_hash": "abc123",
      "proposed_rule": "Verify actual state before diagnosing",
      "section": "Process",
      "rationale": "Seen across multiple beads"
    }
  ],
  "archives": [
    {
      "learning_hash": "xyz789",
      "rationale": "Already captured in rules"
    }
  ],
  "rule_changes": [
    {
      "current_rule": "Old rule text",
      "proposed_rule": "New rule text",
      "rationale": "More specific guidance needed"
    }
  ]
}
` + "```" + `

That's my analysis.
`

	proposals, err := ParseProposals(output)
	if err != nil {
		t.Fatalf("ParseProposals() error = %v", err)
	}

	// Check consolidations
	if len(proposals.Consolidations) != 1 {
		t.Errorf("Expected 1 consolidation, got %d", len(proposals.Consolidations))
	}
	if len(proposals.Consolidations) > 0 {
		c := proposals.Consolidations[0]
		if len(c.LearningHashes) != 2 {
			t.Errorf("Expected 2 learning hashes, got %d", len(c.LearningHashes))
		}
		if c.ConsolidatedText != "Always verify state before acting" {
			t.Errorf("Unexpected consolidated text: %s", c.ConsolidatedText)
		}
	}

	// Check promotions
	if len(proposals.Promotions) != 1 {
		t.Errorf("Expected 1 promotion, got %d", len(proposals.Promotions))
	}
	if len(proposals.Promotions) > 0 {
		p := proposals.Promotions[0]
		if p.LearningHash != "abc123" {
			t.Errorf("Expected hash abc123, got %s", p.LearningHash)
		}
		if p.Section != "Process" {
			t.Errorf("Expected section Process, got %s", p.Section)
		}
	}

	// Check archives
	if len(proposals.Archives) != 1 {
		t.Errorf("Expected 1 archive, got %d", len(proposals.Archives))
	}
	if len(proposals.Archives) > 0 {
		a := proposals.Archives[0]
		if a.LearningHash != "xyz789" {
			t.Errorf("Expected hash xyz789, got %s", a.LearningHash)
		}
	}

	// Check rule changes
	if len(proposals.RuleChanges) != 1 {
		t.Errorf("Expected 1 rule change, got %d", len(proposals.RuleChanges))
	}
	if len(proposals.RuleChanges) > 0 {
		rc := proposals.RuleChanges[0]
		if rc.CurrentRule != "Old rule text" {
			t.Errorf("Expected 'Old rule text', got %s", rc.CurrentRule)
		}
	}
}

func TestParseProposals_EmptyProposals(t *testing.T) {
	output := `
No changes needed at this time.

` + "```json" + `
{}
` + "```"

	proposals, err := ParseProposals(output)
	if err != nil {
		t.Fatalf("ParseProposals() error = %v", err)
	}

	if len(proposals.Consolidations) != 0 {
		t.Errorf("Expected 0 consolidations, got %d", len(proposals.Consolidations))
	}
	if len(proposals.Promotions) != 0 {
		t.Errorf("Expected 0 promotions, got %d", len(proposals.Promotions))
	}
	if len(proposals.Archives) != 0 {
		t.Errorf("Expected 0 archives, got %d", len(proposals.Archives))
	}
	if len(proposals.RuleChanges) != 0 {
		t.Errorf("Expected 0 rule changes, got %d", len(proposals.RuleChanges))
	}
}

func TestParseProposals_NoJSONBlock(t *testing.T) {
	output := `
This is just plain text with no JSON block.
`

	_, err := ParseProposals(output)
	if err == nil {
		t.Error("Expected error for missing JSON block, got nil")
	}
	// The error message now comes from jsonutil
	if err != nil && !strings.Contains(err.Error(), "parsing proposals") {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestParseProposals_InvalidJSON(t *testing.T) {
	output := `
` + "```json" + `
{
  "consolidations": [
    {
      "learning_hashes": "not an array"
    }
  ]
}
` + "```"

	_, err := ParseProposals(output)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestParseProposals_JSONWithoutMarker(t *testing.T) {
	// Test with plain ``` instead of ```json
	output := `
` + "```" + `
{
  "promotions": [
    {
      "learning_hash": "test123",
      "proposed_rule": "Test rule",
      "section": "Safety",
      "rationale": "Test rationale"
    }
  ]
}
` + "```"

	proposals, err := ParseProposals(output)
	if err != nil {
		t.Fatalf("ParseProposals() error = %v", err)
	}

	if len(proposals.Promotions) != 1 {
		t.Errorf("Expected 1 promotion, got %d", len(proposals.Promotions))
	}
}

func TestExtractJSONBlock_MultipleBlocks(t *testing.T) {
	// Should extract the first block
	output := `
Here's some analysis:

` + "```json" + `
{"promotions": []}
` + "```" + `

And here's more:

` + "```json" + `
{"archives": []}
` + "```"

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err == nil {
		// Direct parsing worked (shouldn't in this case)
		t.Error("Expected direct parsing to fail for multiple blocks")
		return
	}

	// This test is now covered by jsonutil tests
	// The proposed behavior is to extract first block via jsonutil.ExtractCodeBlock
	var proposals Proposals
	if err := json.Unmarshal([]byte(output), &proposals); err == nil {
		t.Errorf("Expected parsing to fail for multiple blocks")
	}
}

func TestExtractJSONBlock_WithWhitespace(t *testing.T) {
	output := `
` + "```json" + `

  {
    "consolidations": []
  }

` + "```"

	// Test via jsonutil.ExtractCodeBlock (now the standard mechanism)
	// This is now tested in jsonutil_test.go - here we just verify ParseProposals still works
	proposals, err := ParseProposals(output)
	if err != nil {
		t.Fatalf("Failed to parse proposals: %v", err)
	}
	if proposals.Consolidations == nil {
		t.Error("Expected non-nil consolidations after parsing")
	}
}

func TestProposalsJSONRoundtrip(t *testing.T) {
	// Test that we can marshal and unmarshal proposals correctly
	original := Proposals{
		Consolidations: []ConsolidationProposal{
			{
				LearningHashes:   []string{"hash1", "hash2"},
				ConsolidatedText: "Combined learning",
				Rationale:        "They're similar",
			},
		},
		Promotions: []PromotionProposal{
			{
				LearningHash: "hash3",
				ProposedRule: "New rule",
				Section:      "Architecture",
				Rationale:    "Seen multiple times",
			},
		},
		Archives: []ArchiveProposal{
			{
				LearningHash: "hash4",
				Rationale:    "Obsolete",
			},
		},
		RuleChanges: []RuleChangeProposal{
			{
				CurrentRule:  "Old text",
				ProposedRule: "New text",
				Rationale:    "Better clarity",
			},
		},
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded Proposals
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Compare
	if len(decoded.Consolidations) != len(original.Consolidations) {
		t.Errorf("Consolidation count mismatch: %d vs %d",
			len(decoded.Consolidations), len(original.Consolidations))
	}
	if len(decoded.Promotions) != len(original.Promotions) {
		t.Errorf("Promotion count mismatch: %d vs %d",
			len(decoded.Promotions), len(original.Promotions))
	}
	if len(decoded.Archives) != len(original.Archives) {
		t.Errorf("Archive count mismatch: %d vs %d",
			len(decoded.Archives), len(original.Archives))
	}
	if len(decoded.RuleChanges) != len(original.RuleChanges) {
		t.Errorf("RuleChange count mismatch: %d vs %d",
			len(decoded.RuleChanges), len(original.RuleChanges))
	}

	// Check a few field values
	if len(decoded.Consolidations) > 0 {
		if decoded.Consolidations[0].ConsolidatedText != "Combined learning" {
			t.Errorf("Consolidated text mismatch: %s", decoded.Consolidations[0].ConsolidatedText)
		}
	}
	if len(decoded.Promotions) > 0 {
		if decoded.Promotions[0].Section != "Architecture" {
			t.Errorf("Section mismatch: %s", decoded.Promotions[0].Section)
		}
	}
}

func TestProposalsNormalizeNilFields(t *testing.T) {
	p := &Proposals{}
	p.normalizeNilFields()

	if p.Consolidations == nil {
		t.Error("Expected Consolidations to be non-nil after normalization")
	}
	if p.Promotions == nil {
		t.Error("Expected Promotions to be non-nil after normalization")
	}
	if p.Archives == nil {
		t.Error("Expected Archives to be non-nil after normalization")
	}
	if p.RuleChanges == nil {
		t.Error("Expected RuleChanges to be non-nil after normalization")
	}

	// Verify already non-nil slices are preserved
	p2 := &Proposals{
		Consolidations: []ConsolidationProposal{{LearningHashes: []string{"a"}, ConsolidatedText: "t", Rationale: "r"}},
		Promotions:     []PromotionProposal{{LearningHash: "a"}},
		Archives:       []ArchiveProposal{{LearningHash: "a"}},
		RuleChanges:    []RuleChangeProposal{{CurrentRule: "r"}},
	}
	p2.normalizeNilFields()

	if len(p2.Consolidations) != 1 {
		t.Errorf("Expected 1 consolidation, got %d", len(p2.Consolidations))
	}
	if len(p2.Promotions) != 1 {
		t.Errorf("Expected 1 promotion, got %d", len(p2.Promotions))
	}
	if len(p2.Archives) != 1 {
		t.Errorf("Expected 1 archive, got %d", len(p2.Archives))
	}
	if len(p2.RuleChanges) != 1 {
		t.Errorf("Expected 1 rule change, got %d", len(p2.RuleChanges))
	}
}

func TestProposalsNormalizeNilFieldsOnNilProposals(t *testing.T) {
	var p *Proposals
	p.normalizeNilFields() // should not panic
}

func TestConsolidationProposalNormalizeNilFields(t *testing.T) {
	c := &ConsolidationProposal{}
	c.normalizeNilFields()

	if c.LearningHashes == nil {
		t.Error("Expected LearningHashes to be non-nil after normalization")
	}

	// Verify already non-nil slices are preserved
	c2 := &ConsolidationProposal{LearningHashes: []string{"hash1"}}
	c2.normalizeNilFields()

	if len(c2.LearningHashes) != 1 {
		t.Errorf("Expected 1 learning hash, got %d", len(c2.LearningHashes))
	}
}

func TestConsolidationProposalNormalizeNilFieldsOnNilReceiver(t *testing.T) {
	var c *ConsolidationProposal
	c.normalizeNilFields() // should not panic
}

func TestParseProposalsNormalizesNilFields(t *testing.T) {
	// JSON with missing fields — they should be normalized to empty slices
	output := "```json\n{}\n```"

	proposals, err := ParseProposals(output)
	if err != nil {
		t.Fatalf("ParseProposals() error = %v", err)
	}

	if proposals.Consolidations == nil {
		t.Error("Expected Consolidations to be non-nil after parsing empty JSON")
	}
	if proposals.Promotions == nil {
		t.Error("Expected Promotions to be non-nil after parsing empty JSON")
	}
	if proposals.Archives == nil {
		t.Error("Expected Archives to be non-nil after parsing empty JSON")
	}
	if proposals.RuleChanges == nil {
		t.Error("Expected RuleChanges to be non-nil after parsing empty JSON")
	}
}

func TestParseProposalsNormalizesExplicitNull(t *testing.T) {
	output := "```json\n{\"consolidations\": null, \"promotions\": null, \"archives\": null, \"rule_changes\": null}\n```"

	proposals, err := ParseProposals(output)
	if err != nil {
		t.Fatalf("ParseProposals() error = %v", err)
	}

	if proposals.Consolidations == nil {
		t.Error("Expected Consolidations to be non-nil after parsing explicit null")
	}
	if proposals.Promotions == nil {
		t.Error("Expected Promotions to be non-nil after parsing explicit null")
	}
	if proposals.Archives == nil {
		t.Error("Expected Archives to be non-nil after parsing explicit null")
	}
	if proposals.RuleChanges == nil {
		t.Error("Expected RuleChanges to be non-nil after parsing explicit null")
	}
}

func TestParseProposalsNormalizesNestedConsolidationHashes(t *testing.T) {
	// ConsolidationProposal with missing learning_hashes should be normalized
	output := "```json\n{\"consolidations\": [{\"consolidated_text\": \"test\", \"rationale\": \"test\"}]}\n```"

	proposals, err := ParseProposals(output)
	if err != nil {
		t.Fatalf("ParseProposals() error = %v", err)
	}

	if len(proposals.Consolidations) != 1 {
		t.Fatalf("Expected 1 consolidation, got %d", len(proposals.Consolidations))
	}
	if proposals.Consolidations[0].LearningHashes == nil {
		t.Error("Expected LearningHashes to be non-nil after parsing")
	}
}
