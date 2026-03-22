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

// helperCreateRunWithProposals creates a run directory structure with proposals and decisions.
// Returns the root directory where "runs" subdirectory is created.
func helperCreateRunWithProposals(t *testing.T, rootDir string, projectID string, runID string, proposals *reviewdistiller.DistillationResult, decisions []Decision) {
	store := runstore.NewStore(rootDir)

	// Create a run state
	specID := "test-spec"
	if proposals != nil {
		specID = proposals.SpecID
	}

	rs := &runstore.RunState{
		RunID:     runID,
		ProjectID: projectID,
		SpecID:    specID,
		Status:    "completed",
		StartedAt: time.Now(),
	}
	rs.NormalizeNilFields()

	if err := store.Save(rs); err != nil {
		t.Fatalf("failed to save run state: %v", err)
	}

	// Write proposals to evidence directory
	evidenceDir := store.RunEvidenceDir(runID)
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		t.Fatalf("failed to create evidence dir: %v", err)
	}

	if proposals != nil {
		proposalsData, err := json.MarshalIndent(proposals, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal proposals: %v", err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), proposalsData, 0644); err != nil {
			t.Fatalf("failed to write proposals: %v", err)
		}
	}

	// Write decisions
	if len(decisions) > 0 {
		if err := SaveDecisions(evidenceDir, decisions); err != nil {
			t.Fatalf("failed to save decisions: %v", err)
		}
	}
}

func TestDiscoverPending_NoRuns(t *testing.T) {
	tmpDir := t.TempDir()

	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending should not error with no runs, got: %v", err)
	}
	if pending == nil {
		t.Fatal("DiscoverPending should return empty slice, not nil")
	}
	if len(pending) != 0 {
		t.Fatalf("DiscoverPending should return empty slice, got %d pending proposals", len(pending))
	}
}

func TestDiscoverPending_EmptyRunsDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create runs subdirectory but leave it empty
	runsDir := filepath.Join(tmpDir, "runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatalf("failed to create runs directory: %v", err)
	}

	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending should not error with empty runs dir, got: %v", err)
	}
	if pending == nil {
		t.Fatal("DiscoverPending should return empty slice, not nil")
	}
	if len(pending) != 0 {
		t.Fatalf("DiscoverPending should return empty slice, got %d pending proposals", len(pending))
	}
}

func TestDiscoverPending_NoRunsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Explicitly verify that we don't create any runs/ subdirectory at all
	runsDir := filepath.Join(tmpDir, "runs")
	if _, err := os.Stat(runsDir); !os.IsNotExist(err) {
		t.Fatalf("test setup error: runs directory should not exist")
	}

	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending should not error when runs/ directory does not exist, got: %v", err)
	}
	if pending == nil {
		t.Fatal("DiscoverPending should return empty slice, not nil")
	}
	if len(pending) != 0 {
		t.Fatalf("DiscoverPending should return empty slice, got %d pending proposals", len(pending))
	}
}

func TestDiscoverPending_ReturnsProposalsWithoutDecisions(t *testing.T) {
	tmpDir := t.TempDir()

	// Run 1: has distillation-proposals.json but NO proposal-decisions.json
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-abc1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
			{
				ID:             "run-1-proposal-abc2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Run 2: has no proposals file at all (nil proposals)

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", nil, nil)

	// Discover pending proposals
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pending) != 2 {
		t.Fatalf("expected 2 pending proposals from run-1, got %d", len(pending))
	}

	// Verify both proposals from run-1 are returned
	foundProposals := make(map[string]bool)
	for _, p := range pending {
		foundProposals[p.Proposal.ID] = true
		// Verify run-2 proposals are skipped
		if p.RunID == "run-2" {
			t.Error("expected run-2 to be skipped because it has no proposals file")
		}
	}

	if !foundProposals["run-1-proposal-abc1"] {
		t.Error("expected to find run-1-proposal-abc1")
	}
	if !foundProposals["run-1-proposal-abc2"] {
		t.Error("expected to find run-1-proposal-abc2")
	}
}

func TestDiscoverPending_SortedByCreationTimeDescending(t *testing.T) {
	tmpDir := t.TempDir()

	// Create run 1 with earlier creation time
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
		},
		CreatedAt: time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC),
	}

	// Create run 2 with later creation time
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-2-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, nil)

	// Discover pending proposals
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pending) != 2 {
		t.Fatalf("expected 2 pending proposals, got %d", len(pending))
	}

	// Should be sorted descending by creation time (run-2 first, then run-1)
	if pending[0].RunID != "run-2" {
		t.Errorf("first pending proposal RunID mismatch, got %q, want %q", pending[0].RunID, "run-2")
	}
	if pending[1].RunID != "run-1" {
		t.Errorf("second pending proposal RunID mismatch, got %q, want %q", pending[1].RunID, "run-1")
	}
}

func TestDiscoverPending_FilterByProposalType(t *testing.T) {
	tmpDir := t.TempDir()

	proposals := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
			{
				ID:    "run-1-proposal-2",
				Type:  "validation_gap",
				Title: "Proposal 2",
			},
			{
				ID:    "run-1-proposal-3",
				Type:  "doctrine_rule",
				Title: "Proposal 3",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals, nil)

	// Discover with proposal type filter
	typeFilter := []string{"doctrine_rule"}
	pending, err := DiscoverPending(tmpDir, "test-project", &typeFilter, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pending) != 2 {
		t.Fatalf("expected 2 filtered proposals, got %d", len(pending))
	}

	for _, p := range pending {
		if p.Proposal.Type != "doctrine_rule" {
			t.Errorf("proposal type mismatch, got %q, want %q", p.Proposal.Type, "doctrine_rule")
		}
	}
}

func TestDiscoverPending_FilterByRunID(t *testing.T) {
	tmpDir := t.TempDir()

	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
		},
		CreatedAt: time.Now(),
	}

	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-2-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 2",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, nil)

	// Discover with run ID filter
	runIDFilter := []string{"run-1"}
	pending, err := DiscoverPending(tmpDir, "test-project", nil, &runIDFilter)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pending) != 1 {
		t.Fatalf("expected 1 filtered proposal, got %d", len(pending))
	}

	if pending[0].RunID != "run-1" {
		t.Errorf("pending proposal RunID mismatch, got %q, want %q", pending[0].RunID, "run-1")
	}
}

func TestDiscoverPending_NoDecisions_ReturnsAllPending(t *testing.T) {
	tmpDir := t.TempDir()

	// Run 1: has distillation-proposals.json with 2 proposals, NO proposal-decisions.json
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
			{
				ID:             "run-1-proposal-2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Run 2: has distillation-proposals.json with 2 proposals, NO proposal-decisions.json
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-2-proposal-1",
				Type:           "doctrine_rule",
				Title:          "Proposal 3",
				ProposedChange: "Change 3",
			},
			{
				ID:             "run-2-proposal-2",
				Type:           "validation_gap",
				Title:          "Proposal 4",
				ProposedChange: "Change 4",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}

	// Create runs without decisions (pass nil for decisions)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, nil)

	// Discover pending proposals
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	// Verify all 4 proposals are returned
	if len(pending) != 4 {
		t.Fatalf("expected 4 pending proposals, got %d", len(pending))
	}

	// Build a map to verify proposals
	foundProposals := make(map[string]*PendingProposal)
	for i := range pending {
		foundProposals[pending[i].Proposal.ID] = &pending[i]
	}

	// Verify all expected proposal IDs are present
	expectedIDs := []string{"run-1-proposal-1", "run-1-proposal-2", "run-2-proposal-1", "run-2-proposal-2"}
	for _, expectedID := range expectedIDs {
		if _, found := foundProposals[expectedID]; !found {
			t.Errorf("expected to find proposal %s", expectedID)
		}
	}

	// Verify RunID, SpecID, and CreatedAt are correctly populated
	if pp := foundProposals["run-1-proposal-1"]; pp != nil {
		if pp.RunID != "run-1" {
			t.Errorf("expected RunID 'run-1', got %s", pp.RunID)
		}
		if pp.SpecID != "spec-1" {
			t.Errorf("expected SpecID 'spec-1', got %s", pp.SpecID)
		}
		if pp.CreatedAt != proposals1.CreatedAt {
			t.Errorf("expected CreatedAt %v, got %v", proposals1.CreatedAt, pp.CreatedAt)
		}
	}

	if pp := foundProposals["run-2-proposal-1"]; pp != nil {
		if pp.RunID != "run-2" {
			t.Errorf("expected RunID 'run-2', got %s", pp.RunID)
		}
		if pp.SpecID != "spec-2" {
			t.Errorf("expected SpecID 'spec-2', got %s", pp.SpecID)
		}
		if pp.CreatedAt != proposals2.CreatedAt {
			t.Errorf("expected CreatedAt %v, got %v", proposals2.CreatedAt, pp.CreatedAt)
		}
	}
}

func TestDiscoverAll_ReturnsAllProposalsWithDecisionStatus(t *testing.T) {
	tmpDir := t.TempDir()

	proposals := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
			{
				ID:    "run-1-proposal-2",
				Type:  "validation_gap",
				Title: "Proposal 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	decisions := []Decision{
		{
			ProposalID: "run-1-proposal-1",
			Action:     "accepted",
			Reason:     "good",
		},
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals, decisions)

	// Discover all proposals
	all, err := DiscoverAll(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("expected 2 proposals, got %d", len(all))
	}

	// Check for decision status
	foundDecided := false
	foundPending := false
	for _, p := range all {
		if p.Proposal.ID == "run-1-proposal-1" && p.Decision != nil {
			foundDecided = true
		}
		if p.Proposal.ID == "run-1-proposal-2" && p.Decision == nil {
			foundPending = true
		}
	}

	if !foundDecided {
		t.Error("expected to find decided proposal with Decision set")
	}
	if !foundPending {
		t.Error("expected to find pending proposal with Decision nil")
	}
}

func TestDiscoverAll_IncludesPendingAndDecided(t *testing.T) {
	tmpDir := t.TempDir()

	// Run 1: some decided
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:  "run-1",
		SpecID: "spec-1",
		Proposals: []reviewdistiller.Proposal{
			{ID: "run-1-proposal-1", Type: "doctrine_rule"},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	decisions1 := []Decision{
		{ProposalID: "run-1-proposal-1", Action: "accepted"},
	}

	// Run 2: all pending
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:  "run-2",
		SpecID: "spec-2",
		Proposals: []reviewdistiller.Proposal{
			{ID: "run-2-proposal-1", Type: "doctrine_rule"},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, decisions1)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, nil)

	all, err := DiscoverAll(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("expected 2 proposals, got %d", len(all))
	}
}

func TestDiscoverAll_FiltersByProposalType(t *testing.T) {
	tmpDir := t.TempDir()

	proposals := &reviewdistiller.DistillationResult{
		RunID:  "run-1",
		SpecID: "spec-1",
		Proposals: []reviewdistiller.Proposal{
			{ID: "run-1-proposal-1", Type: "doctrine_rule"},
			{ID: "run-1-proposal-2", Type: "validation_gap"},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals, nil)

	typeFilter := []string{"doctrine_rule"}
	all, err := DiscoverAll(tmpDir, "test-project", &typeFilter, nil)

	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	if len(all) != 1 {
		t.Fatalf("expected 1 filtered proposal, got %d", len(all))
	}

	if all[0].Proposal.Type != "doctrine_rule" {
		t.Errorf("proposal type mismatch, got %q", all[0].Proposal.Type)
	}
}

func TestDiscoverPending_FindsProposals(t *testing.T) {
	tmpDir := t.TempDir()

	proposals := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
			{
				ID:             "run-1-proposal-2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals, nil)

	// Discover pending proposals
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pending) != 2 {
		t.Fatalf("expected 2 pending proposals, got %d", len(pending))
	}

	// Verify both proposals are found
	foundIDs := make(map[string]bool)
	for _, p := range pending {
		foundIDs[p.Proposal.ID] = true
	}

	if !foundIDs["run-1-proposal-1"] {
		t.Error("expected to find run-1-proposal-1")
	}
	if !foundIDs["run-1-proposal-2"] {
		t.Error("expected to find run-1-proposal-2")
	}
}

func TestDiscoverPending_FiltersOutDecidedProposals(t *testing.T) {
	tmpDir := t.TempDir()

	proposals := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
			{
				ID:             "run-1-proposal-2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
			{
				ID:             "run-1-proposal-3",
				Type:           "doctrine_rule",
				Title:          "Proposal 3",
				ProposedChange: "Change 3",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Mark two proposals as decided
	decisions := []Decision{
		{
			ProposalID: "run-1-proposal-1",
			Action:     "accepted",
			Reason:     "good",
		},
		{
			ProposalID: "run-1-proposal-3",
			Action:     "rejected",
			Reason:     "too risky",
		},
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals, decisions)

	// Discover pending proposals - should only return the undecided one
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pending) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pending))
	}

	if pending[0].Proposal.ID != "run-1-proposal-2" {
		t.Errorf("expected pending proposal ID %q, got %q", "run-1-proposal-2", pending[0].Proposal.ID)
	}
}

func TestDiscoverAll_IncludesDecided(t *testing.T) {
	tmpDir := t.TempDir()

	// Create run with mix of accepted, rejected, and pending proposals
	proposals := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
			{
				ID:    "run-1-proposal-2",
				Type:  "validation_gap",
				Title: "Proposal 2",
			},
			{
				ID:    "run-1-proposal-3",
				Type:  "doctrine_rule",
				Title: "Proposal 3",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	decisions := []Decision{
		{
			ProposalID: "run-1-proposal-1",
			Action:     "accepted",
			Reason:     "good idea",
		},
		{
			ProposalID: "run-1-proposal-3",
			Action:     "rejected",
			Reason:     "too risky",
		},
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals, decisions)

	// Discover all proposals (both decided and pending)
	all, err := DiscoverAll(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	if len(all) != 3 {
		t.Fatalf("expected 3 proposals, got %d", len(all))
	}

	// Check that decisions are populated correctly
	proposalMap := make(map[string]*AllProposal)
	for i := range all {
		proposalMap[all[i].Proposal.ID] = &all[i]
	}

	// run-1-proposal-1 should have decision: accepted
	if p1, ok := proposalMap["run-1-proposal-1"]; !ok {
		t.Error("expected to find run-1-proposal-1")
	} else if p1.Decision == nil {
		t.Error("expected run-1-proposal-1 to have a decision")
	} else if p1.Decision.Action != "accepted" {
		t.Errorf("expected run-1-proposal-1 decision action %q, got %q", "accepted", p1.Decision.Action)
	}

	// run-1-proposal-2 should have no decision (pending)
	if p2, ok := proposalMap["run-1-proposal-2"]; !ok {
		t.Error("expected to find run-1-proposal-2")
	} else if p2.Decision != nil {
		t.Error("expected run-1-proposal-2 to have no decision (pending)")
	}

	// run-1-proposal-3 should have decision: rejected
	if p3, ok := proposalMap["run-1-proposal-3"]; !ok {
		t.Error("expected to find run-1-proposal-3")
	} else if p3.Decision == nil {
		t.Error("expected run-1-proposal-3 to have a decision")
	} else if p3.Decision.Action != "rejected" {
		t.Errorf("expected run-1-proposal-3 decision action %q, got %q", "rejected", p3.Decision.Action)
	}
}

func TestDiscover_FilterByType(t *testing.T) {
	tmpDir := t.TempDir()

	// Create run with multiple proposal types
	proposals := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
			{
				ID:    "run-1-proposal-2",
				Type:  "validation_gap",
				Title: "Proposal 2",
			},
			{
				ID:    "run-1-proposal-3",
				Type:  "doctrine_rule",
				Title: "Proposal 3",
			},
			{
				ID:    "run-1-proposal-4",
				Type:  "validation_gap",
				Title: "Proposal 4",
			},
		},
		CreatedAt: time.Now(),
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals, nil)

	// Filter by doctrine_rule type
	typeFilter := []string{"doctrine_rule"}
	all, err := DiscoverAll(tmpDir, "test-project", &typeFilter, nil)

	if err != nil {
		t.Fatalf("DiscoverAll with type filter failed: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("expected 2 filtered proposals, got %d", len(all))
	}

	// Verify all returned proposals are of the requested type
	for _, p := range all {
		if p.Proposal.Type != "doctrine_rule" {
			t.Errorf("expected proposal type %q, got %q", "doctrine_rule", p.Proposal.Type)
		}
	}

	// Test filtering by validation_gap type
	typeFilter2 := []string{"validation_gap"}
	all2, err := DiscoverAll(tmpDir, "test-project", &typeFilter2, nil)

	if err != nil {
		t.Fatalf("DiscoverAll with validation_gap filter failed: %v", err)
	}

	if len(all2) != 2 {
		t.Fatalf("expected 2 filtered proposals, got %d", len(all2))
	}

	for _, p := range all2 {
		if p.Proposal.Type != "validation_gap" {
			t.Errorf("expected proposal type %q, got %q", "validation_gap", p.Proposal.Type)
		}
	}
}

func TestDiscover_FilterByRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create run 1 with proposals
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
			{
				ID:    "run-1-proposal-2",
				Type:  "validation_gap",
				Title: "Proposal 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Create run 2 with different proposals
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-2-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 3",
			},
			{
				ID:    "run-2-proposal-2",
				Type:  "doctrine_rule",
				Title: "Proposal 4",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}

	// Create run 3 with proposals
	proposals3 := &reviewdistiller.DistillationResult{
		RunID:     "run-3",
		SpecID:    "spec-3",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-3-proposal-1",
				Type:  "validation_gap",
				Title: "Proposal 5",
			},
		},
		CreatedAt: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, nil)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-3", proposals3, nil)

	// Filter by run-1
	runFilter := []string{"run-1"}
	all, err := DiscoverAll(tmpDir, "test-project", nil, &runFilter)

	if err != nil {
		t.Fatalf("DiscoverAll with run filter failed: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("expected 2 proposals from run-1, got %d", len(all))
	}

	for _, p := range all {
		if p.RunID != "run-1" {
			t.Errorf("expected proposal from run-1, got from %q", p.RunID)
		}
	}

	// Filter by multiple runs
	runFilter2 := []string{"run-1", "run-3"}
	all2, err := DiscoverAll(tmpDir, "test-project", nil, &runFilter2)

	if err != nil {
		t.Fatalf("DiscoverAll with multiple run filter failed: %v", err)
	}

	if len(all2) != 3 {
		t.Fatalf("expected 3 proposals from run-1 and run-3, got %d", len(all2))
	}

	foundRuns := make(map[string]bool)
	for _, p := range all2 {
		foundRuns[p.RunID] = true
		if p.RunID != "run-1" && p.RunID != "run-3" {
			t.Errorf("expected proposal from run-1 or run-3, got from %q", p.RunID)
		}
	}

	if !foundRuns["run-1"] {
		t.Error("expected to find proposals from run-1")
	}
	if !foundRuns["run-3"] {
		t.Error("expected to find proposals from run-3")
	}
}

func TestDiscoverAll_ReturnsAllWithStatus(t *testing.T) {
	tmpDir := t.TempDir()

	// Run 1: mix of decided and undecided proposals
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
			{
				ID:    "run-1-proposal-2",
				Type:  "validation_gap",
				Title: "Proposal 2",
			},
			{
				ID:    "run-1-proposal-3",
				Type:  "doctrine_rule",
				Title: "Proposal 3",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Run 2: all undecided proposals
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-2-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 4",
			},
			{
				ID:    "run-2-proposal-2",
				Type:  "validation_gap",
				Title: "Proposal 5",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}

	// Decisions for run 1: accept first, reject third, leave second undecided
	decisions1 := []Decision{
		{
			ProposalID: "run-1-proposal-1",
			Action:     "accepted",
			Reason:     "good change",
		},
		{
			ProposalID: "run-1-proposal-3",
			Action:     "rejected",
			Reason:     "too risky",
		},
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, decisions1)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, nil)

	// Discover all proposals
	all, err := DiscoverAll(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	if len(all) != 5 {
		t.Fatalf("expected 5 proposals total, got %d", len(all))
	}

	// Build map for easy lookup
	proposalMap := make(map[string]*AllProposal)
	for i := range all {
		proposalMap[all[i].Proposal.ID] = &all[i]
	}

	// Verify run-1-proposal-1 is decided with accepted action
	if p, ok := proposalMap["run-1-proposal-1"]; !ok {
		t.Error("expected to find run-1-proposal-1")
	} else {
		if p.Decision == nil {
			t.Error("expected run-1-proposal-1 to have a Decision")
		} else if p.Decision.Action != "accepted" {
			t.Errorf("expected run-1-proposal-1 decision action %q, got %q", "accepted", p.Decision.Action)
		}
	}

	// Verify run-1-proposal-2 is undecided (Decision nil)
	if p, ok := proposalMap["run-1-proposal-2"]; !ok {
		t.Error("expected to find run-1-proposal-2")
	} else {
		if p.Decision != nil {
			t.Errorf("expected run-1-proposal-2 to have nil Decision, got %+v", p.Decision)
		}
	}

	// Verify run-1-proposal-3 is decided with rejected action
	if p, ok := proposalMap["run-1-proposal-3"]; !ok {
		t.Error("expected to find run-1-proposal-3")
	} else {
		if p.Decision == nil {
			t.Error("expected run-1-proposal-3 to have a Decision")
		} else if p.Decision.Action != "rejected" {
			t.Errorf("expected run-1-proposal-3 decision action %q, got %q", "rejected", p.Decision.Action)
		}
	}

	// Verify run-2 proposals are all undecided
	if p, ok := proposalMap["run-2-proposal-1"]; !ok {
		t.Error("expected to find run-2-proposal-1")
	} else {
		if p.Decision != nil {
			t.Errorf("expected run-2-proposal-1 to have nil Decision, got %+v", p.Decision)
		}
	}

	if p, ok := proposalMap["run-2-proposal-2"]; !ok {
		t.Error("expected to find run-2-proposal-2")
	} else {
		if p.Decision != nil {
			t.Errorf("expected run-2-proposal-2 to have nil Decision, got %+v", p.Decision)
		}
	}
}

func TestDiscoverPending_EmptyStore(t *testing.T) {
	tmpDir := t.TempDir()

	// Call DiscoverPending on a completely empty store (no runs directory)
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending should not error with empty store, got: %v", err)
	}
	if pending == nil {
		t.Fatal("DiscoverPending should return empty slice, not nil")
	}
	if len(pending) != 0 {
		t.Fatalf("DiscoverPending should return empty slice, got %d pending proposals", len(pending))
	}
}

func TestDiscoverPending_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create run-1 with valid proposals
	validProposals := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", validProposals, nil)

	// Create run-2 with malformed JSON file
	store := runstore.NewStore(tmpDir)
	rs2 := &runstore.RunState{
		RunID:     "run-2",
		ProjectID: "test-project",
		SpecID:    "test-spec",
		Status:    "completed",
		StartedAt: time.Now(),
	}
	rs2.NormalizeNilFields()
	if err := store.Save(rs2); err != nil {
		t.Fatalf("failed to save run state: %v", err)
	}

	evidenceDir := store.RunEvidenceDir("run-2")
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		t.Fatalf("failed to create evidence dir: %v", err)
	}

	// Write malformed JSON to distillation-proposals.json
	malformedJSON := `{
		"runID": "run-2",
		"proposals": [
			{ "id": "malformed", "type": "test"
		]
	}` // Missing closing brace for array
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), []byte(malformedJSON), 0644); err != nil {
		t.Fatalf("failed to write malformed JSON: %v", err)
	}

	// Discover pending proposals - should skip malformed run-2 and return only run-1
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending should handle malformed JSON gracefully, got error: %v", err)
	}

	if len(pending) != 1 {
		t.Fatalf("expected 1 proposal (from run-1), got %d pending proposals", len(pending))
	}

	if pending[0].RunID != "run-1" {
		t.Errorf("expected proposal from run-1, got from %q", pending[0].RunID)
	}

	if pending[0].Proposal.ID != "run-1-proposal-1" {
		t.Errorf("expected proposal run-1-proposal-1, got %q", pending[0].Proposal.ID)
	}
}

func TestDiscoverPending_WithDecisions_FiltersPending(t *testing.T) {
	tmpDir := t.TempDir()

	// Create run with 3 proposals
	proposals := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-1",
				Type:           "doctrine_rule",
				Title:          "Proposal 1",
				ProposedChange: "Change 1",
			},
			{
				ID:             "run-1-proposal-2",
				Type:           "validation_gap",
				Title:          "Proposal 2",
				ProposedChange: "Change 2",
			},
			{
				ID:             "run-1-proposal-3",
				Type:           "doctrine_rule",
				Title:          "Proposal 3",
				ProposedChange: "Change 3",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Create decisions for 2 of them
	decisions := []Decision{
		{
			ProposalID: "run-1-proposal-1",
			Action:     "accepted",
		},
		{
			ProposalID: "run-1-proposal-2",
			Action:     "rejected",
		},
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals, decisions)

	// Discover pending proposals
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	// Should return only 1 pending proposal (the one without a decision)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pending))
	}

	// Verify it's the undecided proposal
	if pending[0].Proposal.ID != "run-1-proposal-3" {
		t.Errorf("expected pending proposal to be run-1-proposal-3, got %s", pending[0].Proposal.ID)
	}

	// Verify RunID and SpecID
	if pending[0].RunID != "run-1" {
		t.Errorf("expected RunID run-1, got %s", pending[0].RunID)
	}
	if pending[0].SpecID != "spec-1" {
		t.Errorf("expected SpecID spec-1, got %s", pending[0].SpecID)
	}
}

func TestDiscoverPending_SortedByCreatedAtDescending(t *testing.T) {
	tmpDir := t.TempDir()

	// Create run 1 with earliest creation time
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-1-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 1",
			},
		},
		CreatedAt: time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC),
	}

	// Create run 2 with middle creation time
	proposals2 := &reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-2",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-2-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 2",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Create run 3 with latest creation time
	proposals3 := &reviewdistiller.DistillationResult{
		RunID:     "run-3",
		SpecID:    "spec-3",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:    "run-3-proposal-1",
				Type:  "doctrine_rule",
				Title: "Proposal 3",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}

	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-2", proposals2, nil)
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-3", proposals3, nil)

	// Discover pending proposals
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pending) != 3 {
		t.Fatalf("expected 3 pending proposals, got %d", len(pending))
	}

	// Should be sorted descending by creation time (run-3 first, then run-2, then run-1)
	if pending[0].RunID != "run-3" {
		t.Errorf("first pending proposal RunID mismatch, got %q, want %q", pending[0].RunID, "run-3")
	}
	if pending[1].RunID != "run-2" {
		t.Errorf("second pending proposal RunID mismatch, got %q, want %q", pending[1].RunID, "run-2")
	}
	if pending[2].RunID != "run-1" {
		t.Errorf("third pending proposal RunID mismatch, got %q, want %q", pending[2].RunID, "run-1")
	}
}

func TestDiscoverPending_MalformedJSON_Skipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Run 1: has VALID proposals
	proposals1 := &reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-1",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-1",
				Type:           "doctrine_rule",
				Title:          "Valid Proposal",
				ProposedChange: "Valid Change",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Run 1: create with valid proposals
	helperCreateRunWithProposals(t, tmpDir, "test-project", "run-1", proposals1, nil)

	// Run 2: manually create with MALFORMED JSON in distillation-proposals.json
	store := runstore.NewStore(tmpDir)
	rs := &runstore.RunState{
		RunID:     "run-2",
		ProjectID: "test-project",
		SpecID:    "spec-2",
		Status:    "completed",
		StartedAt: time.Now(),
	}
	rs.NormalizeNilFields()
	if err := store.Save(rs); err != nil {
		t.Fatalf("failed to save run state: %v", err)
	}

	// Write malformed JSON to distillation-proposals.json
	evidenceDir := store.RunEvidenceDir("run-2")
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		t.Fatalf("failed to create evidence dir: %v", err)
	}
	malformedJSON := `{"RunID": "run-2", "Proposals": [{"ID": "incomplete-proposal"` // incomplete JSON
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), []byte(malformedJSON), 0644); err != nil {
		t.Fatalf("failed to write malformed proposals: %v", err)
	}

	// Discover pending proposals
	pending, err := DiscoverPending(tmpDir, "test-project", nil, nil)

	// Should not error even though run-2 has malformed JSON
	if err != nil {
		t.Fatalf("DiscoverPending should not error with malformed JSON, got: %v", err)
	}

	// Should only return proposals from run-1 (run-2 should be skipped)
	if pending == nil {
		t.Fatal("DiscoverPending should return empty slice, not nil")
	}

	if len(pending) != 1 {
		t.Fatalf("expected 1 pending proposal from run-1, got %d", len(pending))
	}

	// Verify the proposal is from run-1
	if pending[0].RunID != "run-1" {
		t.Errorf("expected proposal from run-1, got from %s", pending[0].RunID)
	}
	if pending[0].Proposal.ID != "run-1-proposal-1" {
		t.Errorf("expected proposal run-1-proposal-1, got %s", pending[0].Proposal.ID)
	}
}
