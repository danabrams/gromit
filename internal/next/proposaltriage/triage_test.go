package proposaltriage

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestDiscoverPending(t *testing.T) {
	tmpDir := t.TempDir()

	// Run 1: has proposals but no decisions (all pending)
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-abc1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
			{
				ID:             "run-1-proposal-abc2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)

	// Run 2: has proposals, some with decisions, some without
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-2-proposal-xyz1",
				Type:           "planner_heuristic",
				Title:          "Proposal 3",
				ProposedChange: "Change 3",
			},
			{
				ID:             "run-2-proposal-xyz2",
				Type:           "refinement_guidance",
				Title:          "Proposal 4",
				ProposedChange: "Change 4",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}
	decisions2 := []Decision{
		{
			ProposalID: "run-2-proposal-xyz1",
			Action:     "accepted",
			DecidedAt:  time.Now(),
		},
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, decisions2)

	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	// Should have 3 pending proposals: 2 from run-1 and 1 from run-2 (xyz2 is not decided)
	if len(pending) != 3 {
		t.Fatalf("Expected 3 pending proposals, got %d", len(pending))
	}

	// Verify pending proposals include the expected ones
	pendingIDs := make(map[string]bool)
	for _, p := range pending {
		pendingIDs[p.Proposal.ID] = true
	}

	expectedPending := map[string]bool{
		"run-1-proposal-abc1": true,
		"run-1-proposal-abc2": true,
		"run-2-proposal-xyz2": true,
	}

	for id := range expectedPending {
		if !pendingIDs[id] {
			t.Errorf("Expected pending proposal %s not found", id)
		}
	}

	if pendingIDs["run-2-proposal-xyz1"] {
		t.Error("Decided proposal run-2-proposal-xyz1 should not be pending")
	}
}

func TestDiscoverPending_NoProp(t *testing.T) {
	tmpDir := t.TempDir()

	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending should not error with no proposals, got: %v", err)
	}
	if pending == nil {
		t.Fatal("DiscoverPending should return empty slice, not nil")
	}
	if len(pending) != 0 {
		t.Fatalf("DiscoverPending should return empty slice, got %d pending proposals", len(pending))
	}
}

func TestDiscoverPending_SortsByCreatedTimeDescending(t *testing.T) {
	tmpDir := t.TempDir()

	// Create runs with different creation times
	// Run 1: created at 2026-03-20 10:00 (oldest)
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-p1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)

	// Run 2: created at 2026-03-21 10:00 (middle)
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-2-proposal-p2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, nil)

	// Run 3: created at 2026-03-22 10:00 (newest)
	proposals3 := &reviewdistiller.DistillationResult{
		RunID:     "run-3",
		SpecID:    "spec-3",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-3-proposal-p3",
				Type:           "planner_heuristic",
				Title:          "Proposal 3",
				ProposedChange: "Change 3",
			},
		},
		CreatedAt: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-3", proposals3, nil)

	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pending) != 3 {
		t.Fatalf("Expected 3 pending proposals, got %d", len(pending))
	}

	// Verify descending order by creation time (newest first)
	expectedOrder := []string{"run-3-proposal-p3", "run-2-proposal-p2", "run-1-proposal-p1"}
	for i, expectedID := range expectedOrder {
		if pending[i].Proposal.ID != expectedID {
			t.Errorf("Position %d: expected %s, got %s", i, expectedID, pending[i].Proposal.ID)
		}
	}
}

func TestDiscoverAll(t *testing.T) {
	tmpDir := t.TempDir()

	// Run 1: has proposals but no decisions
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-abc1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
			{
				ID:             "run-1-proposal-abc2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)

	// Run 2: has proposals with some decisions
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-2-proposal-xyz1",
				Type:           "planner_heuristic",
				Title:          "Proposal 3",
				ProposedChange: "Change 3",
			},
			{
				ID:             "run-2-proposal-xyz2",
				Type:           "refinement_guidance",
				Title:          "Proposal 4",
				ProposedChange: "Change 4",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}
	decisions2 := []Decision{
		{
			ProposalID: "run-2-proposal-xyz1",
			Action:     "accepted",
			DecidedAt:  time.Now(),
		},
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, decisions2)

	all, err := DiscoverAll(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	if len(all) != 4 {
		t.Fatalf("Expected 4 total proposals, got %d", len(all))
	}

	// Verify we have both pending and decided proposals
	decisionCount := 0
	pendingCount := 0
	proposalIDs := make(map[string]bool)

	for _, ap := range all {
		proposalIDs[ap.Proposal.ID] = true
		if ap.Decision != nil {
			decisionCount++
		} else {
			pendingCount++
		}
	}

	if decisionCount != 1 {
		t.Errorf("Expected 1 decided proposal, got %d", decisionCount)
	}
	if pendingCount != 3 {
		t.Errorf("Expected 3 pending proposals, got %d", pendingCount)
	}

	// Verify order: run-2 (newer) proposals come before run-1 (older)
	expectedOrder := []string{"run-2-proposal-xyz1", "run-2-proposal-xyz2", "run-1-proposal-abc1", "run-1-proposal-abc2"}
	for i, expectedID := range expectedOrder {
		if all[i].Proposal.ID != expectedID {
			t.Errorf("Position %d: expected %s, got %s", i, expectedID, all[i].Proposal.ID)
		}
	}
}

func TestDiscoverAll_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	all, err := DiscoverAll(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverAll should not error with no proposals, got: %v", err)
	}
	if all == nil {
		t.Fatal("DiscoverAll should return empty slice, not nil")
	}
	if len(all) != 0 {
		t.Fatalf("DiscoverAll should return empty slice, got %d proposals", len(all))
	}
}

func TestDiscoverAll_WithProposalTypeFilter(t *testing.T) {
	tmpDir := t.TempDir()

	proposals := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "proposal-1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
			{
				ID:             "proposal-2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
			{
				ID:             "proposal-3",
				Type:           "planner_heuristic",
				Title:          "Proposal 3",
				ProposedChange: "Change 3",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals, nil)

	// Filter for doctrine_rule and validation_gap only
	typeFilter := []string{"doctrine_rule", "validation_gap"}
	all, err := DiscoverAll(tmpDir, "test-project", &typeFilter, nil)

	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("Expected 2 filtered proposals, got %d", len(all))
	}

	expectedTypes := map[string]bool{"doctrine_rule": true, "validation_gap": true}
	for _, ap := range all {
		if !expectedTypes[ap.Proposal.Type] {
			t.Errorf("Got proposal with type %s, not in filter", ap.Proposal.Type)
		}
	}
}

func TestDiscoverAll_WithRunIDFilter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two runs
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "proposal-1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)

	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "proposal-2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, nil)

	// Filter for run-1 only
	runIDFilter := []string{"run-1"}
	all, err := DiscoverAll(tmpDir, "test-project", nil, &runIDFilter)

	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	if len(all) != 1 {
		t.Fatalf("Expected 1 filtered proposal, got %d", len(all))
	}

	if all[0].RunID != "run-1" {
		t.Errorf("Expected run-1, got %s", all[0].RunID)
	}
}

func TestLoadDecisions_FileDoesNotExist(t *testing.T) {
	tmpDir := t.TempDir()

	decisions, err := LoadDecisions(tmpDir)

	if err != nil {
		t.Fatalf("LoadDecisions should not error for missing file, got: %v", err)
	}
	if decisions == nil {
		t.Fatal("LoadDecisions should return empty slice, not nil")
	}
	if len(decisions) != 0 {
		t.Fatalf("LoadDecisions should return empty slice, got %d decisions", len(decisions))
	}
}

func TestLoadDecisions_FileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a decisions file
	decisionsData := []Decision{
		{
			ProposalID:     "proposal-1",
			Action:         "accepted",
			DecidedAt:      time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
			ApprovedTitle:  "Approved Title 1",
			ApprovedChange: "Approved Change 1",
		},
		{
			ProposalID: "proposal-2",
			Action:     "rejected",
			Reason:     "Not aligned with goals",
			DecidedAt:  time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		},
	}

	err := SaveDecisions(tmpDir, decisionsData)
	if err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// Now load them back
	loaded, err := LoadDecisions(tmpDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("Expected 2 decisions, got %d", len(loaded))
	}

	if loaded[0].ProposalID != "proposal-1" || loaded[0].Action != "accepted" {
		t.Errorf("Decision 1 mismatch: got %+v", loaded[0])
	}

	if loaded[1].ProposalID != "proposal-2" || loaded[1].Action != "rejected" {
		t.Errorf("Decision 2 mismatch: got %+v", loaded[1])
	}
}

func TestSaveDecision_NewDecision(t *testing.T) {
	tmpDir := t.TempDir()

	decision := Decision{
		ProposalID:     "proposal-1",
		Action:         "accepted",
		ApprovedTitle:  "New Title",
		ApprovedChange: "New Change",
		DecidedAt:      time.Now(),
	}

	err := SaveDecision(tmpDir, decision)
	if err != nil {
		t.Fatalf("SaveDecision failed: %v", err)
	}

	// Load and verify
	loaded, err := LoadDecisions(tmpDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(loaded))
	}

	if loaded[0].ProposalID != "proposal-1" || loaded[0].Action != "accepted" {
		t.Errorf("Decision mismatch: got %+v", loaded[0])
	}
}

func TestSaveDecision_OverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Save initial decision
	initialDecision := Decision{
		ProposalID: "proposal-1",
		Action:     "rejected",
		Reason:     "Initial reason",
		DecidedAt:  time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	err := SaveDecision(tmpDir, initialDecision)
	if err != nil {
		t.Fatalf("SaveDecision failed: %v", err)
	}

	// Overwrite with new decision (reject after accept scenario)
	newDecision := Decision{
		ProposalID:    "proposal-1",
		Action:        "accepted",
		ApprovedTitle: "Updated Title",
		DecidedAt:     time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}
	err = SaveDecision(tmpDir, newDecision)
	if err != nil {
		t.Fatalf("SaveDecision failed: %v", err)
	}

	// Load and verify we have only one decision with the new values
	loaded, err := LoadDecisions(tmpDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Expected 1 decision, got %d (should have overwritten)", len(loaded))
	}

	if loaded[0].Action != "accepted" {
		t.Errorf("Expected action 'accepted', got %s", loaded[0].Action)
	}
	if loaded[0].ApprovedTitle != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %s", loaded[0].ApprovedTitle)
	}
}

func TestSaveDecision_PreservesOtherDecisions(t *testing.T) {
	tmpDir := t.TempDir()

	// Save multiple decisions
	decisions := []Decision{
		{
			ProposalID: "proposal-1",
			Action:     "accepted",
			DecidedAt:  time.Now(),
		},
		{
			ProposalID: "proposal-2",
			Action:     "rejected",
			Reason:     "Not aligned",
			DecidedAt:  time.Now(),
		},
	}
	err := SaveDecisions(tmpDir, decisions)
	if err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// Now save a new decision for a different proposal
	newDecision := Decision{
		ProposalID: "proposal-3",
		Action:     "accepted",
		DecidedAt:  time.Now(),
	}
	err = SaveDecision(tmpDir, newDecision)
	if err != nil {
		t.Fatalf("SaveDecision failed: %v", err)
	}

	// Load and verify all three are present
	loaded, err := LoadDecisions(tmpDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("Expected 3 decisions, got %d", len(loaded))
	}

	// Check each proposal is present
	proposalIDs := make(map[string]bool)
	for _, d := range loaded {
		proposalIDs[d.ProposalID] = true
	}

	expectedIDs := map[string]bool{
		"proposal-1": true,
		"proposal-2": true,
		"proposal-3": true,
	}

	for id := range expectedIDs {
		if !proposalIDs[id] {
			t.Errorf("Expected proposal %s not found in decisions", id)
		}
	}
}
