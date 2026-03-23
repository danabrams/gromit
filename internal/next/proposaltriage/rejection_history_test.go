package proposaltriage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestLoadRejectedProposals(t *testing.T) {
	// Create a temporary directory for test data
	tmpDir := t.TempDir()

	// Create a run with proposals
	projectID := "test-project"
	run1ID := "run-001"
	run1Dir := filepath.Join(tmpDir, "runs", run1ID)
	if err := os.MkdirAll(run1Dir, 0755); err != nil {
		t.Fatalf("failed to create run directory: %v", err)
	}

	// Create run state
	runState := &runstore.RunState{
		RunID:     run1ID,
		SpecID:    "spec-001",
		ProjectID: projectID,
		Status:    runstore.StatusCompleted,
	}
	runStateBytes, _ := json.MarshalIndent(runState, "", "  ")
	if err := os.WriteFile(filepath.Join(run1Dir, "run.json"), runStateBytes, 0644); err != nil {
		t.Fatalf("failed to write run.json: %v", err)
	}

	// Create evidence directory and distillation-proposals.json
	evidenceDir := filepath.Join(run1Dir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		t.Fatalf("failed to create evidence directory: %v", err)
	}

	proposal1 := reviewdistiller.Proposal{
		ID:                  "prop-001",
		Type:                "doctrine_rule",
		Title:               "Add error validation",
		ProposedChange:      "Check for nil pointers",
		Confidence:          "high",
		WhatHappened:        "Code crashed with nil pointer",
		WhatWasMissing:      "Error validation",
		Rationale:           "Prevents crashes",
		ConfidenceRationale: "Test case reproduces issue",
	}

	proposal2 := reviewdistiller.Proposal{
		ID:                  "prop-002",
		Type:                "validation_gap",
		Title:               "Validate input bounds",
		ProposedChange:      "Add range check",
		Confidence:          "medium",
		WhatHappened:        "Out of bounds error",
		WhatWasMissing:      "Range validation",
		Rationale:           "Prevents out of bounds",
		ConfidenceRationale: "Similar patterns in codebase",
	}

	proposals := []reviewdistiller.Proposal{proposal1, proposal2}
	distResult := reviewdistiller.DistillationResult{
		RunID:     run1ID,
		SpecID:    "spec-001",
		Outcome:   "completed",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: proposals,
		CreatedAt: time.Now(),
	}

	distBytes, _ := json.MarshalIndent(distResult, "", "  ")
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), distBytes, 0644); err != nil {
		t.Fatalf("failed to write distillation-proposals.json: %v", err)
	}

	// Create proposal-decisions.json with one rejected proposal
	decisions := []Decision{
		{
			ProposalID: "prop-001",
			Action:     "rejected",
			Reason:     "Already implemented in codebase",
			DecidedAt:  time.Now(),
		},
		{
			ProposalID: "prop-002",
			Action:     "accepted",
			Reason:     "Good suggestion",
			DecidedAt:  time.Now(),
		},
	}

	decisionsBytes, _ := json.MarshalIndent(decisions, "", "  ")
	if err := os.WriteFile(filepath.Join(evidenceDir, "proposal-decisions.json"), decisionsBytes, 0644); err != nil {
		t.Fatalf("failed to write proposal-decisions.json: %v", err)
	}

	// Call LoadRejectedProposals
	result, err := LoadRejectedProposals(tmpDir, projectID)
	if err != nil {
		t.Fatalf("LoadRejectedProposals failed: %v", err)
	}

	// Verify result is a JSON array
	var rejected []map[string]interface{}
	if err := json.Unmarshal(result, &rejected); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// Should have exactly one rejected proposal
	if len(rejected) != 1 {
		t.Fatalf("expected 1 rejected proposal, got %d", len(rejected))
	}

	// Verify the rejected proposal has expected fields
	rejectedProposal := rejected[0]
	if rejectedProposal["type"] != "doctrine_rule" {
		t.Errorf("expected type 'doctrine_rule', got %v", rejectedProposal["type"])
	}
	if rejectedProposal["title"] != "Add error validation" {
		t.Errorf("expected title 'Add error validation', got %v", rejectedProposal["title"])
	}
	if rejectedProposal["proposed_change"] != "Check for nil pointers" {
		t.Errorf("expected proposed_change 'Check for nil pointers', got %v", rejectedProposal["proposed_change"])
	}
	if rejectedProposal["rejection_reason"] != "Already implemented in codebase" {
		t.Errorf("expected rejection_reason 'Already implemented in codebase', got %v", rejectedProposal["rejection_reason"])
	}
}
