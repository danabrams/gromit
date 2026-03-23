package proposaltriage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestDismissSiblings_DismissesAllExceptAccepted(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)
	runID := "run-123"

	// Create a group with 3 proposals
	group := ProposalGroup{
		GroupID: "group-1",
		Proposals: []PendingProposal{
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-1",
					Title: "Proposal 1",
				},
				RunID:     runID,
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-2",
					Title: "Proposal 2",
				},
				RunID:     runID,
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-3",
					Title: "Proposal 3",
				},
				RunID:     runID,
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
		},
		GroupReason: "same file",
	}

	// Accept prop-1, dismiss prop-2 and prop-3
	acceptedProposalID := "prop-1"
	decisions, err := DismissSiblings(acceptedProposalID, group, store)

	if err != nil {
		t.Fatalf("DismissSiblings failed: %v", err)
	}

	// Should return 2 decisions (for prop-2 and prop-3)
	if len(decisions) != 2 {
		t.Fatalf("expected 2 dismissed decisions, got %d", len(decisions))
	}

	// Verify decisions were created correctly
	for i, decision := range decisions {
		if decision.Action != "dismissed" {
			t.Errorf("decision %d: Action should be 'dismissed', got %q", i, decision.Action)
		}
		if decision.DismissedBy != acceptedProposalID {
			t.Errorf("decision %d: DismissedBy should be %q, got %q", i, acceptedProposalID, decision.DismissedBy)
		}
		if decision.DecidedAt.IsZero() {
			t.Errorf("decision %d: DecidedAt should not be zero", i)
		}
		// ProposalID should be one of the siblings
		if decision.ProposalID != "prop-2" && decision.ProposalID != "prop-3" {
			t.Errorf("decision %d: ProposalID should be prop-2 or prop-3, got %q", i, decision.ProposalID)
		}
	}

	// Verify decisions were saved to the correct evidence directories
	for _, decision := range decisions {
		evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
		loaded, err := LoadDecisions(evidenceDir)
		if err != nil {
			t.Fatalf("LoadDecisions failed: %v", err)
		}

		// Find the decision for this proposal
		found := false
		for _, d := range loaded {
			if d.ProposalID == decision.ProposalID {
				found = true
				if d.Action != "dismissed" {
					t.Errorf("saved decision for %s: Action should be 'dismissed', got %q", decision.ProposalID, d.Action)
				}
				if d.DismissedBy != acceptedProposalID {
					t.Errorf("saved decision for %s: DismissedBy should be %q, got %q", decision.ProposalID, acceptedProposalID, d.DismissedBy)
				}
				break
			}
		}
		if !found {
			t.Errorf("decision for %q was not saved to evidence directory", decision.ProposalID)
		}
	}
}

func TestDismissSiblings_EmptyGroupReturnsEmptySlice(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)

	// Single proposal group (no siblings)
	group := ProposalGroup{
		GroupID: "group-1",
		Proposals: []PendingProposal{
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-1",
					Title: "Proposal 1",
				},
				RunID:     "run-123",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
		},
		GroupReason: "single proposal",
	}

	acceptedProposalID := "prop-1"
	decisions, err := DismissSiblings(acceptedProposalID, group, store)

	if err != nil {
		t.Fatalf("DismissSiblings failed: %v", err)
	}

	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions for single-proposal group, got %d", len(decisions))
	}
}

func TestDismissSiblings_MultipleRuns(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)

	// Create a group with proposals from different runs
	group := ProposalGroup{
		GroupID: "group-1",
		Proposals: []PendingProposal{
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-1",
					Title: "Proposal 1",
				},
				RunID:     "run-1",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-2",
					Title: "Proposal 2",
				},
				RunID:     "run-2",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
		},
		GroupReason: "same file, different runs",
	}

	acceptedProposalID := "prop-1"
	decisions, err := DismissSiblings(acceptedProposalID, group, store)

	if err != nil {
		t.Fatalf("DismissSiblings failed: %v", err)
	}

	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}

	// Verify decision was saved to the correct evidence directory for run-2
	evidenceDir := filepath.Join(tmpDir, "runs", "run-2", "evidence")
	loaded, err := LoadDecisions(evidenceDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 decision in run-2 evidence, got %d", len(loaded))
	}

	if loaded[0].ProposalID != "prop-2" {
		t.Errorf("expected decision for prop-2, got %q", loaded[0].ProposalID)
	}

	// Verify run-1 evidence directory does not have any decisions
	evidenceDirRun1 := filepath.Join(tmpDir, "runs", "run-1", "evidence")
	_, err = os.Stat(evidenceDirRun1)
	if err == nil {
		// Directory exists, check it's empty
		loaded, err := LoadDecisions(evidenceDirRun1)
		if err != nil {
			t.Fatalf("LoadDecisions failed: %v", err)
		}
		if len(loaded) != 0 {
			t.Errorf("run-1 evidence should be empty, got %d decisions", len(loaded))
		}
	}
}

func TestDismissSiblings_SkipsNilProposal(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)

	// Create a group with one nil Proposal and one valid Proposal
	group := ProposalGroup{
		GroupID: "group-1",
		Proposals: []PendingProposal{
			{
				Proposal: nil, // nil Proposal should be skipped
				RunID:    "run-1",
				SpecID:   "spec-1",
				GroupID:  "group-1",
			},
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-2",
					Title: "Proposal 2",
				},
				RunID:     "run-2",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
		},
		GroupReason: "one nil proposal",
	}

	acceptedProposalID := "prop-2"
	decisions, err := DismissSiblings(acceptedProposalID, group, store)

	if err != nil {
		t.Fatalf("DismissSiblings failed: %v", err)
	}

	// Should return 0 decisions (nil proposal is skipped, prop-2 is the accepted one)
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestDismissSiblings_AcceptedProposalNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)

	// Create a group with two proposals
	group := ProposalGroup{
		GroupID: "group-1",
		Proposals: []PendingProposal{
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-1",
					Title: "Proposal 1",
				},
				RunID:     "run-1",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-2",
					Title: "Proposal 2",
				},
				RunID:     "run-2",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
		},
		GroupReason: "test accepted not found",
	}

	// Try to accept a proposal ID that doesn't exist in the group
	acceptedProposalID := "nonexistent-id"
	decisions, err := DismissSiblings(acceptedProposalID, group, store)

	// Should return an error and no decisions
	if err == nil {
		t.Fatalf("DismissSiblings should return an error when acceptedProposalID not found")
	}

	if decisions != nil && len(decisions) > 0 {
		t.Errorf("expected no decisions when acceptedProposalID not found, got %d", len(decisions))
	}
}

func TestDismissSiblings_PartialSaveFailure(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)

	// Create a group with proposals from two different runs
	group := ProposalGroup{
		GroupID: "group-1",
		Proposals: []PendingProposal{
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-1",
					Title: "Proposal 1",
				},
				RunID:     "run-success",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-2",
					Title: "Proposal 2",
				},
				RunID:     "run-fail",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-3",
					Title: "Proposal 3",
				},
				RunID:     "run-success",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
		},
		GroupReason: "partial save failure test",
	}

	// Make the run-fail evidence directory read-only before calling DismissSiblings
	// This will force SaveDecisions to fail for run-fail but succeed for run-success
	failEvidenceDir := filepath.Join(tmpDir, "runs", "run-fail", "evidence")
	if err := os.MkdirAll(failEvidenceDir, 0o755); err != nil {
		t.Fatalf("failed to create fail evidence dir: %v", err)
	}
	if err := os.Chmod(failEvidenceDir, 0o444); err != nil { // read-only
		t.Fatalf("failed to chmod fail evidence dir: %v", err)
	}

	// Cleanup: restore permissions after test
	t.Cleanup(func() {
		os.Chmod(failEvidenceDir, 0o755)
	})

	acceptedProposalID := "prop-1"
	decisions, err := DismissSiblings(acceptedProposalID, group, store)

	// Should return 2 decisions (for prop-2 and prop-3), even though one save failed
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions to be created, got %d", len(decisions))
	}

	// Should return a non-nil error due to the failed save
	if err == nil {
		t.Fatalf("expected a combined error from failed save, got nil")
	}

	// Verify that decisions for run-success were actually saved
	successEvidenceDir := filepath.Join(tmpDir, "runs", "run-success", "evidence")
	loaded, err := LoadDecisions(successEvidenceDir)
	if err != nil {
		t.Fatalf("LoadDecisions for run-success failed: %v", err)
	}

	// Should have 1 decision (prop-3; prop-1 is the accepted one)
	if len(loaded) != 1 {
		t.Fatalf("expected 1 decision in run-success evidence, got %d", len(loaded))
	}

	if loaded[0].ProposalID != "prop-3" {
		t.Errorf("expected decision for prop-3, got %q", loaded[0].ProposalID)
	}

	if loaded[0].Action != "dismissed" {
		t.Errorf("expected dismissed action, got %q", loaded[0].Action)
	}

	if loaded[0].DismissedBy != acceptedProposalID {
		t.Errorf("expected DismissedBy=%q, got %q", acceptedProposalID, loaded[0].DismissedBy)
	}
}

func TestDismissSiblings_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)

	// Create a group with 3 proposals
	group := ProposalGroup{
		GroupID: "group-1",
		Proposals: []PendingProposal{
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-1",
					Title: "Proposal 1",
				},
				RunID:     "run-123",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-2",
					Title: "Proposal 2",
				},
				RunID:     "run-123",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
			{
				Proposal: &reviewdistiller.Proposal{
					ID:    "prop-3",
					Title: "Proposal 3",
				},
				RunID:     "run-123",
				SpecID:    "spec-1",
				CreatedAt: time.Now(),
				GroupID:   "group-1",
			},
		},
		GroupReason: "idempotency test",
	}

	acceptedProposalID := "prop-1"

	// First call to DismissSiblings
	decisions1, err := DismissSiblings(acceptedProposalID, group, store)
	if err != nil {
		t.Fatalf("first DismissSiblings call failed: %v", err)
	}

	if len(decisions1) != 2 {
		t.Fatalf("first call: expected 2 decisions, got %d", len(decisions1))
	}

	// Load decisions from evidence directory after first call
	evidenceDir := filepath.Join(tmpDir, "runs", "run-123", "evidence")
	loadedAfterFirst, err := LoadDecisions(evidenceDir)
	if err != nil {
		t.Fatalf("LoadDecisions after first call failed: %v", err)
	}

	if len(loadedAfterFirst) != 2 {
		t.Fatalf("first call: expected 2 decisions in evidence, got %d", len(loadedAfterFirst))
	}

	// Retry: call DismissSiblings again with the same parameters
	// This simulates a retry after partial failure
	decisions2, err := DismissSiblings(acceptedProposalID, group, store)
	if err != nil {
		t.Fatalf("second DismissSiblings call failed: %v", err)
	}

	if len(decisions2) != 2 {
		t.Fatalf("second call: expected 2 decisions, got %d", len(decisions2))
	}

	// Load decisions from evidence directory after second call
	loadedAfterSecond, err := LoadDecisions(evidenceDir)
	if err != nil {
		t.Fatalf("LoadDecisions after second call failed: %v", err)
	}

	// Critical: verify that decisions were NOT duplicated.
	// SaveDecisions uses load-merge-save with deduplication by ProposalID,
	// so retrying should result in the same number of decisions.
	if len(loadedAfterSecond) != 2 {
		t.Fatalf("second call: expected 2 decisions in evidence (not duplicated), got %d", len(loadedAfterSecond))
	}

	// Verify the decisions are the same (by ProposalID)
	decisionMap := make(map[string]bool)
	for _, d := range loadedAfterSecond {
		decisionMap[d.ProposalID] = true
	}

	if !decisionMap["prop-2"] || !decisionMap["prop-3"] {
		t.Errorf("expected decisions for prop-2 and prop-3, got proposal IDs: %v", decisionMap)
	}
}
