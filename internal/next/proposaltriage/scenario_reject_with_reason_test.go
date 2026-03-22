package proposaltriage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

// TestScenario_RejectProposalWithReason tests the end-to-end flow of rejecting a pending proposal with a reason.
// Setup: temp store with run-204 containing a validation_gap proposal.
// Action: Call Reject on the pending proposal with a reason.
// Verify:
// - proposal-decisions.json in run-204's evidence directory contains a rejected decision with the given reason
// - The proposal no longer appears in DiscoverPending results
// - Neither doctrine nor playbook is modified
func TestScenario_RejectProposalWithReason(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-204"
	doctrineDir := filepath.Join(tmpDir, "doctrine")
	playbookDir := filepath.Join(tmpDir, "playbook")

	// === Seed ===

	// Create run with a validation_gap proposal
	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-migration",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-204-proposal-c9d0e1f2",
				Type:           "validation_gap",
				Title:          "Add migration rollback tests",
				ProposedChange: "Add automated rollback validation for one-off migrations",
				Rationale:      "Prevents data loss during failed migrations",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	// Seed initial doctrine with an existing rule (to verify it stays unchanged)
	docStore := doctrine.NewFSStore()
	docStore.Dir = doctrineDir
	initialDoctrine := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			{
				ID:      "existing-rule-1",
				Summary: "Existing doctrine rule",
				Scope:   "code",
				Status:  "active",
			},
		},
	}
	if err := docStore.Save(initialDoctrine); err != nil {
		t.Fatalf("failed to save initial doctrine: %v", err)
	}

	// Seed initial playbook with an existing entry (to verify it stays unchanged)
	pbStore := &playbook.Store{Dir: playbookDir}
	initialEntries := []playbook.Entry{
		{
			ID:     "pb-existing1",
			Type:   "planner_heuristic",
			Title:  "Existing playbook entry",
			Status: "active",
		},
	}
	if err := pbStore.Save(initialEntries); err != nil {
		t.Fatalf("failed to save initial playbook: %v", err)
	}

	// Verify proposal exists as pending before rejection
	pendingBefore, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before rejection failed: %v", err)
	}
	if len(pendingBefore) != 1 {
		t.Fatalf("expected 1 pending proposal before rejection, got %d", len(pendingBefore))
	}
	if pendingBefore[0].Proposal.ID != "run-204-proposal-c9d0e1f2" {
		t.Fatalf("expected proposal ID run-204-proposal-c9d0e1f2, got %q", pendingBefore[0].Proposal.ID)
	}

	// === Invoke ===

	pp := &PendingProposal{
		Proposal: pendingBefore[0].Proposal,
		RunID:    pendingBefore[0].RunID,
		SpecID:   pendingBefore[0].SpecID,
	}

	rejectionReason := "Too specific to this one-off migration"
	decision, err := Reject(pp, rejectionReason)
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Reject returned nil decision")
	}

	// Save the decision to the run's evidence directory
	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// === Assert ===

	// 1. proposal-decisions.json contains a rejected decision with the given reason
	decisionsData, err := os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("failed to read proposal-decisions.json: %v", err)
	}

	var savedDecisions []Decision
	if err := json.Unmarshal(decisionsData, &savedDecisions); err != nil {
		t.Fatalf("failed to unmarshal decisions: %v", err)
	}

	if len(savedDecisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(savedDecisions))
	}

	savedDecision := savedDecisions[0]
	if savedDecision.ProposalID != "run-204-proposal-c9d0e1f2" {
		t.Errorf("decision proposal ID mismatch, got %q", savedDecision.ProposalID)
	}
	if savedDecision.Action != "rejected" {
		t.Errorf("decision action should be 'rejected', got %q", savedDecision.Action)
	}
	if savedDecision.Reason != rejectionReason {
		t.Errorf("decision reason mismatch, got %q, want %q", savedDecision.Reason, rejectionReason)
	}
	if savedDecision.DecidedAt.IsZero() {
		t.Error("decision DecidedAt should not be zero")
	}
	if savedDecision.MaterializedID != "" {
		t.Errorf("rejected decision should have empty MaterializedID, got %q", savedDecision.MaterializedID)
	}

	// 2. Proposal no longer appears in DiscoverPending results
	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after rejection failed: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after rejection, got %d", len(pendingAfter))
	}

	// 3a. Doctrine is NOT modified
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("failed to load doctrine after rejection: %v", err)
	}
	if len(loadedDoctrine.Rules) != 1 {
		t.Errorf("doctrine rules count changed: got %d, want 1", len(loadedDoctrine.Rules))
	}
	if loadedDoctrine.Rules[0].ID != "existing-rule-1" {
		t.Errorf("doctrine rule ID changed, got %q, want existing-rule-1", loadedDoctrine.Rules[0].ID)
	}
	if loadedDoctrine.Rules[0].Status != "active" {
		t.Errorf("doctrine rule status changed, got %q, want active", loadedDoctrine.Rules[0].Status)
	}

	// 3b. Playbook is NOT modified
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("failed to load playbook after rejection: %v", err)
	}
	if len(loadedEntries) != 1 {
		t.Errorf("playbook entries count changed: got %d, want 1", len(loadedEntries))
	}
	if loadedEntries[0].ID != "pb-existing1" {
		t.Errorf("playbook entry ID changed, got %q, want pb-existing1", loadedEntries[0].ID)
	}
	if loadedEntries[0].Status != "active" {
		t.Errorf("playbook entry status changed, got %q, want active", loadedEntries[0].Status)
	}
}
