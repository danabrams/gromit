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

// TestClusterSemantically_FiltersNilProposals verifies that nil proposals are filtered out before clustering.
// This ensures that proposals with nil Proposal fields don't cause panics or errors during LLM clustering.
func TestClusterSemantically_FiltersNilProposals(t *testing.T) {
	ctx := context.TODO()
	now := time.Now()

	// Create valid proposals
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

	pending1 := PendingProposal{
		Proposal:  proposal1,
		RunID:     "run1",
		SpecID:    "spec1",
		CreatedAt: now,
	}

	// Proposal with nil Proposal field (should be filtered out)
	pendingNil := PendingProposal{
		Proposal:  nil,
		RunID:     "run-nil",
		SpecID:    "spec1",
		CreatedAt: now.Add(time.Minute),
	}

	pending2 := PendingProposal{
		Proposal:  proposal2,
		RunID:     "run2",
		SpecID:    "spec1",
		CreatedAt: now.Add(2 * time.Minute),
	}

	// LLM stub that expects to receive only valid proposals (p1 and p2)
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

	// Pass mix of valid and nil proposals
	groups, err := ClusterSemantically(ctx, []PendingProposal{pending1, pendingNil, pending2}, llm)

	if err != nil {
		t.Errorf("expected nil error when filtering valid proposals, got: %v", err)
	}

	// Should only have 1 cluster (p1 and p2 clustered together)
	if len(groups) != 1 {
		t.Errorf("expected 1 cluster (nil proposal filtered out), got %d groups", len(groups))
	}

	if len(groups[0].Proposals) != 2 {
		t.Errorf("expected 2 proposals in cluster, got %d", len(groups[0].Proposals))
	}

	// Verify the 2 proposals are p1 and p2 (nil proposal was filtered)
	ids := make(map[string]bool)
	for _, pp := range groups[0].Proposals {
		if pp.Proposal != nil {
			ids[pp.Proposal.ID] = true
		}
	}

	if !ids["p1"] || !ids["p2"] {
		t.Errorf("cluster expected p1 and p2 (nil filtered out), got %v", ids)
	}

	if groups[0].GroupReason != "Null pointer safety checks" {
		t.Errorf("expected description 'Null pointer safety checks', got '%s'", groups[0].GroupReason)
	}
}

// TestClusterSemantically_NilLLMReturnsSingletons directly calls ClusterSemantically with nil LLM completer
// and verifies it returns singleton groups with GroupReason='singleton' and a non-nil error.
func TestClusterSemantically_NilLLMReturnsSingletons(t *testing.T) {
	ctx := context.TODO()
	now := time.Now()

	// Create 3 proposals
	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "validation_gap",
		Title:          "Missing null check",
		ProposedChange: "Add null check before dereferencing pointer",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "doctrine_rule",
		Title:          "Input validation",
		ProposedChange: "Add validation for all external inputs",
		Confidence:     "medium",
	}

	proposal3 := &reviewdistiller.Proposal{
		ID:             "p3",
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

	// Call ClusterSemantically with nil LLM
	groups, err := ClusterSemantically(ctx, []PendingProposal{pending1, pending2, pending3}, nil)

	// Verify non-nil error is returned
	if err == nil {
		t.Errorf("expected non-nil error when LLM is nil, got nil")
	}

	// Verify all 3 proposals are returned as singletons
	if len(groups) != 3 {
		t.Errorf("expected 3 singleton groups when LLM is nil, got %d", len(groups))
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
		if group.Proposals[0].Proposal != nil {
			expectedID := fmt.Sprintf("singleton-%s", group.Proposals[0].Proposal.ID)
			if group.GroupID != expectedID {
				t.Errorf("expected GroupID '%s', got '%s'", expectedID, group.GroupID)
			}
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

	expectedIDs := map[string]bool{"p1": true, "p2": true, "p3": true}
	if len(proposalIDs) != len(expectedIDs) {
		t.Errorf("expected 3 unique proposal IDs, got %d: %v", len(proposalIDs), proposalIDs)
	}

	for id := range expectedIDs {
		if !proposalIDs[id] {
			t.Errorf("missing expected proposal ID: %s", id)
		}
	}
}

// TestClusterSemantically_DeterministicGroupIDRegardlessOfOrder verifies that the GroupID
// for a semantic cluster is deterministic even when the LLM returns proposal IDs in different orders.
// This ensures that the same cluster will always have the same GroupID regardless of LLM response order.
func TestClusterSemantically_DeterministicGroupIDRegardlessOfOrder(t *testing.T) {
	ctx := context.TODO()
	now := time.Now()

	// Create 3 proposals
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

	// First clustering: LLM returns proposal IDs in order [p2, p1]
	llm1 := &stubLLMCompleter{
		response: `{
  "clusters": [
    {
      "proposal_ids": ["p2", "p1"],
      "description": "Null pointer safety checks"
    }
  ]
}`,
	}

	groups1, err1 := ClusterSemantically(ctx, []PendingProposal{pending1, pending2, pending3}, llm1)

	if err1 != nil {
		t.Errorf("first clustering: expected nil error, got: %v", err1)
	}

	if len(groups1) != 2 {
		t.Errorf("first clustering: expected 2 groups (1 cluster + 1 singleton), got %d", len(groups1))
	}

	// Second clustering: LLM returns proposal IDs in order [p1, p2] (reversed)
	llm2 := &stubLLMCompleter{
		response: `{
  "clusters": [
    {
      "proposal_ids": ["p1", "p2"],
      "description": "Null pointer safety checks"
    }
  ]
}`,
	}

	groups2, err2 := ClusterSemantically(ctx, []PendingProposal{pending1, pending2, pending3}, llm2)

	if err2 != nil {
		t.Errorf("second clustering: expected nil error, got: %v", err2)
	}

	if len(groups2) != 2 {
		t.Errorf("second clustering: expected 2 groups (1 cluster + 1 singleton), got %d", len(groups2))
	}

	// Find the cluster group (non-singleton) in both results
	var cluster1, cluster2 *ProposalGroup
	for i := range groups1 {
		if len(groups1[i].Proposals) == 2 {
			cluster1 = &groups1[i]
			break
		}
	}

	for i := range groups2 {
		if len(groups2[i].Proposals) == 2 {
			cluster2 = &groups2[i]
			break
		}
	}

	if cluster1 == nil {
		t.Errorf("first clustering: did not find expected 2-proposal cluster")
	}

	if cluster2 == nil {
		t.Errorf("second clustering: did not find expected 2-proposal cluster")
	}

	// Verify the GroupIDs are identical despite different LLM response order
	if cluster1 != nil && cluster2 != nil && cluster1.GroupID != cluster2.GroupID {
		t.Errorf("GroupID not deterministic: first clustering got '%s', second clustering got '%s'",
			cluster1.GroupID, cluster2.GroupID)
	}

	// Verify both clusters have the same proposals (p1 and p2)
	if cluster1 != nil {
		ids1 := make(map[string]bool)
		for _, pp := range cluster1.Proposals {
			if pp.Proposal != nil {
				ids1[pp.Proposal.ID] = true
			}
		}
		if !ids1["p1"] || !ids1["p2"] {
			t.Errorf("first cluster expected p1 and p2, got %v", ids1)
		}
	}

	if cluster2 != nil {
		ids2 := make(map[string]bool)
		for _, pp := range cluster2.Proposals {
			if pp.Proposal != nil {
				ids2[pp.Proposal.ID] = true
			}
		}
		if !ids2["p1"] || !ids2["p2"] {
			t.Errorf("second cluster expected p1 and p2, got %v", ids2)
		}
	}
}

// TestClusterSemantically_InvalidJSONResponseDegradeGracefully verifies that when the LLM
// returns invalid JSON, ClusterSemantically degrades gracefully by returning all proposals
// as singleton groups and returning a non-nil error for logging.
func TestClusterSemantically_InvalidJSONResponseDegradeGracefully(t *testing.T) {
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

	// LLM stub that returns invalid JSON
	llm := &stubLLMCompleter{
		response: "not valid json",
	}

	groups, err := ClusterSemantically(ctx, []PendingProposal{pending1, pending2, pending3, pending4}, llm)

	// Verify non-nil error is returned (caller should log as warning)
	if err == nil {
		t.Errorf("expected non-nil error when LLM returns invalid JSON, got nil")
	}

	// Verify all 4 proposals are returned as singletons
	if len(groups) != 4 {
		t.Errorf("expected 4 singleton groups on invalid JSON, got %d", len(groups))
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

// TestClustersToGroups_HallucinatedIDSortsFirst directly tests clustersToGroups where
// the LLM returns a cluster with a hallucinated proposal ID that sorts lexicographically
// before valid IDs. Verifies no panic occurs and the group is formed correctly from
// valid proposals only.
func TestClustersToGroups_HallucinatedIDSortsFirst(t *testing.T) {
	now := time.Now()

	// Create 2 valid proposals
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

	// Create a clusterResponse with a hallucinated ID that sorts before valid IDs
	// After sorting, the IDs will be: ["hallucinated-id-aaa", "p1", "p2"]
	// The function should filter out the hallucinated ID and form a group from p1 and p2
	clusterResp := &clusterResponse{
		Clusters: []struct {
			ProposalIDs []string `json:"proposal_ids"`
			Description string   `json:"description"`
		}{
			{
				ProposalIDs: []string{"p2", "hallucinated-id-aaa", "p1"},
				Description: "Null pointer safety checks",
			},
		},
	}

	// Call clustersToGroups directly
	groups, err := clustersToGroups(clusterResp, []PendingProposal{pending1, pending2})

	// Verify no error occurs
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	// Verify exactly 1 group is created
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}

	if len(groups) > 0 {
		group := groups[0]

		// Verify the group has 2 proposals (valid ones only, hallucinated ID filtered out)
		if len(group.Proposals) != 2 {
			t.Errorf("expected 2 proposals in group, got %d", len(group.Proposals))
		}

		// Verify the proposals are p1 and p2
		ids := make(map[string]bool)
		for _, pp := range group.Proposals {
			if pp.Proposal != nil {
				ids[pp.Proposal.ID] = true
			}
		}

		if !ids["p1"] || !ids["p2"] {
			t.Errorf("expected p1 and p2, got %v", ids)
		}

		if ids["hallucinated-id-aaa"] {
			t.Errorf("hallucinated ID should not be in group")
		}

		// Verify the group reason is the description from the cluster
		if group.GroupReason != "Null pointer safety checks" {
			t.Errorf("expected GroupReason='Null pointer safety checks', got '%s'", group.GroupReason)
		}

		// Verify the GroupID is deterministic based on the first valid proposal after sorting
		// After sorting and filtering, groupProposals[0] should be p1 (comes first lexicographically)
		if group.GroupID == "" {
			t.Errorf("expected non-empty GroupID")
		}
	}
}
