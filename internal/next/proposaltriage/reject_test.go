package proposaltriage

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestReject_DismissedProposal(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-reject-dismissed-001"

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-123",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-reject-dismissed-001-proposal-gap1",
				Type:           "validation_gap",
				Title:          "Contract assertion pitfall",
				ProposedChange: "Avoid file-path-specific contract assertions when behavior can be verified by scenario tests",
				Rationale:      "File paths break during refactoring",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pendingProposals) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pendingProposals))
	}

	// Create a dismissed decision for the proposal
	dismissedDecision := Decision{
		ProposalID:  pendingProposals[0].Proposal.ID,
		Action:      "dismissed",
		Reason:      "Out of scope for this release",
		DismissedBy: "project-manager",
		DecidedAt:   time.Now(),
	}

	// Create the pending proposal for rejection
	rejectionPP := &PendingProposal{
		Proposal: pendingProposals[0].Proposal,
		RunID:    runID,
		SpecID:   pendingProposals[0].SpecID,
	}

	// Try to reject a proposal that already has a dismissed decision
	rejectionDecision, err := Reject(rejectionPP, "This should fail", &dismissedDecision)

	// Verify that Reject returns an error
	if err == nil {
		t.Fatal("Reject should return an error for dismissed proposal")
	}

	// Verify the error message contains the expected text
	if !strings.Contains(err.Error(), "dismissed") && !strings.Contains(err.Error(), "terminal state") {
		t.Errorf("error should contain 'dismissed' or 'terminal state', got: %v", err)
	}

	// Verify that no decision was returned
	if rejectionDecision != nil {
		t.Fatal("Reject should return nil decision for dismissed proposal")
	}
}

func TestReject_NeverAccepted(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-reject-never-001"

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-123",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-reject-never-001-proposal-gap1",
				Type:           "validation_gap",
				Title:          "Contract assertion pitfall",
				ProposedChange: "Avoid file-path-specific contract assertions when behavior can be verified by scenario tests",
				Rationale:      "File paths break during refactoring",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pendingProposals) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pendingProposals))
	}

	// Create a rejection decision without ever accepting
	rejectionPP := &PendingProposal{
		Proposal: pendingProposals[0].Proposal,
		RunID:    runID,
		SpecID:   pendingProposals[0].SpecID,
	}

	rejectionDecision, err := Reject(rejectionPP, "Too specific to this one-off migration", nil)
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if rejectionDecision == nil {
		t.Fatal("Reject returned nil decision")
	}

	// Verify the decision has the correct fields
	if rejectionDecision.ProposalID != "run-reject-never-001-proposal-gap1" {
		t.Errorf("proposal ID mismatch, got %q", rejectionDecision.ProposalID)
	}

	if rejectionDecision.Action != "rejected" {
		t.Errorf("action should be 'rejected', got %q", rejectionDecision.Action)
	}

	if rejectionDecision.Reason != "Too specific to this one-off migration" {
		t.Errorf("reason mismatch, got %q", rejectionDecision.Reason)
	}

	// Save the decision
	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*rejectionDecision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// Verify the decision was saved correctly
	loadedDecisions, err := LoadDecisions(evidenceDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	if len(loadedDecisions) != 1 {
		t.Fatalf("expected 1 saved decision, got %d", len(loadedDecisions))
	}

	if loadedDecisions[0].Action != "rejected" {
		t.Errorf("saved decision action should be 'rejected', got %q", loadedDecisions[0].Action)
	}

	if loadedDecisions[0].Reason != "Too specific to this one-off migration" {
		t.Errorf("saved decision reason mismatch, got %q", loadedDecisions[0].Reason)
	}

	// Verify no entries were created in stores
	playbookDir := filepath.Join(tmpDir, "playbook")
	pbStore := &playbook.Store{Dir: playbookDir}
	entries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("LoadPlaybook failed: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("expected 0 playbook entries (no acceptance), got %d", len(entries))
	}
}

func TestReject_AfterAccept_Playbook(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-reject-after-001"
	playbookDir := filepath.Join(tmpDir, "playbook")

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-123",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-reject-after-001-proposal-heur1",
				Type:           "planner_heuristic",
				Title:          "Add request memoization",
				ProposedChange: "Cache frequently accessed request results to improve performance",
				Rationale:      "Reduces redundant computations and improves response times",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pendingProposals) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pendingProposals))
	}

	// Accept the proposal
	pp := &PendingProposal{
		Proposal: pendingProposals[0].Proposal,
		RunID:    pendingProposals[0].RunID,
		SpecID:   pendingProposals[0].SpecID,
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	acceptedDecision, err := Promote(pp, "", "", "", nil, pbStore,
		"local", // use local scope
		"",      // evidenceDir
	)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if acceptedDecision == nil {
		t.Fatal("Promote returned nil decision")
	}

	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*acceptedDecision}); err != nil {
		t.Fatalf("SaveDecisions (accept) failed: %v", err)
	}

	// Verify entry was created in playbook
	entries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("LoadPlaybook failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 playbook entry after accept, got %d", len(entries))
	}

	if entries[0].Status != "active" {
		t.Errorf("entry status should be 'active', got %q", entries[0].Status)
	}

	// Now reject the previously accepted proposal
	rejectionDecision, err := Reject(pp, "Found more efficient approach with distributed caching", acceptedDecision)
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if rejectionDecision == nil {
		t.Fatal("Reject returned nil decision")
	}

	// Call RejectAfterAccept
	err = RejectAfterAccept(acceptedDecision, rejectionDecision, []Decision{*acceptedDecision}, nil, pbStore)
	if err != nil {
		t.Fatalf("RejectAfterAccept failed: %v", err)
	}

	// Save the rejection decision (overwriting the accepted decision)
	if err := SaveDecisions(evidenceDir, []Decision{*rejectionDecision}); err != nil {
		t.Fatalf("SaveDecisions (reject) failed: %v", err)
	}

	// Verify the playbook entry is now superseded
	entriesAfter, err := pbStore.Load()
	if err != nil {
		t.Fatalf("LoadPlaybook failed: %v", err)
	}

	if len(entriesAfter) != 1 {
		t.Fatalf("expected 1 playbook entry after reject, got %d", len(entriesAfter))
	}

	if entriesAfter[0].Status != "superseded" {
		t.Errorf("entry status should be 'superseded', got %q", entriesAfter[0].Status)
	}

	if entriesAfter[0].SupersededBy != rejectionDecision.ProposalID {
		t.Errorf("entry SupersededBy should be %q, got %q", rejectionDecision.ProposalID, entriesAfter[0].SupersededBy)
	}

	// Verify active entries list is empty
	activeEntries := playbook.ActiveEntries(entriesAfter)
	if len(activeEntries) != 0 {
		t.Fatalf("expected 0 active entries after rejection, got %d", len(activeEntries))
	}

	// Verify the decision was overwritten
	loadedDecisions, err := LoadDecisions(evidenceDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	if len(loadedDecisions) != 1 {
		t.Fatalf("expected 1 saved decision, got %d", len(loadedDecisions))
	}

	if loadedDecisions[0].Action != "rejected" {
		t.Errorf("saved decision action should be 'rejected', got %q", loadedDecisions[0].Action)
	}

	if loadedDecisions[0].Reason != "Found more efficient approach with distributed caching" {
		t.Errorf("saved decision reason mismatch, got %q", loadedDecisions[0].Reason)
	}
}

func TestReject_AfterAccept_Doctrine(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-reject-doctrine-001"
	doctrineDir := filepath.Join(tmpDir, "doctrine")

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-123",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-reject-doctrine-001-proposal-rule1",
				Type:           "doctrine_rule",
				Title:          "Interactive UI specs must include accessibility scenario checks",
				ProposedChange: "Require accessibility scenarios in all UI spec reviews",
				Rationale:      "Improves accessibility outcomes",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pendingProposals) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pendingProposals))
	}

	// Accept the proposal
	pp := &PendingProposal{
		Proposal: pendingProposals[0].Proposal,
		RunID:    pendingProposals[0].RunID,
		SpecID:   pendingProposals[0].SpecID,
	}

	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = doctrineDir
	acceptedDecision, err := Promote(pp, "", "", "", doctrineStore, nil,
		"local", // use local scope
		"",      // evidenceDir
	)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if acceptedDecision == nil {
		t.Fatal("Promote returned nil decision")
	}

	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*acceptedDecision}); err != nil {
		t.Fatalf("SaveDecisions (accept) failed: %v", err)
	}

	// Verify rule was created in doctrine
	doctrineDocs, err := doctrineStore.Load()
	if err != nil {
		t.Fatalf("LoadDoctrine failed: %v", err)
	}

	if len(doctrineDocs.Rules) != 1 {
		t.Fatalf("expected 1 doctrine rule after accept, got %d", len(doctrineDocs.Rules))
	}

	if doctrineDocs.Rules[0].Status != "active" {
		t.Errorf("rule status should be 'active', got %q", doctrineDocs.Rules[0].Status)
	}

	// Now reject the previously accepted proposal
	rejectionDecision, err := Reject(pp, "Incompatible with current dev process", acceptedDecision)
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if rejectionDecision == nil {
		t.Fatal("Reject returned nil decision")
	}

	// Call RejectAfterAccept
	err = RejectAfterAccept(acceptedDecision, rejectionDecision, []Decision{*acceptedDecision}, doctrineStore, nil)
	if err != nil {
		t.Fatalf("RejectAfterAccept failed: %v", err)
	}

	// Save the rejection decision (overwriting the accepted decision)
	if err := SaveDecisions(evidenceDir, []Decision{*rejectionDecision}); err != nil {
		t.Fatalf("SaveDecisions (reject) failed: %v", err)
	}

	// Verify the doctrine rule is now superseded
	doctrineDocsAfter, err := doctrineStore.Load()
	if err != nil {
		t.Fatalf("LoadDoctrine failed: %v", err)
	}

	if len(doctrineDocsAfter.Rules) != 1 {
		t.Fatalf("expected 1 doctrine rule after reject, got %d", len(doctrineDocsAfter.Rules))
	}

	if doctrineDocsAfter.Rules[0].Status != "superseded" {
		t.Errorf("rule status should be 'superseded', got %q", doctrineDocsAfter.Rules[0].Status)
	}

	if doctrineDocsAfter.Rules[0].SupersededBy != rejectionDecision.ProposalID {
		t.Errorf("rule SupersededBy should be %q, got %q", rejectionDecision.ProposalID, doctrineDocsAfter.Rules[0].SupersededBy)
	}

	// Verify active rules list is empty
	var activeRules []doctrine.Rule
	for _, rule := range doctrineDocsAfter.Rules {
		if rule.Status == "active" {
			activeRules = append(activeRules, rule)
		}
	}
	if len(activeRules) != 0 {
		t.Fatalf("expected 0 active rules after rejection, got %d", len(activeRules))
	}

	// Verify the decision was overwritten
	loadedDecisions, err := LoadDecisions(evidenceDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	if len(loadedDecisions) != 1 {
		t.Fatalf("expected 1 saved decision, got %d", len(loadedDecisions))
	}

	if loadedDecisions[0].Action != "rejected" {
		t.Errorf("saved decision action should be 'rejected', got %q", loadedDecisions[0].Action)
	}

	if loadedDecisions[0].Reason != "Incompatible with current dev process" {
		t.Errorf("saved decision reason mismatch, got %q", loadedDecisions[0].Reason)
	}
}

func TestReject_InvalidInputs(t *testing.T) {
	tests := []struct {
		name          string
		pp            *PendingProposal
		reason        string
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil PendingProposal",
			pp:            nil,
			reason:        "some reason",
			expectError:   true,
			errorContains: "pending proposal is nil",
		},
		{
			name: "nil Proposal in PendingProposal",
			pp: &PendingProposal{
				Proposal: nil,
			},
			reason:        "some reason",
			expectError:   true,
			errorContains: "pending proposal is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := Reject(tt.pp, tt.reason, nil)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				// Not checking error message contents since they may vary
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if decision == nil {
					t.Fatal("expected decision, got nil")
				}
			}
		})
	}
}

func TestRejectAfterAccept_InvalidInputs(t *testing.T) {
	tests := []struct {
		name          string
		acceptedDec   *Decision
		rejectionDec  *Decision
		doctrineStore doctrine.Store
		playbookStore *playbook.Store
		doctrineDir   string
		playbookDir   string
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil accepted decision",
			acceptedDec:   nil,
			rejectionDec:  &Decision{ProposalID: "prop1"},
			expectError:   true,
			errorContains: "decisions cannot be nil",
		},
		{
			name:          "nil rejection decision",
			acceptedDec:   &Decision{ProposalID: "prop1"},
			rejectionDec:  nil,
			expectError:   true,
			errorContains: "decisions cannot be nil",
		},
		{
			name:          "accepted decision missing materialized ID",
			acceptedDec:   &Decision{ProposalID: "prop1", MaterializedID: ""},
			rejectionDec:  &Decision{ProposalID: "prop2"},
			expectError:   true,
			errorContains: "accepted decision has no materialized ID",
		},
		{
			name:          "missing doctrine store for promoted entry",
			acceptedDec:   &Decision{ProposalID: "prop1", MaterializedID: "promoted-abc123"},
			rejectionDec:  &Decision{ProposalID: "prop2"},
			doctrineStore: nil,
			expectError:   true,
			errorContains: "doctrine store required for promoted entry",
		},
		{
			name:          "missing playbook store for playbook entry",
			acceptedDec:   &Decision{ProposalID: "prop1", MaterializedID: "pb-abc123"},
			rejectionDec:  &Decision{ProposalID: "prop2"},
			playbookStore: nil,
			playbookDir:   "",
			expectError:   true,
			errorContains: "playbook store required for playbook entry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RejectAfterAccept(tt.acceptedDec, tt.rejectionDec, []Decision{}, tt.doctrineStore, tt.playbookStore)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				// Not checking error message contents since they may vary
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestReject_DismissedProposalReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-reject-dismissed-001"

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-123",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-reject-dismissed-001-proposal-gap1",
				Type:           "validation_gap",
				Title:          "Error handling gap",
				ProposedChange: "Add nil check before accessing field",
				Rationale:      "Prevents runtime panics",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pendingProposals) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pendingProposals))
	}

	pp := &PendingProposal{
		Proposal: pendingProposals[0].Proposal,
		RunID:    runID,
		SpecID:   pendingProposals[0].SpecID,
	}

	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")

	// Create a dismissed decision for the proposal
	dismissedDecision := &Decision{
		ProposalID:  pp.Proposal.ID,
		Action:      "dismissed",
		DismissedBy: "some-other-proposal-id",
		DecidedAt:   time.Now(),
	}

	// Save the dismissed decision
	if err := SaveDecisions(evidenceDir, []Decision{*dismissedDecision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// Now try to reject the dismissed proposal
	decisions, err := LoadDecisions(evidenceDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}
	// Find the existing decision for this proposal
	existingDecision, found := FindExistingDecision(pp.Proposal.ID, decisions)
	var existingDecPtr *Decision
	if found {
		existingDecPtr = &existingDecision
	}
	rejectionDecision, err := Reject(pp, "Some rejection reason", existingDecPtr)

	// Should return an error because the proposal is in a terminal state (dismissed)
	if err == nil {
		t.Fatal("expected error when rejecting a dismissed proposal, got nil")
	}

	// Verify the error message mentions dismissed
	if !strings.Contains(err.Error(), "dismissed") {
		t.Errorf("error should mention dismissed, got: %v", err)
	}

	// Verify no decision was returned
	if rejectionDecision != nil {
		t.Errorf("expected nil decision, got: %v", rejectionDecision)
	}
}

func TestRejectAfterAccept_DismissedProposal(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-reject-after-dismissed-001"
	playbookDir := filepath.Join(tmpDir, "playbook")

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-123",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-reject-after-dismissed-001-proposal-heur1",
				Type:           "planner_heuristic",
				Title:          "Add caching layer",
				ProposedChange: "Implement a caching layer for frequent queries",
				Rationale:      "Improves performance for repeated queries",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pendingProposals) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pendingProposals))
	}

	// Accept the proposal
	pp := &PendingProposal{
		Proposal: pendingProposals[0].Proposal,
		RunID:    pendingProposals[0].RunID,
		SpecID:   pendingProposals[0].SpecID,
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	acceptedDecision, err := Promote(pp, "", "", "", nil, pbStore,
		"local", // use local scope
		"",      // evidenceDir
	)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if acceptedDecision == nil {
		t.Fatal("Promote returned nil decision")
	}

	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*acceptedDecision}); err != nil {
		t.Fatalf("SaveDecisions (accept) failed: %v", err)
	}

	// Create a dismissed decision for the same proposal
	dismissedDecision := Decision{
		ProposalID:  pp.Proposal.ID,
		Action:      "dismissed",
		Reason:      "Out of scope for this release",
		DismissedBy: "project-manager",
		DecidedAt:   time.Now(),
	}

	// Create a rejection decision
	rejectionDecision, err := Reject(pp, "Change of mind on approach", acceptedDecision)
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if rejectionDecision == nil {
		t.Fatal("Reject returned nil decision")
	}

	// Try to call RejectAfterAccept with both accepted and dismissed decisions
	// This should return an error because the proposal is in a terminal state (dismissed)
	// Pass dismissed decision first so it's found by FindExistingDecision
	err = RejectAfterAccept(acceptedDecision, rejectionDecision, []Decision{dismissedDecision, *acceptedDecision}, nil, pbStore)

	// Verify that RejectAfterAccept returns an error
	if err == nil {
		t.Fatal("RejectAfterAccept should return an error for dismissed proposal")
	}

	// Verify the error message contains the expected text
	if !strings.Contains(err.Error(), "dismissed") && !strings.Contains(err.Error(), "terminal state") {
		t.Errorf("error should contain 'dismissed' or 'terminal state', got: %v", err)
	}
}

func TestRejectAfterAccept_DismissedProposal_AcceptedFirst(t *testing.T) {
	// Variant of TestRejectAfterAccept_DismissedProposal where the accepted decision
	// comes FIRST in the decisions slice (dismissed second). This documents that
	// ValidateTerminalState uses first-match semantics: when the accepted entry is
	// found first the terminal-state check does not fire, so RejectAfterAccept
	// proceeds and supersedes the playbook entry without error.
	//
	// Contrast with TestRejectAfterAccept_DismissedProposal where dismissed comes
	// first and the call returns an error. Callers must ensure dismissed decisions
	// appear before accepted ones in the slice when they want the guard to trigger.
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-reject-after-dismissed-002"
	playbookDir := filepath.Join(tmpDir, "playbook")

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-123",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-reject-after-dismissed-002-proposal-heur1",
				Type:           "planner_heuristic",
				Title:          "Add caching layer variant",
				ProposedChange: "Implement a caching layer for frequent queries (variant)",
				Rationale:      "Improves performance",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingProposals, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pendingProposals) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pendingProposals))
	}

	// Accept the proposal
	pp := &PendingProposal{
		Proposal: pendingProposals[0].Proposal,
		RunID:    pendingProposals[0].RunID,
		SpecID:   pendingProposals[0].SpecID,
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	acceptedDecision, err := Promote(pp, "", "", "", nil, pbStore,
		"local",
		"",
	)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if acceptedDecision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Create a dismissed decision for the same proposal
	dismissedDecision := Decision{
		ProposalID:  pp.Proposal.ID,
		Action:      "dismissed",
		Reason:      "Reconsidered",
		DismissedBy: "tech-lead",
		DecidedAt:   time.Now(),
	}

	// Create a rejection decision (Reject sees acceptedDecision, not dismissed, so it succeeds)
	rejectionDecision, err := Reject(pp, "Approach abandoned", acceptedDecision)
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if rejectionDecision == nil {
		t.Fatal("Reject returned nil decision")
	}

	// Pass accepted decision FIRST, dismissed second.
	// FindExistingDecision returns the first match (accepted), so ValidateTerminalState
	// does NOT fire. RejectAfterAccept should succeed and supersede the playbook entry.
	err = RejectAfterAccept(acceptedDecision, rejectionDecision, []Decision{*acceptedDecision, dismissedDecision}, nil, pbStore)

	// With accepted first, the terminal-state guard does not trigger.
	if err != nil {
		t.Fatalf("expected RejectAfterAccept to succeed when accepted decision comes first, got: %v", err)
	}

	// Verify the playbook entry is now superseded (the rejection was applied)
	entriesAfter, err := pbStore.Load()
	if err != nil {
		t.Fatalf("failed to load playbook entries: %v", err)
	}

	if len(entriesAfter) != 1 {
		t.Fatalf("expected 1 playbook entry, got %d", len(entriesAfter))
	}

	if entriesAfter[0].Status != "superseded" {
		t.Errorf("expected entry status 'superseded', got %q", entriesAfter[0].Status)
	}
}
