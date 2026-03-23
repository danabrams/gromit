package proposaltriage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestScenario_DismissedFilteringHiddenByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"

	// Run 1: Two proposals in a group
	run1ID := "run-501"
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     run1ID,
		SpecID:    "spec-501",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-501-proposal-aaaa",
				Type:           "doctrine_rule",
				Title:          "API Input Validation",
				ProposedChange: "Add validation to API endpoints",
				Rationale:      "Improve security",
			},
			{
				ID:             "run-501-proposal-bbbb",
				Type:           "doctrine_rule",
				Title:          "Alternative API Validation",
				ProposedChange: "Add validation to API endpoints",
				Rationale:      "Improve security differently",
			},
		},
		CreatedAt: time.Now(),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, run1ID, proposals1, nil)

	// Before dismissing: both proposals should be pending
	pendingBefore, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before dismissal failed: %v", err)
	}
	if len(pendingBefore) != 2 {
		t.Fatalf("expected 2 pending proposals before dismissal, got %d", len(pendingBefore))
	}

	// Create a dismissed decision for the second proposal
	evidenceDir := filepath.Join(tmpDir, "runs", run1ID, "evidence")
	dismissedDecision := Decision{
		ProposalID:  "run-501-proposal-bbbb",
		Action:      "dismissed",
		DismissedBy: "run-501-proposal-aaaa",
		DecidedAt:   time.Now(),
	}
	if err := SaveDecision(evidenceDir, dismissedDecision); err != nil {
		t.Fatalf("SaveDecision failed: %v", err)
	}

	// After dismissing: only one proposal should be pending
	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after dismissal failed: %v", err)
	}
	if len(pendingAfter) != 1 {
		t.Fatalf("expected 1 pending proposal after dismissal, got %d", len(pendingAfter))
	}
	if pendingAfter[0].Proposal.ID != "run-501-proposal-aaaa" {
		t.Errorf("pending proposal should be aaaa, got %q", pendingAfter[0].Proposal.ID)
	}

	// DiscoverAll should return both proposals
	all, err := DiscoverAll(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 proposals in DiscoverAll, got %d", len(all))
	}

	// Find the dismissed proposal in DiscoverAll
	var dismissedProposal *AllProposal
	for i := range all {
		if all[i].Proposal.ID == "run-501-proposal-bbbb" {
			dismissedProposal = &all[i]
			break
		}
	}

	if dismissedProposal == nil {
		t.Fatal("dismissed proposal not found in DiscoverAll")
	}

	// Verify the dismissed proposal has the dismissed decision attached
	if dismissedProposal.Decision == nil {
		t.Fatal("dismissed proposal should have a decision")
	}
	if dismissedProposal.Decision.Action != "dismissed" {
		t.Errorf("decision action should be 'dismissed', got %q", dismissedProposal.Decision.Action)
	}
	if dismissedProposal.Decision.DismissedBy != "run-501-proposal-aaaa" {
		t.Errorf("decision DismissedBy should be 'run-501-proposal-aaaa', got %q", dismissedProposal.Decision.DismissedBy)
	}
}
