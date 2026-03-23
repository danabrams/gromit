package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/proposaltriage"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_AcceptProposalIntoGlobalPlaybook(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "fixture-app"

	runID := "run-301"
	proposalID := "run-301-proposal-e5f6a7b8"
	proposalContent := "Prefer package-scoped compile checks before full test suite"

	// Create run
	run := &runstore.RunState{
		RunID:                 runID,
		SpecID:                "spec-compile-checks",
		ProjectID:             projectID,
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 21, 10, 15, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "sonnet"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed distillation-proposals.json with a planner_heuristic proposal
	evidenceDir := store.RunEvidenceDir(runID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-compile-checks",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 10, 20, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  proposalID,
				Type:                "planner_heuristic",
				Title:               "Compile before test",
				WhatHappened:        "Full test suite ran before compile checks",
				WhatWasMissing:      "Package-scoped compile checks",
				ProposedChange:      proposalContent,
				Rationale:           "Faster feedback on syntax errors",
				Confidence:          "high",
				ConfidenceRationale: "Common best practice",
				EvidenceReferences:  []string{"review-outcome.json"},
			},
		},
	}

	proposalsJSON, err := json.MarshalIndent(distResult, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), proposalsJSON, 0o644); err != nil {
		t.Fatalf("write proposals: %v", err)
	}

	// === Invoke ===
	// Discover the proposal
	allProposals, err := proposaltriage.DiscoverAll(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}
	if len(allProposals) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(allProposals))
	}

	targetProposal := allProposals[0]
	if targetProposal.Proposal.ID != proposalID {
		t.Fatalf("expected proposal ID %q, got %q", proposalID, targetProposal.Proposal.ID)
	}

	// Set up global stores
	globalDoctrineDir := filepath.Join(tmp, "global", "doctrine")
	globalPlaybookDir := filepath.Join(tmp, "global", "playbook")

	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = globalDoctrineDir
	playbookStore := &playbook.Store{Dir: globalPlaybookDir}

	// Create PendingProposal with scope=global (mirrors what the CLI does)
	pp := &proposaltriage.PendingProposal{
		Proposal: targetProposal.Proposal,
		RunID:    targetProposal.RunID,
		SpecID:   targetProposal.SpecID,
		Scope:    "global",
	}

	// Accept the proposal into the global store
	decision, err := proposaltriage.Promote(
		pp,
		"", "", "", // no field overrides
		doctrineStore,
		playbookStore,
		pp.Scope,
	)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Save decision to evidence directory
	if err := proposaltriage.SaveDecisions(evidenceDir, []proposaltriage.Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions: %v", err)
	}

	// === Assert ===

	// 1. A new entry exists in global playbook entries.json
	entries, err := playbookStore.Load()
	if err != nil {
		t.Fatalf("load global playbook: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in global playbook, got %d", len(entries))
	}

	entry := entries[0]

	// 2. Entry type is planner_heuristic
	if entry.Type != "planner_heuristic" {
		t.Errorf("entry type = %q, want 'planner_heuristic'", entry.Type)
	}

	// 3. Entry scope is global
	if entry.Scope != "global" {
		t.Errorf("entry scope = %q, want 'global'", entry.Scope)
	}

	// 4. Entry content matches the proposal's proposed change
	if entry.Content != proposalContent {
		t.Errorf("entry content = %q, want %q", entry.Content, proposalContent)
	}

	// 5. Entry status is active
	if entry.Status != "active" {
		t.Errorf("entry status = %q, want 'active'", entry.Status)
	}

	// 6. Provenance traces back to run-301
	if entry.SourceRunID != runID {
		t.Errorf("entry source_run_id = %q, want %q", entry.SourceRunID, runID)
	}
	if entry.SourceProposalID != proposalID {
		t.Errorf("entry source_proposal_id = %q, want %q", entry.SourceProposalID, proposalID)
	}
	if entry.SourceSpecID != "spec-compile-checks" {
		t.Errorf("entry source_spec_id = %q, want 'spec-compile-checks'", entry.SourceSpecID)
	}

	// 7. Entry ID matches the materialized ID in the decision
	if entry.ID != decision.MaterializedID {
		t.Errorf("entry ID = %q, decision materialized_id = %q — should match", entry.ID, decision.MaterializedID)
	}

	// 8. Decision records accepted action
	if decision.Action != "accepted" {
		t.Errorf("decision action = %q, want 'accepted'", decision.Action)
	}

	// 9. The entries.json file physically exists at the global path
	entriesPath := filepath.Join(tmp, "global", "playbook", "entries.json")
	if _, err := os.Stat(entriesPath); os.IsNotExist(err) {
		t.Fatal("expected entries.json at global playbook path, file does not exist")
	}

	// 10. Parse the file directly to verify JSON structure and provenance
	rawData, err := os.ReadFile(entriesPath)
	if err != nil {
		t.Fatalf("read entries.json: %v", err)
	}
	var rawEntries []map[string]interface{}
	if err := json.Unmarshal(rawData, &rawEntries); err != nil {
		t.Fatalf("unmarshal entries.json: %v", err)
	}
	if len(rawEntries) != 1 {
		t.Fatalf("expected 1 entry in raw JSON, got %d", len(rawEntries))
	}
	if rawEntries[0]["source_run_id"] != runID {
		t.Errorf("raw JSON source_run_id = %v, want %q", rawEntries[0]["source_run_id"], runID)
	}
}
