package proposaltriage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestGroupByContentHash_IdenticalProposalsGrouped(t *testing.T) {
	// Two proposals with identical type + proposed_change should be grouped together
	now := time.Now()

	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "doctrine_rule",
		Title:          "First proposal",
		ProposedChange: "Add validation for email format",
		Rationale:      "To prevent invalid emails",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "doctrine_rule",
		Title:          "Different title",
		ProposedChange: "Add validation for email format",
		Rationale:      "Different rationale",
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

	groups := GroupByContentHash([]PendingProposal{pending1, pending2})

	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}

	if len(groups[0].Proposals) != 2 {
		t.Errorf("expected 2 proposals in group, got %d", len(groups[0].Proposals))
	}

	if groups[0].GroupReason != "exact_match" {
		t.Errorf("expected GroupReason='exact_match', got '%s'", groups[0].GroupReason)
	}

	if groups[0].GroupID == "" {
		t.Errorf("expected non-empty GroupID")
	}
}

func TestGroupByContentHash_DifferentProposalsRemainSingletons(t *testing.T) {
	// Proposals with different content should remain as separate groups (singletons)
	now := time.Now()

	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "doctrine_rule",
		Title:          "First proposal",
		ProposedChange: "Add validation for email format",
		Rationale:      "To prevent invalid emails",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "doctrine_rule",
		Title:          "Second proposal",
		ProposedChange: "Add validation for phone number",
		Rationale:      "To prevent invalid phone numbers",
		Confidence:     "medium",
	}

	proposal3 := &reviewdistiller.Proposal{
		ID:             "p3",
		Type:           "validation_gap",
		Title:          "Third proposal",
		ProposedChange: "Add validation for email format",
		Rationale:      "Different type",
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

	pending3 := PendingProposal{
		Proposal:  proposal3,
		RunID:     "run3",
		SpecID:    "spec1",
		CreatedAt: now.Add(2 * time.Minute),
	}

	groups := GroupByContentHash([]PendingProposal{pending1, pending2, pending3})

	if len(groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(groups))
	}

	for i, group := range groups {
		if len(group.Proposals) != 1 {
			t.Errorf("group %d: expected 1 proposal, got %d", i, len(group.Proposals))
		}
		if group.GroupReason != "exact_match" {
			t.Errorf("group %d: expected GroupReason='exact_match', got '%s'", i, group.GroupReason)
		}
	}
}

func TestGroupByContentHash_WhitespaceNormalization(t *testing.T) {
	// Proposals with whitespace differences in proposed_change should be grouped
	now := time.Now()

	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "doctrine_rule",
		Title:          "First",
		ProposedChange: "Add  validation\n\nfor email",
		Rationale:      "Reason 1",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "doctrine_rule",
		Title:          "Second",
		ProposedChange: "Add validation for email",
		Rationale:      "Reason 2",
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

	groups := GroupByContentHash([]PendingProposal{pending1, pending2})

	if len(groups) != 1 {
		t.Errorf("expected 1 group (whitespace normalized), got %d", len(groups))
	}

	if len(groups[0].Proposals) != 2 {
		t.Errorf("expected 2 proposals in group, got %d", len(groups[0].Proposals))
	}
}

func TestGroupByContentHash_EmptyInput(t *testing.T) {
	groups := GroupByContentHash([]PendingProposal{})

	if len(groups) != 0 {
		t.Errorf("expected 0 groups for empty input, got %d", len(groups))
	}
}

func TestGroupByContentHash_GroupIDConsistency(t *testing.T) {
	// Same content should always produce the same GroupID
	proposal := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "doctrine_rule",
		Title:          "Test",
		ProposedChange: "Add validation",
		Rationale:      "Reason",
		Confidence:     "high",
	}

	pending := PendingProposal{
		Proposal:  proposal,
		RunID:     "run1",
		SpecID:    "spec1",
		CreatedAt: time.Now(),
	}

	groups1 := GroupByContentHash([]PendingProposal{pending})
	groups2 := GroupByContentHash([]PendingProposal{pending})

	if groups1[0].GroupID != groups2[0].GroupID {
		t.Errorf("same content produced different GroupIDs: %s vs %s", groups1[0].GroupID, groups2[0].GroupID)
	}
}

func TestClusterSemantically_SingletonWhenLLMFails(t *testing.T) {
	// When LLM call fails, all ungrouped proposals should become singletons
	ctx := testContext(t)
	now := time.Now()

	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "validation_gap",
		Title:          "Missing null check",
		ProposedChange: "Add null check before dereferencing",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "validation_gap",
		Title:          "Missing bounds check",
		ProposedChange: "Add bounds check before array access",
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

	// LLM that always fails
	failingLLM := &stubLLMCompleter{err: fmt.Errorf("LLM unavailable")}

	groups, err := ClusterSemantically(ctx, []PendingProposal{pending1, pending2}, failingLLM)

	// Should not fail, but return singletons
	if err == nil {
		t.Errorf("expected non-nil error when LLM fails, got nil")
	}

	if len(groups) != 2 {
		t.Errorf("expected 2 singleton groups when LLM fails, got %d", len(groups))
	}

	for i, group := range groups {
		if len(group.Proposals) != 1 {
			t.Errorf("group %d: expected 1 proposal (singleton), got %d", i, len(group.Proposals))
		}
	}
}

func TestClusterSemantically_SemanticClustering(t *testing.T) {
	// When LLM succeeds, semantically similar proposals are clustered
	ctx := testContext(t)
	now := time.Now()

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
		Title:          "Null pointer dereference",
		ProposedChange: "Check for nil before using pointer",
		Confidence:     "high",
	}

	proposal3 := &reviewdistiller.Proposal{
		ID:             "p3",
		Type:           "doctrine_rule",
		Title:          "Always validate input",
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

	// LLM that clusters p1 and p2 together
	successLLM := &stubLLMCompleter{
		response: `{
  "clusters": [
    {
      "proposal_ids": ["p1", "p2"],
      "description": "Null pointer safety checks"
    },
    {
      "proposal_ids": ["p3"],
      "description": "Input validation guidance"
    }
  ]
}`,
	}

	groups, err := ClusterSemantically(ctx, []PendingProposal{pending1, pending2, pending3}, successLLM)

	if err != nil {
		t.Errorf("expected nil error when LLM succeeds, got: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(groups))
	}

	// First group should have p1 and p2
	if len(groups[0].Proposals) != 2 {
		t.Errorf("group 0: expected 2 proposals, got %d", len(groups[0].Proposals))
	}
	if groups[0].GroupReason != "Null pointer safety checks" {
		t.Errorf("group 0: expected description 'Null pointer safety checks', got '%s'", groups[0].GroupReason)
	}

	// Second group should have p3
	if len(groups[1].Proposals) != 1 {
		t.Errorf("group 1: expected 1 proposal, got %d", len(groups[1].Proposals))
	}
	if groups[1].GroupReason != "Input validation guidance" {
		t.Errorf("group 1: expected description 'Input validation guidance', got '%s'", groups[1].GroupReason)
	}
}

// testContext returns a context suitable for testing
func testContext(t *testing.T) context.Context {
	return context.TODO()
}

func TestGroupProposals_FullPipeline_ExactMatchesAndSemanticClusters(t *testing.T) {
	// Exercise the full pipeline: exact matches + ungrouped singletons + LLM clustering
	ctx := testContext(t)
	now := time.Now()

	// Create proposals with exact matches (two pairs with same content)
	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "doctrine_rule",
		Title:          "Email validation 1",
		ProposedChange: "Add validation for email format",
		Rationale:      "Prevent invalid emails",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "doctrine_rule",
		Title:          "Email validation 2",
		ProposedChange: "Add validation for email format", // exact match with p1
		Rationale:      "Different rationale",
		Confidence:     "medium",
	}

	// Different validation gap proposals that will be semantically clustered
	proposal3 := &reviewdistiller.Proposal{
		ID:             "p3",
		Type:           "validation_gap",
		Title:          "Null pointer check",
		ProposedChange: "Add nil check before dereferencing",
		Confidence:     "high",
	}

	proposal4 := &reviewdistiller.Proposal{
		ID:             "p4",
		Type:           "validation_gap",
		Title:          "Bounds check",
		ProposedChange: "Check bounds before array access",
		Confidence:     "high",
	}

	pending1 := PendingProposal{Proposal: proposal1, RunID: "run1", SpecID: "spec1", CreatedAt: now}
	pending2 := PendingProposal{Proposal: proposal2, RunID: "run2", SpecID: "spec1", CreatedAt: now.Add(time.Minute)}
	pending3 := PendingProposal{Proposal: proposal3, RunID: "run3", SpecID: "spec1", CreatedAt: now.Add(2 * time.Minute)}
	pending4 := PendingProposal{Proposal: proposal4, RunID: "run4", SpecID: "spec1", CreatedAt: now.Add(3 * time.Minute)}

	// LLM that clusters the validation gap proposals together
	llm := &stubLLMCompleter{
		response: `{
  "clusters": [
    {
      "proposal_ids": ["p3", "p4"],
      "description": "Pointer and bounds safety checks"
    }
  ]
}`,
	}

	groups, warnings := GroupProposals(ctx, []PendingProposal{pending1, pending2, pending3, pending4}, llm)

	// Expected: 2 groups (one exact match of p1+p2, one semantic cluster of p3+p4)
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	// Check exact match group
	var exactMatchGroup *ProposalGroup
	var semanticGroup *ProposalGroup
	for i := range groups {
		if groups[i].GroupReason == "exact_match" {
			exactMatchGroup = &groups[i]
		} else if groups[i].GroupReason == "Pointer and bounds safety checks" {
			semanticGroup = &groups[i]
		}
	}

	if exactMatchGroup == nil {
		t.Errorf("expected exact_match group, got none")
	} else {
		if len(exactMatchGroup.Proposals) != 2 {
			t.Errorf("exact_match group: expected 2 proposals, got %d", len(exactMatchGroup.Proposals))
		}
	}

	if semanticGroup == nil {
		t.Errorf("expected semantic cluster group, got none")
	} else {
		if len(semanticGroup.Proposals) != 2 {
			t.Errorf("semantic group: expected 2 proposals, got %d", len(semanticGroup.Proposals))
		}
	}

	// No warnings expected when LLM succeeds
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings on success, got %d: %v", len(warnings), warnings)
	}
}

func TestGroupProposals_LLMFailureGeneratesWarning(t *testing.T) {
	// When LLM fails, ungrouped proposals should become singletons and warning collected
	ctx := testContext(t)
	now := time.Now()

	// Two exact matches
	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "doctrine_rule",
		Title:          "Email validation 1",
		ProposedChange: "Add validation for email format",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "doctrine_rule",
		Title:          "Email validation 2",
		ProposedChange: "Add validation for email format", // exact match
		Confidence:     "medium",
	}

	// One ungrouped proposal
	proposal3 := &reviewdistiller.Proposal{
		ID:             "p3",
		Type:           "validation_gap",
		Title:          "Null check",
		ProposedChange: "Add nil check",
		Confidence:     "high",
	}

	pending1 := PendingProposal{Proposal: proposal1, RunID: "run1", SpecID: "spec1", CreatedAt: now}
	pending2 := PendingProposal{Proposal: proposal2, RunID: "run2", SpecID: "spec1", CreatedAt: now.Add(time.Minute)}
	pending3 := PendingProposal{Proposal: proposal3, RunID: "run3", SpecID: "spec1", CreatedAt: now.Add(2 * time.Minute)}

	// LLM that fails
	failingLLM := &stubLLMCompleter{err: fmt.Errorf("LLM service unavailable")}

	groups, warnings := GroupProposals(ctx, []PendingProposal{pending1, pending2, pending3}, failingLLM)

	// Expected: 2 groups (one exact match of p1+p2, one singleton p3)
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	// Check exact match group
	var exactMatchGroup *ProposalGroup
	var singletonGroup *ProposalGroup
	for i := range groups {
		if groups[i].GroupReason == "exact_match" {
			exactMatchGroup = &groups[i]
		} else if groups[i].GroupReason == "singleton" {
			singletonGroup = &groups[i]
		}
	}

	if exactMatchGroup == nil {
		t.Errorf("expected exact_match group, got none")
	} else {
		if len(exactMatchGroup.Proposals) != 2 {
			t.Errorf("exact_match group: expected 2 proposals, got %d", len(exactMatchGroup.Proposals))
		}
	}

	if singletonGroup == nil {
		t.Errorf("expected singleton group when LLM fails, got none")
	}

	// Warning should be collected when LLM fails
	if len(warnings) == 0 {
		t.Errorf("expected at least 1 warning when LLM fails, got %d", len(warnings))
	}
}

func TestGroupProposals_NilLLMSkipsClustering(t *testing.T) {
	// When LLM is nil, ungrouped proposals should become singletons with a warning
	ctx := testContext(t)
	now := time.Now()

	// Two exact matches
	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "doctrine_rule",
		Title:          "Email validation 1",
		ProposedChange: "Add validation for email format",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "doctrine_rule",
		Title:          "Email validation 2",
		ProposedChange: "Add validation for email format", // exact match
		Confidence:     "medium",
	}

	// One ungrouped proposal
	proposal3 := &reviewdistiller.Proposal{
		ID:             "p3",
		Type:           "validation_gap",
		Title:          "Null check",
		ProposedChange: "Add nil check",
		Confidence:     "high",
	}

	pending1 := PendingProposal{Proposal: proposal1, RunID: "run1", SpecID: "spec1", CreatedAt: now}
	pending2 := PendingProposal{Proposal: proposal2, RunID: "run2", SpecID: "spec1", CreatedAt: now.Add(time.Minute)}
	pending3 := PendingProposal{Proposal: proposal3, RunID: "run3", SpecID: "spec1", CreatedAt: now.Add(2 * time.Minute)}

	// Pass nil LLM
	groups, warnings := GroupProposals(ctx, []PendingProposal{pending1, pending2, pending3}, nil)

	// Expected: 2 groups (one exact match of p1+p2, one singleton p3)
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	// Check exact match group
	var exactMatchGroup *ProposalGroup
	var singletonGroup *ProposalGroup
	for i := range groups {
		if groups[i].GroupReason == "exact_match" {
			exactMatchGroup = &groups[i]
		} else if groups[i].GroupReason == "singleton" {
			singletonGroup = &groups[i]
		}
	}

	if exactMatchGroup == nil {
		t.Errorf("expected exact_match group, got none")
	} else {
		if len(exactMatchGroup.Proposals) != 2 {
			t.Errorf("exact_match group: expected 2 proposals, got %d", len(exactMatchGroup.Proposals))
		}
	}

	if singletonGroup == nil {
		t.Errorf("expected singleton group when LLM is nil, got none")
	}

	// Warning should be collected when LLM is nil
	if len(warnings) == 0 {
		t.Errorf("expected at least 1 warning when LLM is nil, got %d", len(warnings))
	}

	// Check warning message mentions nil LLM
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "LLM completer is nil") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about nil LLM, got warnings: %v", warnings)
	}
}

func TestFilterGroupsByType_MixedTypeGroup(t *testing.T) {
	// A group with mixed proposal types should be retained with only matching proposals.
	// This aligns with spec scenario: "list filters by type within groups"
	now := time.Now()

	proposal1 := &reviewdistiller.Proposal{
		ID:             "p1",
		Type:           "doctrine_rule",
		Title:          "First proposal",
		ProposedChange: "Add validation for email",
		Rationale:      "Prevent invalid emails",
		Confidence:     "high",
	}

	proposal2 := &reviewdistiller.Proposal{
		ID:             "p2",
		Type:           "validation_gap",
		Title:          "Second proposal",
		ProposedChange: "Add bounds checking",
		Rationale:      "Prevent array overflow",
		Confidence:     "high",
	}

	proposal3 := &reviewdistiller.Proposal{
		ID:             "p3",
		Type:           "validation_gap",
		Title:          "Third proposal",
		ProposedChange: "Add nil check",
		Rationale:      "Prevent null dereference",
		Confidence:     "high",
	}

	// Mixed-type group: contains doctrine_rule and 2 validation_gap
	mixedTypeGroup := ProposalGroup{
		GroupID:     "mixed-group-1",
		GroupReason: "semantic_cluster",
		Proposals: []PendingProposal{
			{
				Proposal:  proposal1,
				RunID:     "run1",
				SpecID:    "spec1",
				CreatedAt: now,
			},
			{
				Proposal:  proposal2,
				RunID:     "run2",
				SpecID:    "spec1",
				CreatedAt: now.Add(time.Minute),
			},
			{
				Proposal:  proposal3,
				RunID:     "run3",
				SpecID:    "spec1",
				CreatedAt: now.Add(2 * time.Minute),
			},
		},
	}

	// Filter by validation_gap
	filtered := FilterGroupsByType([]ProposalGroup{mixedTypeGroup}, "validation_gap")

	// Group should be retained but with only the 2 validation_gap proposals
	if len(filtered) != 1 {
		t.Errorf("expected 1 group (retained with filtered proposals), got %d", len(filtered))
	}

	if len(filtered[0].Proposals) != 2 {
		t.Errorf("expected 2 matching proposals in filtered group, got %d", len(filtered[0].Proposals))
	}

	// Verify the remaining proposals are the validation_gap ones
	for i, pp := range filtered[0].Proposals {
		if pp.Proposal.Type != "validation_gap" {
			t.Errorf("proposal %d: expected type 'validation_gap', got '%s'", i, pp.Proposal.Type)
		}
	}

	// Group metadata should be preserved
	if filtered[0].GroupID != "mixed-group-1" {
		t.Errorf("expected GroupID 'mixed-group-1', got '%s'", filtered[0].GroupID)
	}
	if filtered[0].GroupReason != "semantic_cluster" {
		t.Errorf("expected GroupReason 'semantic_cluster', got '%s'", filtered[0].GroupReason)
	}
}
