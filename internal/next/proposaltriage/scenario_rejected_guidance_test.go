package proposaltriage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestScenario_DistillerAvoidsReProposingRejectedGuidance(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"

	// === Seed ===

	// Run-305: a validation_gap proposal "Avoid file-path contracts" that was rejected
	run305ID := "run-305"
	rejectedTitle := "Avoid file-path contracts"
	rejectedChange := "Replace file-path-based contracts with content-based contracts for migration tooling"
	rejectionReason := "We specifically need path-based contracts for our migration tooling"

	proposals305 := &reviewdistiller.DistillationResult{
		RunID:     run305ID,
		SpecID:    "spec-305",
		Outcome:   "rework_implementation_gap",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  "run-305-proposal-filepath",
				Type:                "validation_gap",
				Title:               rejectedTitle,
				WhatHappened:        "Migration tool relied on hardcoded file paths",
				WhatWasMissing:      "Content-based contract validation",
				ProposedChange:      rejectedChange,
				Rationale:           "File paths are brittle and break on refactors",
				Confidence:          "high",
				ConfidenceRationale: "Observed multiple path breakages",
				EvidenceReferences:  []string{"review-outcome.json"},
			},
		},
		CreatedAt: time.Now(),
	}

	decisions305 := []Decision{
		{
			ProposalID: "run-305-proposal-filepath",
			Action:     "rejected",
			Reason:     rejectionReason,
			DecidedAt:  time.Now(),
		},
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, run305ID, proposals305, decisions305)

	// === Invoke ===

	// Step 1: Load rejected proposals (as the distillation stage would)
	rejectedJSON, err := LoadRejectedProposals(tmpDir, projectID)
	if err != nil {
		t.Fatalf("LoadRejectedProposals failed: %v", err)
	}

	// Step 2: Verify the rejected proposals contain the expected entry
	var rejected []map[string]interface{}
	if err := json.Unmarshal(rejectedJSON, &rejected); err != nil {
		t.Fatalf("failed to unmarshal rejected proposals: %v", err)
	}

	if len(rejected) != 1 {
		t.Fatalf("expected 1 rejected proposal, got %d", len(rejected))
	}

	rp := rejected[0]
	if rp["type"] != "validation_gap" {
		t.Errorf("rejected proposal type = %v, want 'validation_gap'", rp["type"])
	}
	if rp["title"] != rejectedTitle {
		t.Errorf("rejected proposal title = %v, want %q", rp["title"], rejectedTitle)
	}
	if rp["proposed_change"] != rejectedChange {
		t.Errorf("rejected proposal proposed_change = %v, want %q", rp["proposed_change"], rejectedChange)
	}
	if rp["rejection_reason"] != rejectionReason {
		t.Errorf("rejected proposal rejection_reason = %v, want %q", rp["rejection_reason"], rejectionReason)
	}

	// Step 3: Build distiller prompt with rejected proposals injected
	inputs := &reviewdistiller.DistillerInputs{
		RunID:             "run-306",
		SpecID:            "spec-306",
		SpecContent:       "# Migration Tooling Spec\nImplement migration validation.",
		ReviewOutcome:     json.RawMessage(`{"outcome": "rework_implementation_gap"}`),
		RejectedProposals: rejectedJSON,
	}

	prompt := reviewdistiller.BuildPrompt(inputs, "rework_implementation_gap")

	// === Assert ===

	// The prompt must include a "Previously Rejected Proposals" section
	if !strings.Contains(prompt, "Previously Rejected Proposals") {
		t.Error("prompt should include 'Previously Rejected Proposals' section")
	}

	// The rejected proposal title must appear in the prompt
	if !strings.Contains(prompt, rejectedTitle) {
		t.Error("prompt should include the rejected proposal title")
	}

	// The rejection reason must appear in the prompt
	if !strings.Contains(prompt, rejectionReason) {
		t.Error("prompt should include the rejection reason")
	}

	// The proposed change text must appear in the prompt
	if !strings.Contains(prompt, rejectedChange) {
		t.Error("prompt should include the rejected proposed change text")
	}

	// The prompt must instruct the distiller not to re-propose rejected guidance
	if !strings.Contains(prompt, "Do not re-propose guidance that matches previously rejected proposals") {
		t.Error("prompt should instruct the distiller to avoid re-proposing rejected guidance")
	}

	// Verify the rejected proposal no longer appears in pending proposals
	pending, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}
	for _, pp := range pending {
		if pp.Proposal.ID == "run-305-proposal-filepath" {
			t.Error("rejected proposal should not appear in pending proposals")
		}
	}
}
