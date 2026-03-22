package proposaltriage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

// TestScenario_RejectAfterAccept tests the end-to-end flow of accepting a proposal and then rejecting it.
// Setup: temp store with run containing a planner_heuristic proposal.
// Action: Call Accept to materialize the proposal, then Reject and RejectAfterAccept to supersede it.
// Verify:
// - After accept: entry appears in playbook with status=active and in ActiveEntries()
// - After reject: decision is created with action=rejected
// - After RejectAfterAccept: entry status is superseded, decision is overwritten, entry no longer in ActiveEntries()
func TestScenario_RejectAfterAccept(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-reject-001"
	playbookDir := filepath.Join(tmpDir, "playbook")

	// Setup: Create run with a planner_heuristic proposal
	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-123",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-reject-001-proposal-heur1",
				Type:           "planner_heuristic",
				Title:          "Add request memoization",
				ProposedChange: "Cache frequently accessed request results to improve performance",
				Rationale:      "Reduces redundant computations and improves response times",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	// Verify proposal exists as pending before acceptance
	pendingBefore, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before acceptance failed: %v", err)
	}
	if len(pendingBefore) != 1 {
		t.Fatalf("expected 1 pending proposal before acceptance, got %d", len(pendingBefore))
	}
	pp := &PendingProposal{
		Proposal: pendingBefore[0].Proposal,
		RunID:    pendingBefore[0].RunID,
		SpecID:   pendingBefore[0].SpecID,
	}

	// Action 1: Call Accept to materialize the proposal
	pbStore := &playbook.Store{Dir: playbookDir}
	acceptedDecision, err := Accept(pp, "", "", "", nil, pbStore, "", playbookDir, filepath.Join(tmpDir, "runs", runID, "evidence"))
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	if acceptedDecision == nil {
		t.Fatal("Accept returned nil decision")
	}

	// Save the accepted decision
	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*acceptedDecision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// Verify 1: Entry appears in playbook with status=active
	playbookData, err := os.ReadFile(filepath.Join(playbookDir, "entries.json"))
	if err != nil {
		t.Fatalf("failed to read playbook entries.json: %v", err)
	}

	var entries []playbook.Entry
	if err := json.Unmarshal(playbookData, &entries); err != nil {
		t.Fatalf("failed to unmarshal playbook entries: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in playbook after accept, got %d", len(entries))
	}

	entry := entries[0]
	if entry.ID != acceptedDecision.MaterializedID {
		t.Errorf("entry ID mismatch with materialized ID, got %q, want %q", entry.ID, acceptedDecision.MaterializedID)
	}
	if entry.Status != "active" {
		t.Errorf("entry status should be active, got %q", entry.Status)
	}
	if entry.Type != "planner_heuristic" {
		t.Errorf("entry type should be planner_heuristic, got %q", entry.Type)
	}
	if entry.Title != "Add request memoization" {
		t.Errorf("entry title mismatch, got %q", entry.Title)
	}

	// Verify 2: Entry appears in ActiveEntries()
	activeEntries := playbook.ActiveEntries(entries)
	if len(activeEntries) != 1 {
		t.Fatalf("expected 1 active entry, got %d", len(activeEntries))
	}

	// Verify 3: Proposal no longer appears in DiscoverPending after acceptance
	pendingAfterAccept, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance failed: %v", err)
	}
	if len(pendingAfterAccept) != 0 {
		t.Fatalf("expected 0 pending proposals after acceptance, got %d", len(pendingAfterAccept))
	}

	// Action 2: Create a rejection decision and call RejectAfterAccept
	rejectionPP := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:    "run-reject-001-proposal-heur2",
			Title: "Reject memoization approach",
		},
		RunID:  runID,
		SpecID: "spec-123",
	}

	rejectionDecision, err := Reject(rejectionPP, "Found more efficient approach with distributed caching")
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if rejectionDecision == nil {
		t.Fatal("Reject returned nil decision")
	}

	// Call RejectAfterAccept to supersede the entry
	err = RejectAfterAccept(acceptedDecision, rejectionDecision, nil, pbStore, "", playbookDir)
	if err != nil {
		t.Fatalf("RejectAfterAccept failed: %v", err)
	}

	// Save the rejection decision
	if err := SaveDecisions(evidenceDir, []Decision{*acceptedDecision, *rejectionDecision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// Verify 4: Entry status is now superseded
	playbookDataAfter, err := os.ReadFile(filepath.Join(playbookDir, "entries.json"))
	if err != nil {
		t.Fatalf("failed to read playbook entries.json after rejection: %v", err)
	}

	var entriesAfter []playbook.Entry
	if err := json.Unmarshal(playbookDataAfter, &entriesAfter); err != nil {
		t.Fatalf("failed to unmarshal playbook entries after rejection: %v", err)
	}

	if len(entriesAfter) != 1 {
		t.Fatalf("expected 1 entry in playbook after rejection, got %d", len(entriesAfter))
	}

	entryAfter := entriesAfter[0]
	if entryAfter.Status != "superseded" {
		t.Errorf("entry status should be superseded after rejection, got %q", entryAfter.Status)
	}

	if entryAfter.SupersededBy != rejectionDecision.ProposalID {
		t.Errorf("entry SupersededBy should be %q, got %q", rejectionDecision.ProposalID, entryAfter.SupersededBy)
	}

	// Verify 5: Entry no longer appears in ActiveEntries()
	activeEntriesAfter := playbook.ActiveEntries(entriesAfter)
	if len(activeEntriesAfter) != 0 {
		t.Fatalf("expected 0 active entries after rejection, got %d", len(activeEntriesAfter))
	}

	// Verify 6: Decision is overwritten with rejection
	decisionsData, err := os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("failed to read proposal-decisions.json: %v", err)
	}

	var savedDecisions []Decision
	if err := json.Unmarshal(decisionsData, &savedDecisions); err != nil {
		t.Fatalf("failed to unmarshal decisions: %v", err)
	}

	if len(savedDecisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(savedDecisions))
	}

	rejectionDecisionSaved := savedDecisions[1]
	if rejectionDecisionSaved.ProposalID != rejectionDecision.ProposalID {
		t.Errorf("rejection decision proposal ID mismatch, got %q", rejectionDecisionSaved.ProposalID)
	}
	if rejectionDecisionSaved.Action != "rejected" {
		t.Errorf("rejection decision action should be 'rejected', got %q", rejectionDecisionSaved.Action)
	}
	if rejectionDecisionSaved.Reason != "Found more efficient approach with distributed caching" {
		t.Errorf("rejection decision reason mismatch, got %q", rejectionDecisionSaved.Reason)
	}
}
