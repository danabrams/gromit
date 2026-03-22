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

// TestScenario_AcceptDoctrineRule tests the end-to-end flow of accepting a doctrine_rule proposal.
// Setup: temp store with run containing distillation-proposals.json with a doctrine_rule proposal.
// Action: Call Accept on the pending proposal.
// Verify:
// - Rule appears in doctrine/rules.json with promoted-<hash> ID
// - Decision appears in proposal-decisions.json with action=accepted
// - Proposal no longer appears in DiscoverPending results
func TestScenario_AcceptDoctrineRule(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-123"
	doctrineDir := filepath.Join(tmpDir, "doctrine")
	playbookDir := filepath.Join(tmpDir, "playbook")

	// Setup: Create run with a doctrine_rule proposal
	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-123",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-123-proposal-rule1",
				Type:           "doctrine_rule",
				Title:          "Prefer explicit error handling",
				ProposedChange: "Always check errors immediately after operations",
				Rationale:      "Prevents silent failures and improves debugging",
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

	// Action: Call Accept
	doctrineStore := doctrine.NewFSStore()
	playbookStore := &playbook.Store{Dir: playbookDir}

	decision, err := Accept(pp, "", "", "", doctrineStore, playbookStore, doctrineDir, playbookDir, filepath.Join(tmpDir, "runs", runID, "evidence"))
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Accept returned nil decision")
	}

	// Save the decision
	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// Verify 1: Rule appears in doctrine/rules.json with promoted-<hash> ID
	doctrineData, err := os.ReadFile(filepath.Join(doctrineDir, "rules.json"))
	if err != nil {
		t.Fatalf("failed to read doctrine/rules.json: %v", err)
	}

	var doctrine doctrine.Doctrine
	if err := json.Unmarshal(doctrineData, &doctrine); err != nil {
		t.Fatalf("failed to unmarshal doctrine: %v", err)
	}

	if len(doctrine.Rules) != 1 {
		t.Fatalf("expected 1 rule in doctrine, got %d", len(doctrine.Rules))
	}

	rule := doctrine.Rules[0]
	if !isPromotedID(rule.ID) {
		t.Errorf("rule ID should start with 'promoted-', got %q", rule.ID)
	}
	if rule.Summary != "Prefer explicit error handling" {
		t.Errorf("rule summary mismatch, got %q", rule.Summary)
	}
	if rule.Status != "active" {
		t.Errorf("rule status should be active, got %q", rule.Status)
	}

	// Verify 2: Decision appears in proposal-decisions.json with action=accepted
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
	if savedDecision.ProposalID != "run-123-proposal-rule1" {
		t.Errorf("decision proposal ID mismatch, got %q", savedDecision.ProposalID)
	}
	if savedDecision.Action != "accepted" {
		t.Errorf("decision action should be 'accepted', got %q", savedDecision.Action)
	}
	if savedDecision.MaterializedID != rule.ID {
		t.Errorf("decision MaterializedID mismatch with rule ID, got %q, want %q", savedDecision.MaterializedID, rule.ID)
	}

	// Verify 3: Proposal no longer appears in DiscoverPending results
	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance failed: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after acceptance, got %d", len(pendingAfter))
	}
}

// isPromotedID is a helper to check if an ID is a promoted doctrine ID.
func isPromotedID(id string) bool {
	return len(id) > 9 && id[:9] == "promoted-"
}
