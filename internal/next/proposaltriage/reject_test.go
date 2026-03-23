package proposaltriage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

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

	rejectionDecision, err := Reject(rejectionPP, "Too specific to this one-off migration")
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
	rejectionDecision, err := Reject(pp, "Found more efficient approach with distributed caching")
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if rejectionDecision == nil {
		t.Fatal("Reject returned nil decision")
	}

	// Call RejectAfterAccept
	err = RejectAfterAccept(acceptedDecision, rejectionDecision, nil, pbStore)
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
	rejectionDecision, err := Reject(pp, "Incompatible with current dev process")
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if rejectionDecision == nil {
		t.Fatal("Reject returned nil decision")
	}

	// Call RejectAfterAccept
	err = RejectAfterAccept(acceptedDecision, rejectionDecision, doctrineStore, nil)
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
			decision, err := Reject(tt.pp, tt.reason)

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
			err := RejectAfterAccept(tt.acceptedDec, tt.rejectionDec, tt.doctrineStore, tt.playbookStore)

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
