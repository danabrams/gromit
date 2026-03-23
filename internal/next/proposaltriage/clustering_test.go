package proposaltriage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

// TestClusterSemantically tests the success path where 2 of 4 proposals are clustered
// and the remaining 2 become singletons
func TestClusterSemantically(t *testing.T) {
	ctx := context.TODO()
	now := time.Now()

	// Create 4 proposals
	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "validation_gap",
		Title:          "Missing null check",
		ProposedChange: "Add null check before dereferencing pointer",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "validation_gap",
		Title:          "Null pointer dereference risk",
		ProposedChange: "Check for nil before using pointer",
		Confidence:     "high",
	}

	proposal3 := &reviewdistiller.Proposal{
		ID:             "p3",
		Type:           "doctrine_rule",
		Title:          "Input validation",
		ProposedChange: "Add validation for all external inputs",
		Confidence:     "medium",
	}

	proposal4 := &reviewdistiller.Proposal{
		ID:             "p4",
		Type:           "error_handling",
		Title:          "Missing error check",
		ProposedChange: "Add error check after function call",
		Confidence:     "medium",
	}

	pending1 := PendingProposal{
		Proposal:  proposal1,
		RunID:     "run1",
		SpecID:    "spec1",
		CreatedAt: now,
	}

	pending2 := PendingProposal{
		Proposal:  proposal2,
		RunID:     "run2",
		SpecID:    "spec1",
		CreatedAt: now.Add(time.Minute),
	}

	pending3 := PendingProposal{
		Proposal:  proposal3,
		RunID:     "run3",
		SpecID:    "spec1",
		CreatedAt: now.Add(2 * time.Minute),
	}

	pending4 := PendingProposal{
		Proposal:  proposal4,
		RunID:     "run4",
		SpecID:    "spec1",
		CreatedAt: now.Add(3 * time.Minute),
	}

	// LLM stub that clusters p1 and p2, leaving p3 and p4 ungrouped (they'll become singletons)
	llm := &stubLLMCompleter{
		response: `{
  "clusters": [
    {
      "proposal_ids": ["p1", "p2"],
      "description": "Null pointer safety checks"
    }
  ]
}`,
	}

	groups, err := ClusterSemantically(ctx, []PendingProposal{pending1, pending2, pending3, pending4}, llm)

	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	if len(groups) != 3 {
		t.Errorf("expected 3 groups (1 cluster + 2 singletons), got %d", len(groups))
	}

	// Verify the 2-proposal cluster exists
	foundCluster := false
	foundSingletons := 0

	for _, group := range groups {
		if len(group.Proposals) == 2 {
			foundCluster = true

			// Verify the 2 proposals are p1 and p2
			ids := make(map[string]bool)
			for _, pp := range group.Proposals {
				if pp.Proposal != nil {
					ids[pp.Proposal.ID] = true
				}
			}

			if !ids["p1"] || !ids["p2"] {
				t.Errorf("cluster expected p1 and p2, got %v", ids)
			}

			// Verify the LLM-generated description
			if group.GroupReason != "Null pointer safety checks" {
				t.Errorf("expected description 'Null pointer safety checks', got '%s'", group.GroupReason)
			}
		} else if len(group.Proposals) == 1 {
			foundSingletons++

			// Verify singletons have the correct reason
			if group.GroupReason != "singleton" {
				t.Errorf("singleton: expected GroupReason='singleton', got '%s'", group.GroupReason)
			}
		} else {
			t.Errorf("unexpected group size: %d", len(group.Proposals))
		}
	}

	if !foundCluster {
		t.Errorf("did not find expected 2-proposal cluster")
	}

	if foundSingletons != 2 {
		t.Errorf("expected 2 singletons, found %d", foundSingletons)
	}
}

// TestClusterSemanticallyDegradeOnLLMError verifies graceful degradation when LLM fails.
// When the LLM returns an error, all proposals should be returned as singleton groups
// and a non-nil error should be returned for the caller to log as a warning.
func TestClusterSemanticallyDegradeOnLLMError(t *testing.T) {
	ctx := context.TODO()
	now := time.Now()

	// Create 4 proposals
	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "validation_gap",
		Title:          "Missing null check",
		ProposedChange: "Add null check before dereferencing pointer",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "validation_gap",
		Title:          "Null pointer dereference risk",
		ProposedChange: "Check for nil before using pointer",
		Confidence:     "high",
	}

	proposal3 := &reviewdistiller.Proposal{
		ID:             "p3",
		Type:           "doctrine_rule",
		Title:          "Input validation",
		ProposedChange: "Add validation for all external inputs",
		Confidence:     "medium",
	}

	proposal4 := &reviewdistiller.Proposal{
		ID:             "p4",
		Type:           "error_handling",
		Title:          "Missing error check",
		ProposedChange: "Add error check after function call",
		Confidence:     "medium",
	}

	pending1 := PendingProposal{
		Proposal:  proposal1,
		RunID:     "run1",
		SpecID:    "spec1",
		CreatedAt: now,
	}

	pending2 := PendingProposal{
		Proposal:  proposal2,
		RunID:     "run2",
		SpecID:    "spec1",
		CreatedAt: now.Add(time.Minute),
	}

	pending3 := PendingProposal{
		Proposal:  proposal3,
		RunID:     "run3",
		SpecID:    "spec1",
		CreatedAt: now.Add(2 * time.Minute),
	}

	pending4 := PendingProposal{
		Proposal:  proposal4,
		RunID:     "run4",
		SpecID:    "spec1",
		CreatedAt: now.Add(3 * time.Minute),
	}

	// LLM stub that returns an error
	failingLLM := &stubLLMCompleter{
		err: fmt.Errorf("LLM service unavailable"),
	}

	groups, err := ClusterSemantically(ctx, []PendingProposal{pending1, pending2, pending3, pending4}, failingLLM)

	// Verify non-nil error is returned (caller should log as warning)
	if err == nil {
		t.Errorf("expected non-nil error when LLM fails, got nil")
	}

	// Verify all 4 proposals are returned as singletons
	if len(groups) != 4 {
		t.Errorf("expected 4 singleton groups on LLM failure, got %d", len(groups))
	}

	// Verify each group is a singleton with correct GroupReason
	for _, group := range groups {
		if len(group.Proposals) != 1 {
			t.Errorf("expected singleton group, got %d proposals", len(group.Proposals))
		}

		if group.GroupReason != "singleton" {
			t.Errorf("expected GroupReason='singleton', got '%s'", group.GroupReason)
		}

		// Verify GroupID is properly formatted as "singleton-<proposalID>"
		expectedID := fmt.Sprintf("singleton-%s", group.Proposals[0].Proposal.ID)
		if group.GroupID != expectedID {
			t.Errorf("expected GroupID '%s', got '%s'", expectedID, group.GroupID)
		}
	}

	// Verify all proposal IDs are present
	proposalIDs := make(map[string]bool)
	for _, group := range groups {
		for _, pp := range group.Proposals {
			if pp.Proposal != nil {
				proposalIDs[pp.Proposal.ID] = true
			}
		}
	}

	expectedIDs := map[string]bool{"p1": true, "p2": true, "p3": true, "p4": true}
	if len(proposalIDs) != len(expectedIDs) {
		t.Errorf("expected 4 unique proposal IDs, got %d: %v", len(proposalIDs), proposalIDs)
	}

	for id := range expectedIDs {
		if !proposalIDs[id] {
			t.Errorf("missing expected proposal ID: %s", id)
		}
	}
}
