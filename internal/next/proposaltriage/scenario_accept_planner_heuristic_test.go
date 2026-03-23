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

func TestScenario_AcceptPlannerHeuristicIntoPlaybook(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-202"
	playbookDir := filepath.Join(tmpDir, "playbook")

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-202",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-202-proposal-e5f6a7b8",
				Type:           "planner_heuristic",
				Title:          "Prefer package-scoped compile checks before full test suite",
				ProposedChange: "Prefer package-scoped compile checks before full test suite",
				Rationale:      "Catches compilation errors faster with tighter feedback loops",
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
	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")

	decision, err := Promote(pp, "", "", "", nil, pbStore,
		"local", // use local scope
		"",      // evidenceDir
	)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if decision == nil {
		t.Fatal("Accept returned nil decision")
	}

	if err := SaveDecisions(evidenceDir, []Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	playbookData, err := os.ReadFile(filepath.Join(playbookDir, "entries.json"))
	if err != nil {
		t.Fatalf("failed to read playbook/entries.json: %v", err)
	}

	var entries []playbook.Entry
	if err := json.Unmarshal(playbookData, &entries); err != nil {
		t.Fatalf("failed to unmarshal playbook entries: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in playbook, got %d", len(entries))
	}

	entry := entries[0]

	if entry.Type != "planner_heuristic" {
		t.Errorf("entry type should be planner_heuristic, got %q", entry.Type)
	}

	if entry.Status != "active" {
		t.Errorf("entry status should be active, got %q", entry.Status)
	}

	if entry.Content != "Prefer package-scoped compile checks before full test suite" {
		t.Errorf("entry content mismatch, got %q", entry.Content)
	}

	if entry.ID != decision.MaterializedID {
		t.Errorf("entry ID %q does not match decision MaterializedID %q", entry.ID, decision.MaterializedID)
	}
	if len(entry.ID) < 4 || entry.ID[:3] != "pb-" {
		t.Errorf("entry ID should start with 'pb-', got %q", entry.ID)
	}

	if entry.SourceRunID != runID {
		t.Errorf("SourceRunID should be %q, got %q", runID, entry.SourceRunID)
	}
	if entry.SourceProposalID != "run-202-proposal-e5f6a7b8" {
		t.Errorf("SourceProposalID should be %q, got %q", "run-202-proposal-e5f6a7b8", entry.SourceProposalID)
	}
	if entry.SourceSpecID != "spec-202" {
		t.Errorf("SourceSpecID should be %q, got %q", "spec-202", entry.SourceSpecID)
	}
	if entry.CreatedAt.IsZero() {
		t.Error("entry CreatedAt should not be zero")
	}

	activeEntries := playbook.ActiveEntries(entries)
	if len(activeEntries) != 1 {
		t.Fatalf("expected 1 active entry, got %d", len(activeEntries))
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

	savedDecision := savedDecisions[0]
	if savedDecision.ProposalID != "run-202-proposal-e5f6a7b8" {
		t.Errorf("decision proposal ID mismatch, got %q", savedDecision.ProposalID)
	}
	if savedDecision.Action != "accepted" {
		t.Errorf("decision action should be 'accepted', got %q", savedDecision.Action)
	}
	if savedDecision.MaterializedID != entry.ID {
		t.Errorf("decision MaterializedID %q does not match entry ID %q", savedDecision.MaterializedID, entry.ID)
	}

	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance failed: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after acceptance, got %d", len(pendingAfter))
	}
}
