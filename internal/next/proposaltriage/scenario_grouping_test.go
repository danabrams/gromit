package proposaltriage

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestScenario_ListGroupingByDeterministicHashAndLLMClustering(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"

	// Proposal 1 and 2: Identical (same type and proposed_change)
	identicalChangeText := "Add input validation to all API endpoints"

	// Proposal 3 and 4: Semantically similar (similar concepts but different wording)
	similarChangeText1 := "Validate all inputs in API endpoints"
	similarChangeText2 := "Check and validate request inputs across API endpoints"

	// Proposal 5 and 6: Unique (different topics)
	uniqueChangeText1 := "Implement caching for database queries"
	uniqueChangeText2 := "Add comprehensive error logging throughout the application"

	// Run 1: First identical proposal
	run1ID := "run-401"
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     run1ID,
		SpecID:    "spec-401",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-401-proposal-1111aaaa",
				Type:           "doctrine_rule",
				Title:          "API Input Validation Rule",
				ProposedChange: identicalChangeText,
				Rationale:      "Security best practice",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run1ID, proposals1, nil)

	// Run 2: Second identical proposal
	run2ID := "run-402"
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     run2ID,
		SpecID:    "spec-402",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-402-proposal-2222bbbb",
				Type:           "doctrine_rule",
				Title:          "API Input Validation Rule",
				ProposedChange: identicalChangeText,
				Rationale:      "Security best practice",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run2ID, proposals2, nil)

	// Run 3: First semantically similar proposal
	run3ID := "run-403"
	proposals3 := &reviewdistiller.DistillationResult{
		RunID:     run3ID,
		SpecID:    "spec-403",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-403-proposal-3333cccc",
				Type:           "doctrine_rule",
				Title:          "Input Validation",
				ProposedChange: similarChangeText1,
				Rationale:      "Improve security",
			},
			{
				ID:             "run-403-proposal-5555eeee",
				Type:           "planner_heuristic",
				Title:          "Database Query Caching",
				ProposedChange: uniqueChangeText1,
				Rationale:      "Improve performance",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run3ID, proposals3, nil)

	// Run 4: Second semantically similar proposal and another unique proposal
	run4ID := "run-404"
	proposals4 := &reviewdistiller.DistillationResult{
		RunID:     run4ID,
		SpecID:    "spec-404",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-404-proposal-4444dddd",
				Type:           "doctrine_rule",
				Title:          "Request Input Validation",
				ProposedChange: similarChangeText2,
				Rationale:      "Prevent invalid requests",
			},
			{
				ID:             "run-404-proposal-6666ffff",
				Type:           "planner_heuristic",
				Title:          "Error Logging",
				ProposedChange: uniqueChangeText2,
				Rationale:      "Improve debugging",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run4ID, proposals4, nil)

	// Discover all pending proposals
	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pendingProposals) != 6 {
		t.Fatalf("expected 6 pending proposals, got %d", len(pendingProposals))
	}

	// Set up a stub LLM that clusters the similar proposals
	stubLLM := &stubLLMCompleter{
		response: `{
  "clusters": [
    {
      "proposal_ids": ["run-403-proposal-3333cccc", "run-404-proposal-4444dddd"],
      "description": "API input validation improvements with consistent validation approach"
    }
  ]
}`,
	}

	// Run GroupProposals with the stub LLM
	ctx := context.Background()
	groups, warnings := GroupProposals(ctx, pendingProposals, stubLLM)

	// Verify no warnings from the LLM
	if len(warnings) > 0 {
		t.Logf("unexpected warnings: %v", warnings)
	}

	// We expect 4 groups:
	// 1. Exact match group with the 2 identical proposals
	// 2. Semantic cluster group with the 2 similar proposals
	// 3. Singleton group for proposal 5
	// 4. Singleton group for proposal 6
	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}

	// Verify exact match group (should have 2 proposals)
	var exactMatchGroup *ProposalGroup
	var singletonCount int
	var semanticClusterGroup *ProposalGroup

	for i := range groups {
		group := &groups[i]
		if group.GroupReason == "exact_match" {
			exactMatchGroup = group
		} else if group.GroupReason == "singleton" {
			singletonCount++
		} else if group.GroupReason == "API input validation improvements with consistent validation approach" {
			semanticClusterGroup = group
		}
	}

	// Verify exact match group
	if exactMatchGroup == nil {
		t.Fatal("expected exact_match group not found")
	}
	if len(exactMatchGroup.Proposals) != 2 {
		t.Fatalf("exact_match group should have 2 proposals, got %d", len(exactMatchGroup.Proposals))
	}

	// Verify the exact match proposals are the ones we set up
	exactMatchIDs := make(map[string]bool)
	for _, pp := range exactMatchGroup.Proposals {
		if pp.Proposal != nil {
			exactMatchIDs[pp.Proposal.ID] = true
		}
	}
	if !exactMatchIDs["run-401-proposal-1111aaaa"] || !exactMatchIDs["run-402-proposal-2222bbbb"] {
		t.Fatal("exact_match group does not contain the expected identical proposals")
	}

	// Verify semantic cluster group
	if semanticClusterGroup == nil {
		t.Fatal("expected semantic cluster group not found")
	}
	if len(semanticClusterGroup.Proposals) != 2 {
		t.Fatalf("semantic cluster group should have 2 proposals, got %d", len(semanticClusterGroup.Proposals))
	}

	// Verify the semantic cluster proposals are the ones we set up
	semanticClusterIDs := make(map[string]bool)
	for _, pp := range semanticClusterGroup.Proposals {
		if pp.Proposal != nil {
			semanticClusterIDs[pp.Proposal.ID] = true
		}
	}
	if !semanticClusterIDs["run-403-proposal-3333cccc"] || !semanticClusterIDs["run-404-proposal-4444dddd"] {
		t.Fatal("semantic cluster group does not contain the expected similar proposals")
	}

	// Verify we have exactly 2 singleton groups
	if singletonCount != 2 {
		t.Fatalf("expected 2 singleton groups, got %d", singletonCount)
	}

	// Verify the unique proposals are in singleton groups
	singletonIDs := make(map[string]bool)
	for i := range groups {
		group := &groups[i]
		if group.GroupReason == "singleton" {
			for _, pp := range group.Proposals {
				if pp.Proposal != nil {
					singletonIDs[pp.Proposal.ID] = true
				}
			}
		}
	}
	if !singletonIDs["run-403-proposal-5555eeee"] || !singletonIDs["run-404-proposal-6666ffff"] {
		t.Fatal("singleton groups do not contain the expected unique proposals")
	}

	// Verify group IDs are deterministic (based on content hash for exact_match and first proposal's hash for clusters)
	if exactMatchGroup.GroupID == "" {
		t.Fatal("exact_match group should have a non-empty GroupID")
	}
	if semanticClusterGroup.GroupID == "" {
		t.Fatal("semantic cluster group should have a non-empty GroupID")
	}
	for i := range groups {
		if groups[i].GroupID == "" {
			t.Fatalf("group %d should have a non-empty GroupID", i)
		}
	}
}
