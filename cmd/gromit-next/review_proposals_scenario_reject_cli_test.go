package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/proposaltriage"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_RejectCLIRejectsPendingProposal(t *testing.T) {
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
				Type:                "validation_gap",
				Title:               "Validation Gap Proposal",
				WhatHappened:        "Something happened",
				WhatWasMissing:      "Something was missing",
				ProposedChange:      "Add validation check",
				Rationale:           "Improve test coverage",
				Confidence:          "medium",
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
	rejectCmd := newReviewProposalsRejectCmd()
	rejectCmd.SetArgs([]string{
		proposalID,
		"--store-dir", tmp,
		"--reason", "Not applicable to current architecture",
	})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	// Execute command
	err = rejectCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("reject command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===

	// 1. Command output contains expected messages
	if !strings.Contains(outputStr, "Proposal") || !strings.Contains(outputStr, "rejected") {
		t.Errorf("expected 'rejected' in output, got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "Reason:") {
		t.Errorf("expected 'Reason:' in output, got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "Not applicable to current architecture") {
		t.Errorf("expected reason text in output, got: %s", outputStr)
	}

	// 2. Decision file was created in run's evidence directory
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

	if decision.Action != "rejected" {
		t.Errorf("decision action = %q, want 'rejected'", decision.Action)
	}

	if decision.Reason != "Not applicable to current architecture" {
		t.Errorf("decision reason = %q, want 'Not applicable to current architecture'", decision.Reason)
	}

	// 4. Proposal no longer appears in pending list
	pendingAfter, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after reject: %v", err)
	}

	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after reject, got %d", len(pendingAfter))
	}
}

func TestScenario_RejectCLIRejectsAcceptedProposal(t *testing.T) {
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
				Type:           "validation_gap",
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

	// Create playbook entry and pre-create an acceptance decision for this proposal
	runEvidenceDir := filepath.Join(tmp, "runs", runID, "evidence")
	if err := os.MkdirAll(runEvidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	playbookDir := filepath.Join(projectDir, "playbook")
	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		t.Fatalf("mkdir playbook: %v", err)
	}

	// Create a playbook entry that matches the decision's MaterializedID
	entryID := "entry-abc123"
	playbookEntry := playbook.Entry{
		ID:     entryID,
		Type:   "validation_gap",
		Title:  "Test Proposal",
		Status: "active",
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	entries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("load playbook: %v", err)
	}
	entries = append(entries, playbookEntry)
	if err := pbStore.Save(entries); err != nil {
		t.Fatalf("save playbook: %v", err)
	}

	existingDecision := []proposaltriage.Decision{
		{
			ProposalID:     proposalID,
			Action:         "accepted",
			MaterializedID: entryID,
			DecidedAt:      time.Date(2026, 3, 21, 11, 0, 0, 0, time.UTC),
		},
	}
	if err := proposaltriage.SaveDecisions(runEvidenceDir, existingDecision); err != nil {
		t.Fatalf("save existing decision: %v", err)
	}

	// === Invoke ===
	rejectCmd := newReviewProposalsRejectCmd()
	rejectCmd.SetArgs([]string{
		proposalID,
		"--store-dir", tmp,
		"--reason", "Superseded by newer approach",
	})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	// Execute command
	err = rejectCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("reject command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===

	// 1. Command output contains expected messages
	if !strings.Contains(outputStr, "rejected") {
		t.Errorf("expected 'rejected' in output, got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "Previously accepted entry") {
		t.Errorf("expected 'Previously accepted entry' in output, got: %s", outputStr)
	}

	// 2. Decision file was created in run's evidence directory
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

	if decision.Action != "rejected" {
		t.Errorf("decision action = %q, want 'rejected'", decision.Action)
	}

	if decision.Reason != "Superseded by newer approach" {
		t.Errorf("decision reason = %q, want 'Superseded by newer approach'", decision.Reason)
	}
}

func TestScenario_RejectCLIRequiresReason(t *testing.T) {
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

	proposalID := "run-001-proposal-test-003"
	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-test",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		CreatedAt: time.Date(2026, 3, 21, 10, 20, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             proposalID,
				Type:           "validation_gap",
				Title:          "Test Proposal",
				ProposedChange: "Test change",
				Confidence:     "medium",
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
	rejectCmd := newReviewProposalsRejectCmd()
	rejectCmd.SetArgs([]string{
		proposalID,
		"--store-dir", tmp,
		// Intentionally omit --reason flag
	})

	// Execute command
	err = rejectCmd.Execute()

	// === Assert ===
	if err == nil {
		t.Fatal("expected error when rejecting without --reason, got nil")
	}

	if !strings.Contains(err.Error(), "reason") {
		t.Errorf("expected 'reason' in error message, got: %v", err)
	}
}

func TestScenario_RejectCLIRejectsUnknownProposal(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	projectID := "test-project"
	projectDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	// === Invoke ===
	rejectCmd := newReviewProposalsRejectCmd()
	rejectCmd.SetArgs([]string{
		"nonexistent-proposal-id",
		"--store-dir", tmp,
		"--reason", "Test reason",
	})

	// Execute command
	err := rejectCmd.Execute()

	// === Assert ===
	if err == nil {
		t.Fatal("expected error when rejecting nonexistent proposal, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}
