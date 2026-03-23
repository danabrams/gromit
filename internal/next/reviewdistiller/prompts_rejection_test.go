package reviewdistiller

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPrompt_NilRejectedProposals_DoesNotIncludeSection(t *testing.T) {
	inputs := &DistillerInputs{
		RunID:             "run-nil",
		SpecID:            "spec-nil",
		SpecContent:       "spec content",
		RejectedProposals: nil,
	}

	prompt := BuildPrompt(inputs, "accepted")

	if strings.Contains(prompt, "Previously Rejected Proposals") {
		t.Errorf("prompt should NOT include 'Previously Rejected Proposals' section when RejectedProposals is nil")
	}
}

func TestBuildPrompt_EmptyArrayRejectedProposals_DoesNotIncludeSection(t *testing.T) {
	inputs := &DistillerInputs{
		RunID:             "run-empty",
		SpecID:            "spec-empty",
		SpecContent:       "spec content",
		RejectedProposals: []byte(`[]`),
	}

	prompt := BuildPrompt(inputs, "accepted")

	if strings.Contains(prompt, "Previously Rejected Proposals") {
		t.Errorf("prompt should NOT include 'Previously Rejected Proposals' section when RejectedProposals is an empty JSON array")
	}
}

func TestBuildPrompt_IncludesRejectedProposalsInPrompt(t *testing.T) {
	// Create two rejected proposals
	rejectedProposals := []map[string]interface{}{
		{
			"type":             "doctrine_rule",
			"title":            "Error Handling Strategy",
			"proposed_change":  "Always return explicit error structs",
			"rejection_reason": "Incompatible with current architecture",
		},
		{
			"type":             "planner_heuristic",
			"title":            "Pre-validation Checks",
			"proposed_change":  "Validate all inputs before processing",
			"rejection_reason": "Adds unnecessary latency",
		},
	}

	rejectedJSON, err := json.Marshal(rejectedProposals)
	if err != nil {
		t.Fatalf("failed to marshal rejected proposals: %v", err)
	}

	inputs := &DistillerInputs{
		RunID:             "run-123",
		SpecID:            "spec-456",
		SpecContent:       "spec content",
		RejectedProposals: rejectedJSON,
	}

	prompt := BuildPrompt(inputs, "accepted")

	// Verify "Previously Rejected Proposals" section is in the prompt
	if !strings.Contains(prompt, "Previously Rejected Proposals") {
		t.Errorf("prompt should include 'Previously Rejected Proposals' section")
	}

	// Verify first proposal title is included
	if !strings.Contains(prompt, "Error Handling Strategy") {
		t.Errorf("prompt should include first rejected proposal title")
	}

	// Verify first proposal rejection reason is included
	if !strings.Contains(prompt, "Incompatible with current architecture") {
		t.Errorf("prompt should include first rejected proposal rejection reason")
	}

	// Verify second proposal title is included
	if !strings.Contains(prompt, "Pre-validation Checks") {
		t.Errorf("prompt should include second rejected proposal title")
	}

	// Verify second proposal rejection reason is included
	if !strings.Contains(prompt, "Adds unnecessary latency") {
		t.Errorf("prompt should include second rejected proposal rejection reason")
	}
}
