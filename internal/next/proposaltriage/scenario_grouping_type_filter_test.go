package proposaltriage

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestScenario_ListTypeFilterWithinGroups(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"

	// === Seed ===
	// Group A: 2 doctrine_rule + 1 planner_heuristic across 2 runs
	run1ID := "run-501"
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     run1ID,
		SpecID:    "spec-501",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-501-proposal-1111aaaa",
				Type:           "doctrine_rule",
				Title:          "API Validation Rule 1",
				ProposedChange: "Add validation to API endpoints",
				Rationale:      "Improve security",
			},
			{
				ID:             "run-501-proposal-2222bbbb",
				Type:           "doctrine_rule",
				Title:          "Error Handling Rule",
				ProposedChange: "Handle errors explicitly",
				Rationale:      "Prevent silent failures",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run1ID, proposals1, nil)

	run2ID := "run-502"
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     run2ID,
		SpecID:    "spec-502",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-502-proposal-3333cccc",
				Type:           "planner_heuristic",
				Title:          "Performance Heuristic",
				ProposedChange: "Optimize database queries",
				Rationale:      "Improve performance",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run2ID, proposals2, nil)

	// Group B: 2 validation_gap in a single run
	run3ID := "run-503"
	proposals3 := &reviewdistiller.DistillationResult{
		RunID:     run3ID,
		SpecID:    "spec-503",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-503-proposal-4444dddd",
				Type:           "validation_gap",
				Title:          "Input Validation Gap",
				ProposedChange: "Validate user inputs",
				Rationale:      "Prevent invalid data",
			},
			{
				ID:             "run-503-proposal-5555eeee",
				Type:           "validation_gap",
				Title:          "API Response Gap",
				ProposedChange: "Validate API responses",
				Rationale:      "Ensure data integrity",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run3ID, proposals3, nil)

	// Verify precondition: 5 pending proposals total
	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}
	if len(pendingProposals) != 5 {
		t.Fatalf("expected 5 pending proposals, got %d", len(pendingProposals))
	}

	// === Invoke ===
	// Stub LLM returns no clusters so each proposal becomes a singleton group
	stubLLM := &stubLLMCompleter{
		response: `{"clusters": []}`,
	}

	ctx := context.Background()
	groups, warnings := GroupProposals(ctx, pendingProposals, stubLLM)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}

	// All 5 proposals should be in separate singleton groups
	if len(groups) != 5 {
		t.Fatalf("expected 5 groups, got %d", len(groups))
	}

	// Filter groups by type=validation_gap
	validationGapGroups := FilterGroupsByType(groups, []string{"validation_gap"})

	// === Assert ===
	// Only the 2 validation_gap proposals should survive
	if len(validationGapGroups) != 2 {
		t.Fatalf("expected 2 groups with validation_gap type, got %d", len(validationGapGroups))
	}

	// Count total proposals in filtered groups
	totalProposalsInFiltered := 0
	for _, group := range validationGapGroups {
		for _, pp := range group.Proposals {
			totalProposalsInFiltered++
			if pp.Proposal.Type != "validation_gap" {
				t.Errorf("expected type validation_gap, got %q", pp.Proposal.Type)
			}
		}
	}
	if totalProposalsInFiltered != 2 {
		t.Fatalf("expected 2 proposals in filtered groups, got %d", totalProposalsInFiltered)
	}

	// Verify the correct proposal IDs are present
	expectedIDs := map[string]bool{
		"run-503-proposal-4444dddd": true,
		"run-503-proposal-5555eeee": true,
	}
	foundIDs := make(map[string]bool)
	for _, group := range validationGapGroups {
		for _, pp := range group.Proposals {
			if pp.Proposal != nil {
				foundIDs[pp.Proposal.ID] = true
			}
		}
	}
	for expectedID := range expectedIDs {
		if !foundIDs[expectedID] {
			t.Errorf("expected proposal ID %q not found in filtered groups", expectedID)
		}
	}
}
