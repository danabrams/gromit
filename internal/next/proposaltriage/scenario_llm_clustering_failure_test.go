package proposaltriage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestScenario_LLMClusteringFailureDegradeGracefully(t *testing.T) {
	// === Seed ===
	tmpDir := t.TempDir()
	projectID := "test-project"

	// Proposals 1 and 2: Identical (same type and proposed_change) → exact match group
	identicalChangeText := "Add input validation to all API endpoints"

	run1ID := "run-601"
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     run1ID,
		SpecID:    "spec-601",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-601-proposal-aaaa",
				Type:           "doctrine_rule",
				Title:          "API Input Validation Rule",
				ProposedChange: identicalChangeText,
				Rationale:      "Security best practice",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run1ID, proposals1, nil)

	run2ID := "run-602"
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     run2ID,
		SpecID:    "spec-602",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-602-proposal-bbbb",
				Type:           "doctrine_rule",
				Title:          "API Input Validation",
				ProposedChange: identicalChangeText,
				Rationale:      "Prevent bad inputs",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run2ID, proposals2, nil)

	// Proposals 3, 4, 5: Each unique (different type or proposed_change) → singletons
	run3ID := "run-603"
	proposals3 := &reviewdistiller.DistillationResult{
		RunID:     run3ID,
		SpecID:    "spec-603",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-603-proposal-cccc",
				Type:           "validation_gap",
				Title:          "Missing null check",
				ProposedChange: "Add null check before dereferencing pointer",
				Rationale:      "Prevent nil panic",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run3ID, proposals3, nil)

	run4ID := "run-604"
	proposals4 := &reviewdistiller.DistillationResult{
		RunID:     run4ID,
		SpecID:    "spec-604",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-604-proposal-dddd",
				Type:           "planner_heuristic",
				Title:          "Database Query Caching",
				ProposedChange: "Implement caching for database queries",
				Rationale:      "Improve performance",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run4ID, proposals4, nil)

	run5ID := "run-605"
	proposals5 := &reviewdistiller.DistillationResult{
		RunID:     run5ID,
		SpecID:    "spec-605",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-605-proposal-eeee",
				Type:           "planner_heuristic",
				Title:          "Error Logging Strategy",
				ProposedChange: "Add comprehensive error logging throughout the application",
				Rationale:      "Improve debugging",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run5ID, proposals5, nil)

	// Verify all 5 proposals are pending
	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}
	if len(pendingProposals) != 5 {
		t.Fatalf("expected 5 pending proposals, got %d", len(pendingProposals))
	}

	// === Invoke ===
	// LLM endpoint is unreachable
	unreachableLLM := &stubLLMCompleter{
		err: fmt.Errorf("connection refused: LLM endpoint unreachable"),
	}

	ctx := context.Background()
	groups, warnings := GroupProposals(ctx, pendingProposals, unreachableLLM)

	// === Assert ===

	// 1. Deterministic hash grouping still works: the 2 identical proposals form an exact_match group
	var exactMatchGroup *ProposalGroup
	var singletonCount int

	for i := range groups {
		group := &groups[i]
		if group.GroupReason == "exact_match" {
			exactMatchGroup = group
		} else if group.GroupReason == "singleton" {
			singletonCount++
		}
	}

	if exactMatchGroup == nil {
		t.Fatal("expected exact_match group not found — hash grouping should work despite LLM failure")
	}
	if len(exactMatchGroup.Proposals) != 2 {
		t.Fatalf("exact_match group should have 2 proposals, got %d", len(exactMatchGroup.Proposals))
	}

	// Verify the exact match group contains the two identical proposals
	exactMatchIDs := make(map[string]bool)
	for _, pp := range exactMatchGroup.Proposals {
		if pp.Proposal != nil {
			exactMatchIDs[pp.Proposal.ID] = true
		}
	}
	if !exactMatchIDs["run-601-proposal-aaaa"] || !exactMatchIDs["run-602-proposal-bbbb"] {
		t.Fatal("exact_match group does not contain the expected identical proposals")
	}

	// 2. Remaining 3 proposals appear as singletons (LLM clustering failed, so no semantic groups)
	if singletonCount != 3 {
		t.Fatalf("expected 3 singleton groups when LLM is unreachable, got %d", singletonCount)
	}

	// Total groups: 1 exact match + 3 singletons = 4
	if len(groups) != 4 {
		t.Fatalf("expected 4 groups (1 exact_match + 3 singletons), got %d", len(groups))
	}

	// Verify singleton proposals are the 3 unique ones
	singletonIDs := make(map[string]bool)
	for i := range groups {
		if groups[i].GroupReason == "singleton" {
			for _, pp := range groups[i].Proposals {
				if pp.Proposal != nil {
					singletonIDs[pp.Proposal.ID] = true
				}
			}
		}
	}
	if !singletonIDs["run-603-proposal-cccc"] {
		t.Error("singleton groups missing proposal cccc")
	}
	if !singletonIDs["run-604-proposal-dddd"] {
		t.Error("singleton groups missing proposal dddd")
	}
	if !singletonIDs["run-605-proposal-eeee"] {
		t.Error("singleton groups missing proposal eeee")
	}

	// 3. A warning about clustering failure is displayed
	if len(warnings) == 0 {
		t.Fatal("expected at least 1 warning about LLM clustering failure, got 0")
	}

	foundClusteringWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "semantic clustering failed") {
			foundClusteringWarning = true
			break
		}
	}
	if !foundClusteringWarning {
		t.Errorf("expected warning containing 'semantic clustering failed', got: %v", warnings)
	}

	// 4. All group IDs are non-empty (deterministic IDs work without LLM)
	for i := range groups {
		if groups[i].GroupID == "" {
			t.Errorf("group %d (%s) should have a non-empty GroupID", i, groups[i].GroupReason)
		}
	}
}
