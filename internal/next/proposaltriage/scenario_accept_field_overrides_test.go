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

func TestScenario_AcceptWithFieldOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-203"
	playbookDir := filepath.Join(tmpDir, "playbook")

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-203",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-203-proposal-f1f2f3f4",
				Type:           "validation_gap",
				Title:          "Contract assertions target wrong file",
				ProposedChange: "Avoid file-path-specific contract assertions when behavior can be verified by scenario tests",
				Rationale:      "File-path-specific assertions break on refactoring",
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
	if pendingBefore[0].Proposal.ID != "run-203-proposal-f1f2f3f4" {
		t.Fatalf("expected proposal ID run-203-proposal-f1f2f3f4, got %q", pendingBefore[0].Proposal.ID)
	}

	pp := &PendingProposal{
		Proposal: pendingBefore[0].Proposal,
		RunID:    pendingBefore[0].RunID,
		SpecID:   pendingBefore[0].SpecID,
	}

	pbStore := &playbook.Store{Dir: playbookDir}

	overrideTitle := "Prefer scenario tests over file-path contracts"
	overrideChange := "When a behavior can be verified by a scenario test, prefer that over file-path-specific contract assertions which break on refactoring"

	decision, err := Promote(
		pp,
		overrideTitle,
		overrideChange,
		"",  // no rationale override
		nil, // doctrineStore not needed for validation_gap
		pbStore,
	)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if decision == nil {
		t.Fatal("Accept returned nil decision")
	}

	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	if decision.Action != "accepted" {
		t.Errorf("decision action = %q, want %q", decision.Action, "accepted")
	}
	if decision.ProposalID != "run-203-proposal-f1f2f3f4" {
		t.Errorf("decision proposal ID = %q, want %q", decision.ProposalID, "run-203-proposal-f1f2f3f4")
	}
	if decision.ApprovedTitle != overrideTitle {
		t.Errorf("decision ApprovedTitle = %q, want %q", decision.ApprovedTitle, overrideTitle)
	}
	if decision.ApprovedChange != overrideChange {
		t.Errorf("decision ApprovedChange = %q, want %q", decision.ApprovedChange, overrideChange)
	}
	if decision.MaterializedID == "" {
		t.Fatal("decision MaterializedID should be set")
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
		t.Fatalf("expected 1 entry in playbook, got %d", len(entries))
	}

	entry := entries[0]

	if entry.ID != decision.MaterializedID {
		t.Errorf("entry ID = %q, want %q (materialized ID)", entry.ID, decision.MaterializedID)
	}
	if entry.Title != overrideTitle {
		t.Errorf("entry Title = %q, want overridden %q", entry.Title, overrideTitle)
	}
	if entry.Title == "Contract assertions target wrong file" {
		t.Error("entry Title still has original proposal title — override not applied")
	}
	if entry.Content != overrideChange {
		t.Errorf("entry Content = %q, want overridden %q", entry.Content, overrideChange)
	}
	if entry.Content == "Avoid file-path-specific contract assertions when behavior can be verified by scenario tests" {
		t.Error("entry Content still has original proposed change — override not applied")
	}
	if entry.Type != "validation_gap" {
		t.Errorf("entry Type = %q, want %q", entry.Type, "validation_gap")
	}
	if entry.Status != "active" {
		t.Errorf("entry Status = %q, want %q", entry.Status, "active")
	}
	if entry.SourceProposalID != "run-203-proposal-f1f2f3f4" {
		t.Errorf("entry SourceProposalID = %q, want %q", entry.SourceProposalID, "run-203-proposal-f1f2f3f4")
	}
	if entry.SourceRunID != runID {
		t.Errorf("entry SourceRunID = %q, want %q", entry.SourceRunID, runID)
	}

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

	saved := savedDecisions[0]
	if saved.ApprovedTitle != overrideTitle {
		t.Errorf("saved decision ApprovedTitle = %q, want %q", saved.ApprovedTitle, overrideTitle)
	}
	if saved.ApprovedChange != overrideChange {
		t.Errorf("saved decision ApprovedChange = %q, want %q", saved.ApprovedChange, overrideChange)
	}
	if saved.MaterializedID != entry.ID {
		t.Errorf("saved decision MaterializedID = %q, want %q", saved.MaterializedID, entry.ID)
	}

	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance failed: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after acceptance, got %d", len(pendingAfter))
	}
}
