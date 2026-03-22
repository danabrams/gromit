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

func TestScenario_RejectAfterAccept(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-reject-001"
	playbookDir := filepath.Join(tmpDir, "playbook")

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

	pbStore := &playbook.Store{Dir: playbookDir}
	acceptedDecision, err := Promote(pp, "", "", "", nil, pbStore)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	if acceptedDecision == nil {
		t.Fatal("Accept returned nil decision")
	}

	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*acceptedDecision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

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

	activeEntries := playbook.ActiveEntries(entries)
	if len(activeEntries) != 1 {
		t.Fatalf("expected 1 active entry, got %d", len(activeEntries))
	}

	pendingAfterAccept, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance failed: %v", err)
	}
	if len(pendingAfterAccept) != 0 {
		t.Fatalf("expected 0 pending proposals after acceptance, got %d", len(pendingAfterAccept))
	}

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

	err = RejectAfterAccept(acceptedDecision, rejectionDecision, nil, pbStore)
	if err != nil {
		t.Fatalf("RejectAfterAccept failed: %v", err)
	}

	if err := SaveDecisions(evidenceDir, []Decision{*acceptedDecision, *rejectionDecision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

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

	activeEntriesAfter := playbook.ActiveEntries(entriesAfter)
	if len(activeEntriesAfter) != 0 {
		t.Fatalf("expected 0 active entries after rejection, got %d", len(activeEntriesAfter))
	}

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
