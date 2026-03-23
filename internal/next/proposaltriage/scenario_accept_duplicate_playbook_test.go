package proposaltriage

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestScenario_AcceptDuplicateProposalDoesNotCreateDuplicateMemory(t *testing.T) {
	tmpDir := t.TempDir()
	playbookDir := tmpDir

	sharedType := "planner_heuristic"
	sharedChange := "Cache repeated database queries to reduce load"

	expectedMaterializedID := playbook.ComputeID(sharedType, sharedChange)

	pbStore := &playbook.Store{Dir: playbookDir}
	existingEntries := []playbook.Entry{
		{
			ID:               expectedMaterializedID,
			Type:             sharedType,
			Title:            "Cache database queries",
			Content:          sharedChange,
			Rationale:        "Reduces redundant DB calls",
			Status:           "active",
			SourceProposalID: "run-206-proposal-1111aaaa",
			SourceRunID:      "run-206",
			SourceSpecID:     "spec-perf",
			CreatedAt:        time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		},
	}
	if err := pbStore.Save(existingEntries); err != nil {
		t.Fatalf("failed to seed playbook: %v", err)
	}

	laterProposal := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "run-207-proposal-2222bbbb",
			Type:           sharedType,
			Title:          "Cache database queries (rediscovered)",
			ProposedChange: sharedChange,
			Rationale:      "Same insight from a different run",
		},
		RunID:  "run-207",
		SpecID: "spec-perf-v2",
	}

	decision, err := Promote(
		laterProposal,
		"",  // no title override
		"",  // no change override
		"",  // no rationale override
		nil, // doctrineStore not needed for playbook type
		pbStore,

		"local", // use local scope
		"",      // evidenceDir
	)

	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if decision == nil {
		t.Fatal("Accept returned nil decision")
	}

	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	if decision.MaterializedID != expectedMaterializedID {
		t.Errorf("MaterializedID = %q, want %q", decision.MaterializedID, expectedMaterializedID)
	}

	if decision.DuplicateOf != expectedMaterializedID {
		t.Errorf("DuplicateOf = %q, want %q", decision.DuplicateOf, expectedMaterializedID)
	}

	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("failed to load playbook entries: %v", err)
	}
	if len(loadedEntries) != 1 {
		t.Fatalf("expected 1 playbook entry (no duplicate created), got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]
	if entry.ID != expectedMaterializedID {
		t.Errorf("entry ID = %q, want %q", entry.ID, expectedMaterializedID)
	}
	if entry.Status != "active" {
		t.Errorf("entry Status = %q, want %q", entry.Status, "active")
	}
	if entry.SourceProposalID != "run-206-proposal-1111aaaa" {
		t.Errorf("entry SourceProposalID = %q, want original proposal ID %q",
			entry.SourceProposalID, "run-206-proposal-1111aaaa")
	}
	if entry.Title != "Cache database queries" {
		t.Errorf("entry Title = %q, want original title %q", entry.Title, "Cache database queries")
	}
}
