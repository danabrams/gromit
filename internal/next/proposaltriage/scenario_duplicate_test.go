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

// TestScenario_DuplicateProposal tests the scenario where a proposal whose type and change text
// hash to an existing playbook entry ID is accepted. The expected behavior is:
// - No duplicate playbook entry is created
// - The decision records the materialized_id and sets duplicate_of to the existing entry ID
//
// Setup: Create two runs with proposals.
//   Run 1: planner_heuristic proposal with specific change text
//   Run 2: planner_heuristic proposal with identical type and change text
// Action:
//   1. Accept the first proposal -> creates playbook entry with ID pb-<hash>
//   2. Accept the second proposal -> should detect duplicate
// Verify:
//   - Only one playbook entry exists with ID pb-<hash>
//   - Second decision has duplicate_of set to pb-<hash>
//   - Second decision has materialized_id set to pb-<hash>
func TestScenario_DuplicateProposal(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	run1ID := "run-206"
	run2ID := "run-207"
	playbookDir := filepath.Join(tmpDir, "playbook")

	// Shared change text that will hash to the same ID
	sharedChangeText := "Prefer compile checks before full test suite"

	// Setup: Create run 1 with a planner_heuristic proposal
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

	// Setup: Create run 2 with an identical planner_heuristic proposal
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

	// Verify pending proposals exist (we'll use the original proposals from the test setup instead of discovered ones)
	pendingBefore, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before acceptance failed: %v", err)
	}
	if len(pendingBefore) != 2 {
		t.Fatalf("expected 2 pending proposals before acceptance, got %d", len(pendingBefore))
	}

	// Use the original proposals from the test setup (to avoid DiscoverAll pointer issues)
	proposal1 := proposals1.Proposals[0]
	proposal2 := proposals2.Proposals[0]

	// Action 1: Accept the first proposal
	stores := setupStores(t, tmpDir)
	doctrineDir := filepath.Join(tmpDir, "doctrine")

	pp1 := &PendingProposal{
		Proposal: &proposal1,
		RunID:    pendingBefore[0].RunID,
		SpecID:   pendingBefore[0].SpecID,
	}

	decision1, err := Accept(pp1, "", "", "", stores.doctrine, stores.playbook, doctrineDir, playbookDir, filepath.Join(tmpDir, "runs", run1ID, "evidence"))
	if err != nil {
		t.Fatalf("Accept for first proposal failed: %v", err)
	}

	if decision1 == nil {
		t.Fatal("Accept returned nil decision for first proposal")
	}

	// Save the first decision
	evidenceDir1 := filepath.Join(tmpDir, "runs", run1ID, "evidence")
	if err := SaveDecisions(evidenceDir1, []Decision{*decision1}); err != nil {
		t.Fatalf("SaveDecisions for first proposal failed: %v", err)
	}

	// Verify first decision has no duplicate_of
	if decision1.DuplicateOf != "" {
		t.Errorf("first decision should not have duplicate_of set, got %q", decision1.DuplicateOf)
	}

	// Save the first materialized ID for verification
	materializedID := decision1.MaterializedID
	if materializedID == "" {
		t.Fatal("first decision should have materialized_id set")
	}

	// Verify first entry was saved to playbook
	entriesAfterFirst, err := stores.playbook.Load()
	if err != nil {
		t.Fatalf("failed to load playbook entries after first accept: %v", err)
	}
	if len(entriesAfterFirst) != 1 {
		t.Fatalf("expected 1 playbook entry after first accept, got %d", len(entriesAfterFirst))
	}

	// Action 2: Accept the second proposal (should detect duplicate)
	pp2 := &PendingProposal{
		Proposal: &proposal2,
		RunID:    pendingBefore[1].RunID,
		SpecID:   pendingBefore[1].SpecID,
	}

	decision2, err := Accept(pp2, "", "", "", stores.doctrine, stores.playbook, doctrineDir, playbookDir, filepath.Join(tmpDir, "runs", run2ID, "evidence"))
	if err != nil {
		t.Fatalf("Accept for second proposal failed: %v", err)
	}

	if decision2 == nil {
		t.Fatal("Accept returned nil decision for second proposal")
	}

	// Save the second decision
	evidenceDir2 := filepath.Join(tmpDir, "runs", run2ID, "evidence")
	if err := SaveDecisions(evidenceDir2, []Decision{*decision2}); err != nil {
		t.Fatalf("SaveDecisions for second proposal failed: %v", err)
	}

	// Verify 1: Second decision has duplicate_of set to first entry ID
	if decision2.DuplicateOf != materializedID {
		t.Errorf("second decision duplicate_of should be %q, got %q", materializedID, decision2.DuplicateOf)
	}

	// Verify 2: Second decision has materialized_id set
	if decision2.MaterializedID != materializedID {
		t.Errorf("second decision materialized_id should be %q, got %q", materializedID, decision2.MaterializedID)
	}

	// Verify 3: Only one playbook entry exists
	playbookEntries, err := stores.playbook.Load()
	if err != nil {
		t.Fatalf("failed to load playbook entries: %v", err)
	}

	if len(playbookEntries) != 1 {
		t.Fatalf("expected 1 playbook entry (no duplicates created), got %d", len(playbookEntries))
	}

	// Verify the single entry has the correct ID
	entry := playbookEntries[0]
	if entry.ID != materializedID {
		t.Errorf("playbook entry ID mismatch, got %q, want %q", entry.ID, materializedID)
	}

	// Verify the entry is active and not superseded
	if entry.Status != "active" {
		t.Errorf("playbook entry status should be active, got %q", entry.Status)
	}

	// Verify 4: Pending proposals list no longer includes either proposal
	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance failed: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after both accepted, got %d", len(pendingAfter))
	}

	// Verify 5: Validate decisions are persisted in both evidence directories
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

	// Verify the second saved decision has duplicate_of set
	if savedDecisions2[0].DuplicateOf != materializedID {
		t.Errorf("saved second decision duplicate_of should be %q, got %q", materializedID, savedDecisions2[0].DuplicateOf)
	}
}

// testStores holds store instances for testing
type testStores struct {
	doctrine doctrine.Store
	playbook *playbook.Store
}

// setupStores creates test store instances
func setupStores(t *testing.T, tmpDir string) testStores {
	doctrineDir := filepath.Join(tmpDir, "doctrine")
	playbookDir := filepath.Join(tmpDir, "playbook")

	// Ensure directories exist
	if err := os.MkdirAll(doctrineDir, 0755); err != nil {
		t.Fatalf("failed to create doctrine dir: %v", err)
	}
	if err := os.MkdirAll(playbookDir, 0755); err != nil {
		t.Fatalf("failed to create playbook dir: %v", err)
	}

	doctrineStore := doctrine.NewFSStore()
	playbookStore := &playbook.Store{Dir: playbookDir}

	return testStores{
		doctrine: doctrineStore,
		playbook: playbookStore,
	}
}
