package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/proposaltriage"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_AcceptCLIAcceptsDoctrineProposalWithOverrides(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"
	runID := "run-001"

	// Create run
	run := &runstore.RunState{
		RunID:     runID,
		ProjectID: projectID,
		SpecID:    "spec-test",
		Status:    "completed",
		StartedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}
	run.NormalizeNilFields()
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create project directories
	projectDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	// Create evidence directory with proposals
	evidenceDir := store.RunEvidenceDir(runID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	proposalID := "run-001-proposal-test-001"
	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-test",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		CreatedAt: time.Date(2026, 3, 21, 10, 20, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  proposalID,
				Type:                "doctrine_rule",
				Title:               "Original Title",
				WhatHappened:        "Something happened",
				WhatWasMissing:      "Something was missing",
				ProposedChange:      "Original change text",
				Rationale:           "Original rationale",
				Confidence:          "high",
				ConfidenceRationale: "Test proposal",
				EvidenceReferences:  []string{},
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
	// Create the accept command
	acceptCmd := newReviewProposalsAcceptCmd()

	// Set up command with flags
	acceptCmd.SetArgs([]string{
		proposalID,
		"--store-dir", tmp,
		"--title", "Overridden Title",
		"--change", "Overridden change text",
		"--rationale", "Overridden rationale",
	})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	// Execute command
	err = acceptCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("accept command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===

	// 1. Command output contains expected messages
	if !strings.Contains(outputStr, "Proposal") || !strings.Contains(outputStr, "accepted") {
		t.Errorf("expected 'accepted' in output, got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "Materialized ID:") {
		t.Errorf("expected 'Materialized ID:' in output, got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "Target store:") {
		t.Errorf("expected 'Target store:' in output, got: %s", outputStr)
	}

	// 2. Decision file was created in run evidence directory
	runEvidenceDir := filepath.Join(tmp, "runs", runID, "evidence")
	decisionsPath := filepath.Join(runEvidenceDir, "proposal-decisions.json")
	decisionsRaw, err := os.ReadFile(decisionsPath)
	if err != nil {
		t.Fatalf("read decisions file: %v", err)
	}

	var savedDecisions []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw, &savedDecisions); err != nil {
		t.Fatalf("unmarshal decisions: %v", err)
	}

	if len(savedDecisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(savedDecisions))
	}

	decision := savedDecisions[0]

	// 3. Decision has correct properties
	if decision.ProposalID != proposalID {
		t.Errorf("decision proposal_id = %q, want %q", decision.ProposalID, proposalID)
	}

	if decision.Action != "accepted" {
		t.Errorf("decision action = %q, want 'accepted'", decision.Action)
	}

	// 4. Overrides were applied
	if decision.ApprovedTitle != "Overridden Title" {
		t.Errorf("decision approved_title = %q, want 'Overridden Title'", decision.ApprovedTitle)
	}

	if decision.ApprovedChange != "Overridden change text" {
		t.Errorf("decision approved_change = %q, want 'Overridden change text'", decision.ApprovedChange)
	}

	if decision.ApprovedRationale != "Overridden rationale" {
		t.Errorf("decision approved_rationale = %q, want 'Overridden rationale'", decision.ApprovedRationale)
	}

	// 5. Doctrine rule was created
	doctrineDir := filepath.Join(projectDir, "doctrine")
	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = doctrineDir
	loadedDoctrine, err := doctrineStore.Load()
	if err != nil {
		t.Fatalf("load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("expected 1 doctrine rule, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]

	// 6. Materialized ID in decision matches the created rule
	if decision.MaterializedID != rule.ID {
		t.Errorf("materialized_id = %q, want %q (matching rule ID)", decision.MaterializedID, rule.ID)
	}

	// 7. Rule has expected properties
	if rule.Summary != "Overridden Title" {
		t.Errorf("rule summary = %q, want 'Overridden Title'", rule.Summary)
	}

	expectedSource := "promoted:" + proposalID
	if rule.Source != expectedSource {
		t.Errorf("rule source = %q, want %q", rule.Source, expectedSource)
	}

	if rule.Status != "active" {
		t.Errorf("rule status = %q, want 'active'", rule.Status)
	}

	// 8. Proposal no longer appears in pending list
	pendingAfter, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after accept: %v", err)
	}

	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after accept, got %d", len(pendingAfter))
	}
}

func TestScenario_AcceptCLIRejectsAlreadyDecidedProposal(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"
	runID := "run-001"

	// Create run
	run := &runstore.RunState{
		RunID:     runID,
		ProjectID: projectID,
		SpecID:    "spec-test",
		Status:    "completed",
		StartedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}
	run.NormalizeNilFields()
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create project directories
	projectDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	// Create evidence directory with proposals
	evidenceDir := store.RunEvidenceDir(runID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	proposalID := "run-001-proposal-test-002"
	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-test",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		CreatedAt: time.Date(2026, 3, 21, 10, 20, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             proposalID,
				Type:           "doctrine_rule",
				Title:          "Test Proposal",
				ProposedChange: "Test change",
				Confidence:     "high",
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

	// Pre-create a decision for this proposal (simulate already accepted)
	runEvidenceDir := filepath.Join(tmp, "runs", runID, "evidence")
	if err := os.MkdirAll(runEvidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	existingDecision := []proposaltriage.Decision{
		{
			ProposalID:     proposalID,
			Action:         "accepted",
			MaterializedID: "promoted-abc123",
			DecidedAt:      time.Date(2026, 3, 21, 11, 0, 0, 0, time.UTC),
		},
	}
	if err := proposaltriage.SaveDecisions(runEvidenceDir, existingDecision); err != nil {
		t.Fatalf("save existing decision: %v", err)
	}

	// === Invoke ===
	acceptCmd := newReviewProposalsAcceptCmd()
	acceptCmd.SetArgs([]string{
		proposalID,
		"--store-dir", tmp,
	})

	// Execute command
	err = acceptCmd.Execute()

	// === Assert ===
	// Command should fail with "already has a decision" error
	if err == nil {
		t.Fatal("expected error when accepting already-decided proposal, got nil")
	}

	if !strings.Contains(err.Error(), "already has a decision") {
		t.Errorf("expected 'already has a decision' in error, got: %v", err)
	}
}

func TestScenario_AcceptCLIRejectsUnknownProposal(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	projectID := "test-project"
	projectDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	// === Invoke ===
	acceptCmd := newReviewProposalsAcceptCmd()
	acceptCmd.SetArgs([]string{
		"nonexistent-proposal-id",
		"--store-dir", tmp,
	})

	// Execute command
	err := acceptCmd.Execute()

	// === Assert ===
	if err == nil {
		t.Fatal("expected error when accepting nonexistent proposal, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}
