package retro

import (
	"encoding/json"
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
	if err != nil && err.Error() != "no JSON code block found in output" {
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

	jsonStr := extractJSONBlock(output)
	if jsonStr == "" {
		t.Fatal("Expected to extract JSON block, got empty string")
	}

	// Should be the first block
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("Failed to parse extracted JSON: %v", err)
	}

	if _, ok := result["promotions"]; !ok {
		t.Error("Expected first JSON block with 'promotions' key")
	}
}

func TestExtractJSONBlock_WithWhitespace(t *testing.T) {
	output := `
` + "```json" + `

  {
    "consolidations": []
  }

` + "```"

	jsonStr := extractJSONBlock(output)
	if jsonStr == "" {
		t.Fatal("Expected to extract JSON block, got empty string")
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("Failed to parse extracted JSON: %v", err)
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
