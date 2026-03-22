package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/proposaltriage"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ListAllShowsDecidedProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"

	// Create projects directory so loadProjectID works
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Run 1: 2 proposals, 1 accepted and 1 rejected
	run1 := &runstore.RunState{
		RunID:     "run-1",
		ProjectID: projectID,
		SpecID:    "spec-alpha",
		Status:    "completed",
		StartedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	run1.NormalizeNilFields()
	if err := store.Save(run1); err != nil {
		t.Fatalf("save run-1: %v", err)
	}

	evidenceDir1 := store.RunEvidenceDir("run-1")
	if err := os.MkdirAll(evidenceDir1, 0o755); err != nil {
		t.Fatalf("mkdir evidence-1: %v", err)
	}

	proposals1 := reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-alpha",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-1",
				Type:           "doctrine_rule",
				Title:          "Enforce error handling",
				ProposedChange: "Always check errors",
				Confidence:     "high",
			},
			{
				ID:             "run-1-proposal-2",
				Type:           "planner_heuristic",
				Title:          "Split large tasks",
				ProposedChange: "Decompose tasks over 4 files",
				Confidence:     "medium",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 30, 0, 0, time.UTC),
	}
	p1Data, err := json.MarshalIndent(proposals1, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir1, "distillation-proposals.json"), p1Data, 0o644); err != nil {
		t.Fatalf("write proposals1: %v", err)
	}

	// Decisions for run-1: proposal-1 accepted, proposal-2 rejected
	decisions1 := []proposaltriage.Decision{
		{
			ProposalID: "run-1-proposal-1",
			Action:     "accepted",
			Reason:     "good rule",
			DecidedAt:  time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		},
		{
			ProposalID: "run-1-proposal-2",
			Action:     "rejected",
			Reason:     "too aggressive",
			DecidedAt:  time.Date(2026, 3, 20, 11, 5, 0, 0, time.UTC),
		},
	}
	if err := proposaltriage.SaveDecisions(evidenceDir1, decisions1); err != nil {
		t.Fatalf("save decisions1: %v", err)
	}

	// Run 2: 2 proposals, both pending (no decisions)
	run2 := &runstore.RunState{
		RunID:     "run-2",
		ProjectID: projectID,
		SpecID:    "spec-beta",
		Status:    "completed",
		StartedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}
	run2.NormalizeNilFields()
	if err := store.Save(run2); err != nil {
		t.Fatalf("save run-2: %v", err)
	}

	evidenceDir2 := store.RunEvidenceDir("run-2")
	if err := os.MkdirAll(evidenceDir2, 0o755); err != nil {
		t.Fatalf("mkdir evidence-2: %v", err)
	}

	proposals2 := reviewdistiller.DistillationResult{
		RunID:     "run-2",
		SpecID:    "spec-beta",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-2-proposal-1",
				Type:           "validation_gap",
				Title:          "Add integration tests",
				ProposedChange: "Cover edge cases",
				Confidence:     "high",
			},
			{
				ID:             "run-2-proposal-2",
				Type:           "doctrine_rule",
				Title:          "Require code review",
				ProposedChange: "All changes need review",
				Confidence:     "low",
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 30, 0, 0, time.UTC),
	}
	p2Data, err := json.MarshalIndent(proposals2, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir2, "distillation-proposals.json"), p2Data, 0o644); err != nil {
		t.Fatalf("write proposals2: %v", err)
	}

	// === Invoke ===
	// First verify the data layer returns all 4 proposals
	allProposals, err := proposaltriage.DiscoverAll(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}
	if len(allProposals) != 4 {
		t.Fatalf("expected 4 proposals from DiscoverAll, got %d", len(allProposals))
	}

	// Now invoke displayAllProposals and capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	if err := displayAllProposals(allProposals); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("displayAllProposals: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout
	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	output := string(captured)

	// === Assert ===

	// Table header present
	if !strings.Contains(output, "STATUS") {
		t.Error("expected STATUS column header in output")
	}
	if !strings.Contains(output, "ID") {
		t.Error("expected ID column header in output")
	}
	if !strings.Contains(output, "TYPE") {
		t.Error("expected TYPE column header in output")
	}

	// All 4 proposal titles appear
	if !strings.Contains(output, "Enforce error handling") {
		t.Error("expected accepted proposal title 'Enforce error handling'")
	}
	if !strings.Contains(output, "Split large tasks") {
		t.Error("expected rejected proposal title 'Split large tasks'")
	}
	if !strings.Contains(output, "Add integration tests") {
		t.Error("expected pending proposal title 'Add integration tests'")
	}
	if !strings.Contains(output, "Require code review") {
		t.Error("expected pending proposal title 'Require code review'")
	}

	// All 3 statuses appear: accepted, rejected, pending
	if !strings.Contains(output, "accepted") {
		t.Error("expected 'accepted' status in output")
	}
	if !strings.Contains(output, "rejected") {
		t.Error("expected 'rejected' status in output")
	}
	if !strings.Contains(output, "pending") {
		t.Error("expected 'pending' status in output")
	}

	// Both run IDs appear
	if !strings.Contains(output, "run-1") {
		t.Error("expected run-1 in output")
	}
	if !strings.Contains(output, "run-2") {
		t.Error("expected run-2 in output")
	}

	// Verify the data layer has correct decision mapping
	proposalMap := make(map[string]*proposaltriage.AllProposal)
	for i := range allProposals {
		proposalMap[allProposals[i].Proposal.ID] = &allProposals[i]
	}

	// run-1-proposal-1 should be accepted
	if p, ok := proposalMap["run-1-proposal-1"]; !ok {
		t.Error("expected run-1-proposal-1 in results")
	} else if p.Decision == nil || p.Decision.Action != "accepted" {
		t.Error("expected run-1-proposal-1 to have accepted decision")
	}

	// run-1-proposal-2 should be rejected
	if p, ok := proposalMap["run-1-proposal-2"]; !ok {
		t.Error("expected run-1-proposal-2 in results")
	} else if p.Decision == nil || p.Decision.Action != "rejected" {
		t.Error("expected run-1-proposal-2 to have rejected decision")
	}

	// run-2 proposals should be pending (no decision)
	if p, ok := proposalMap["run-2-proposal-1"]; !ok {
		t.Error("expected run-2-proposal-1 in results")
	} else if p.Decision != nil {
		t.Errorf("expected run-2-proposal-1 to be pending, got decision: %s", p.Decision.Action)
	}

	if p, ok := proposalMap["run-2-proposal-2"]; !ok {
		t.Error("expected run-2-proposal-2 in results")
	} else if p.Decision != nil {
		t.Errorf("expected run-2-proposal-2 to be pending, got decision: %s", p.Decision.Action)
	}
}
