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

func TestScenario_DuplicateProposal(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	run1ID := "run-206"
	run2ID := "run-207"

	sharedChangeText := "Prefer compile checks before full test suite"

	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     run1ID,
		SpecID:    "spec-206",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-206-proposal-1111aaaa",
				Type:           "planner_heuristic",
				Title:          "Prefer compile checks before full test suite",
				ProposedChange: sharedChangeText,
				Rationale:      "Compile checks are faster and catch errors earlier",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, run1ID, proposals1, nil)

	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     run2ID,
		SpecID:    "spec-207",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-207-proposal-2222bbbb",
				Type:           "planner_heuristic",
				Title:          "Prefer compile checks before full test suite",
				ProposedChange: sharedChangeText,
				Rationale:      "Compile checks are faster and catch errors earlier",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, run2ID, proposals2, nil)

	pendingBefore, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before acceptance failed: %v", err)
	}
	if len(pendingBefore) != 2 {
		t.Fatalf("expected 2 pending proposals before acceptance, got %d", len(pendingBefore))
	}

	proposal1 := proposals1.Proposals[0]
	proposal2 := proposals2.Proposals[0]

	stores := setupStores(t, tmpDir)

	pp1 := &PendingProposal{
		Proposal: &proposal1,
		RunID:    pendingBefore[0].RunID,
		SpecID:   pendingBefore[0].SpecID,
	}

	decision1, err := Promote(pp1, "", "", "", stores.doctrine, stores.playbook,
		"local", // use local scope
		"",      // evidenceDir
	)
	if err != nil {
		t.Fatalf("Accept for first proposal failed: %v", err)
	}

	if decision1 == nil {
		t.Fatal("Accept returned nil decision for first proposal")
	}

	evidenceDir1 := filepath.Join(tmpDir, "runs", run1ID, "evidence")
	if err := SaveDecisions(evidenceDir1, []Decision{*decision1}); err != nil {
		t.Fatalf("SaveDecisions for first proposal failed: %v", err)
	}

	if decision1.DuplicateOf != "" {
		t.Errorf("first decision should not have duplicate_of set, got %q", decision1.DuplicateOf)
	}

	materializedID := decision1.MaterializedID
	if materializedID == "" {
		t.Fatal("first decision should have materialized_id set")
	}

	entriesAfterFirst, err := stores.playbook.Load()
	if err != nil {
		t.Fatalf("failed to load playbook entries after first accept: %v", err)
	}
	if len(entriesAfterFirst) != 1 {
		t.Fatalf("expected 1 playbook entry after first accept, got %d", len(entriesAfterFirst))
	}

	pp2 := &PendingProposal{
		Proposal: &proposal2,
		RunID:    pendingBefore[1].RunID,
		SpecID:   pendingBefore[1].SpecID,
	}

	decision2, err := Promote(pp2, "", "", "", stores.doctrine, stores.playbook,
		"local", // use local scope
		"",      // evidenceDir
	)
	if err != nil {
		t.Fatalf("Accept for second proposal failed: %v", err)
	}

	if decision2 == nil {
		t.Fatal("Accept returned nil decision for second proposal")
	}

	evidenceDir2 := filepath.Join(tmpDir, "runs", run2ID, "evidence")
	if err := SaveDecisions(evidenceDir2, []Decision{*decision2}); err != nil {
		t.Fatalf("SaveDecisions for second proposal failed: %v", err)
	}

	if decision2.DuplicateOf != materializedID {
		t.Errorf("second decision duplicate_of should be %q, got %q", materializedID, decision2.DuplicateOf)
	}

	if decision2.MaterializedID != materializedID {
		t.Errorf("second decision materialized_id should be %q, got %q", materializedID, decision2.MaterializedID)
	}

	playbookEntries, err := stores.playbook.Load()
	if err != nil {
		t.Fatalf("failed to load playbook entries: %v", err)
	}

	if len(playbookEntries) != 1 {
		t.Fatalf("expected 1 playbook entry (no duplicates created), got %d", len(playbookEntries))
	}

	entry := playbookEntries[0]
	if entry.ID != materializedID {
		t.Errorf("playbook entry ID mismatch, got %q, want %q", entry.ID, materializedID)
	}

	if entry.Status != "active" {
		t.Errorf("playbook entry status should be active, got %q", entry.Status)
	}

	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance failed: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after both accepted, got %d", len(pendingAfter))
	}

	decisions1Data, err := os.ReadFile(filepath.Join(evidenceDir1, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("failed to read decisions for run 1: %v", err)
	}

	var savedDecisions1 []Decision
	if err := json.Unmarshal(decisions1Data, &savedDecisions1); err != nil {
		t.Fatalf("failed to unmarshal decisions for run 1: %v", err)
	}

	if len(savedDecisions1) != 1 {
		t.Fatalf("expected 1 decision in run 1 evidence, got %d", len(savedDecisions1))
	}

	decisions2Data, err := os.ReadFile(filepath.Join(evidenceDir2, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("failed to read decisions for run 2: %v", err)
	}

	var savedDecisions2 []Decision
	if err := json.Unmarshal(decisions2Data, &savedDecisions2); err != nil {
		t.Fatalf("failed to unmarshal decisions for run 2: %v", err)
	}

	if len(savedDecisions2) != 1 {
		t.Fatalf("expected 1 decision in run 2 evidence, got %d", len(savedDecisions2))
	}

	if savedDecisions2[0].DuplicateOf != materializedID {
		t.Errorf("saved second decision duplicate_of should be %q, got %q", materializedID, savedDecisions2[0].DuplicateOf)
	}
}

type testStores struct {
	doctrine doctrine.Store
	playbook *playbook.Store
}

func setupStores(t *testing.T, tmpDir string) testStores {
	doctrineDir := filepath.Join(tmpDir, "doctrine")
	playbookDir := filepath.Join(tmpDir, "playbook")

	if err := os.MkdirAll(doctrineDir, 0755); err != nil {
		t.Fatalf("failed to create doctrine dir: %v", err)
	}
	if err := os.MkdirAll(playbookDir, 0755); err != nil {
		t.Fatalf("failed to create playbook dir: %v", err)
	}

	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = doctrineDir
	playbookStore := &playbook.Store{Dir: playbookDir}

	return testStores{
		doctrine: doctrineStore,
		playbook: playbookStore,
	}
}
