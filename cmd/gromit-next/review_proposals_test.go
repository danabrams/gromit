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
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/proposaltriage"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestReviewProposalsList_DefaultShowsPendingOnly(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"

	// Create projects directory
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Run 1: 1 pending proposal, 1 accepted
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

	// Mark proposal-1 as accepted, proposal-2 as pending
	decisions1 := []proposaltriage.Decision{
		{
			ProposalID: "run-1-proposal-1",
			Action:     "accepted",
			Reason:     "good rule",
			DecidedAt:  time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		},
	}
	if err := proposaltriage.SaveDecisions(evidenceDir1, decisions1); err != nil {
		t.Fatalf("save decisions1: %v", err)
	}

	// === Invoke ===
	listCmd := newReviewProposalsListCmd()
	listCmd.SetArgs([]string{"--store-dir", tmp})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = listCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===
	// Should only show pending proposal-2, not accepted proposal-1
	if !strings.Contains(outputStr, "Split large tasks") {
		t.Error("expected pending proposal 'Split large tasks' in output")
	}

	if strings.Contains(outputStr, "Enforce error handling") {
		t.Error("should not show accepted proposal 'Enforce error handling' in default list")
	}

	// Table header should be present (without STATUS column for pending list)
	if !strings.Contains(outputStr, "ID") || !strings.Contains(outputStr, "TYPE") {
		t.Error("expected table headers in output")
	}
}

func TestReviewProposalsList_AllFlagIncludesDecided(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"

	// Create projects directory
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

	// Run 2: 1 pending proposal
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
	listCmd := newReviewProposalsListCmd()
	listCmd.SetArgs([]string{"--store-dir", tmp, "--all"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = listCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===
	// With --all, should show all 3 proposals: 1 accepted, 1 rejected, 1 pending
	if !strings.Contains(outputStr, "Enforce error handling") {
		t.Error("expected accepted proposal 'Enforce error handling'")
	}

	if !strings.Contains(outputStr, "Split large tasks") {
		t.Error("expected rejected proposal 'Split large tasks'")
	}

	if !strings.Contains(outputStr, "Add integration tests") {
		t.Error("expected pending proposal 'Add integration tests'")
	}

	// Should have STATUS column with all flag
	if !strings.Contains(outputStr, "STATUS") {
		t.Error("expected STATUS column header with --all flag")
	}

	// Should show statuses
	if !strings.Contains(outputStr, "accepted") {
		t.Error("expected 'accepted' status in output")
	}
	if !strings.Contains(outputStr, "rejected") {
		t.Error("expected 'rejected' status in output")
	}
	if !strings.Contains(outputStr, "pending") {
		t.Error("expected 'pending' status in output")
	}
}

func TestReviewProposalsList_TypeFilterWorks(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"

	// Create projects directory
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Create run with multiple proposal types
	run := &runstore.RunState{
		RunID:     "run-1",
		ProjectID: projectID,
		SpecID:    "spec-alpha",
		Status:    "completed",
		StartedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	run.NormalizeNilFields()
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir("run-1")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	proposals := reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-alpha",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-1-proposal-doctrine",
				Type:           "doctrine_rule",
				Title:          "Enforce error handling",
				ProposedChange: "Always check errors",
				Confidence:     "high",
			},
			{
				ID:             "run-1-proposal-validation",
				Type:           "validation_gap",
				Title:          "Add missing tests",
				ProposedChange: "Cover edge cases",
				Confidence:     "medium",
			},
			{
				ID:             "run-1-proposal-planner",
				Type:           "planner_heuristic",
				Title:          "Split large tasks",
				ProposedChange: "Decompose over 4 files",
				Confidence:     "low",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 30, 0, 0, time.UTC),
	}
	pData, err := json.MarshalIndent(proposals, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), pData, 0o644); err != nil {
		t.Fatalf("write proposals: %v", err)
	}

	// === Invoke ===
	// Filter by doctrine_rule type
	listCmd := newReviewProposalsListCmd()
	listCmd.SetArgs([]string{"--store-dir", tmp, "--type", "doctrine_rule"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = listCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===
	// Should only show doctrine_rule proposal
	if !strings.Contains(outputStr, "Enforce error handling") {
		t.Error("expected doctrine_rule proposal 'Enforce error handling'")
	}

	// Should not show other types
	if strings.Contains(outputStr, "Add missing tests") {
		t.Error("should not show validation_gap proposal")
	}

	if strings.Contains(outputStr, "Split large tasks") {
		t.Error("should not show planner_heuristic proposal")
	}

	// TYPE column should show doctrine_rule
	if !strings.Contains(outputStr, "doctrine_rule") {
		t.Error("expected 'doctrine_rule' type in output")
	}
}

func TestReviewProposalsList_RunFilterWorks(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"

	// Create projects directory
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Run 1: 1 proposal
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
				Title:          "Run 1 proposal",
				ProposedChange: "Changes for run 1",
				Confidence:     "high",
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

	// Run 2: 1 proposal
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
				Title:          "Run 2 proposal",
				ProposedChange: "Changes for run 2",
				Confidence:     "medium",
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
	// Filter by run-2 only
	listCmd := newReviewProposalsListCmd()
	listCmd.SetArgs([]string{"--store-dir", tmp, "--run", "run-2"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = listCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===
	// Should only show run-2 proposal
	if !strings.Contains(outputStr, "Run 2 proposal") {
		t.Error("expected run-2 proposal in output")
	}

	if strings.Contains(outputStr, "Run 1 proposal") {
		t.Error("should not show run-1 proposal")
	}

	// RUN column should show run-2
	if !strings.Contains(outputStr, "run-2") {
		t.Error("expected 'run-2' run ID in output")
	}
}

func TestReviewProposalsList_EmptyResultMessage(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	projectID := "test-project"

	// Create projects directory (empty, no proposals)
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// === Invoke ===
	listCmd := newReviewProposalsListCmd()
	listCmd.SetArgs([]string{"--store-dir", tmp})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = listCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===
	// Should show appropriate empty message
	if !strings.Contains(outputStr, "No pending proposals found") && !strings.Contains(outputStr, "No proposals found") {
		t.Errorf("expected 'No pending proposals found' or 'No proposals found' message, got: %s", outputStr)
	}
}

func TestReviewProposalsShow_DisplaysAllFieldsWithDecision(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"

	// Create projects directory
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Create a run
	run := &runstore.RunState{
		RunID:     "run-showcase",
		ProjectID: projectID,
		SpecID:    "spec-complete",
		Status:    "completed",
		StartedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	run.NormalizeNilFields()
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir("run-showcase")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Create a comprehensive proposal with all fields populated
	proposals := reviewdistiller.DistillationResult{
		RunID:     "run-showcase",
		SpecID:    "spec-complete",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  "proposal-comprehensive",
				Type:                "doctrine_rule",
				Title:               "Add comprehensive error handling",
				WhatHappened:        "Code did not properly validate inputs from external APIs",
				WhatWasMissing:      "Error handling was missing for network timeouts and malformed responses",
				ProposedChange:      "Wrap all external API calls with proper error handling and retry logic",
				Rationale:           "This prevents silent failures and improves system reliability",
				Confidence:          "high",
				ConfidenceRationale: "The pattern is well-established and applies broadly",
				EvidenceReferences: []string{
					"Issue #123: Network timeout caused silent data loss",
					"Code review comment on line 456: Missing error handling",
				},
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 30, 0, 0, time.UTC),
	}
	pData, err := json.MarshalIndent(proposals, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), pData, 0o644); err != nil {
		t.Fatalf("write proposals: %v", err)
	}

	// Add a decision
	decisions := []proposaltriage.Decision{
		{
			ProposalID:        "proposal-comprehensive",
			Action:            "accepted",
			Reason:            "Aligns with our reliability standards",
			ApprovedTitle:     "Implement comprehensive error handling for external APIs",
			ApprovedChange:    "Add retry logic and timeout handling to all API calls",
			ApprovedRationale: "Critical for production stability",
			MaterializedID:    "doc-error-handling-001",
			DecidedAt:         time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		},
	}
	if err := proposaltriage.SaveDecisions(evidenceDir, decisions); err != nil {
		t.Fatalf("save decisions: %v", err)
	}

	// === Invoke ===
	showCmd := newReviewProposalsShowCmd()
	showCmd.SetArgs([]string{"proposal-comprehensive", "--store-dir", tmp})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = showCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===
	// Verify all proposal fields are present
	checks := map[string]string{
		"proposal detail header": "=== Proposal Detail ===",
		"proposal ID":            "proposal-comprehensive",
		"proposal type":          "doctrine_rule",
		"proposal title":         "Add comprehensive error handling",
		"proposal confidence":    "high",
		"what happened":          "Code did not properly validate inputs from external APIs",
		"what was missing":       "Error handling was missing for network timeouts",
		"proposed change":        "Wrap all external API calls with proper error handling",
		"rationale":              "This prevents silent failures",
		"confidence rationale":   "The pattern is well-established",
		"evidence reference 1":   "Issue #123: Network timeout caused silent data loss",
		"evidence reference 2":   "Code review comment on line 456",
		"source context run ID":  "run-showcase",
		"source context spec ID": "spec-complete",
		"decision status":        "accepted",
		"decision reason":        "Aligns with our reliability standards",
		"approved title":         "Implement comprehensive error handling for external APIs",
		"approved change":        "Add retry logic and timeout handling",
		"approved rationale":     "Critical for production stability",
		"materialized ID":        "doc-error-handling-001",
		"decision timestamp":     "2026-03-20",
	}

	for checkName, expectedValue := range checks {
		if !strings.Contains(outputStr, expectedValue) {
			t.Errorf("expected %q to contain %q, got:\n%s", checkName, expectedValue, outputStr)
		}
	}
}

func TestReviewProposalsShow_DisplaysPendingProposalWithoutDecision(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"

	// Create projects directory
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Create a run
	run := &runstore.RunState{
		RunID:     "run-pending",
		ProjectID: projectID,
		SpecID:    "spec-alpha",
		Status:    "completed",
		StartedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	run.NormalizeNilFields()
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir("run-pending")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Create a pending proposal (no decision yet)
	proposals := reviewdistiller.DistillationResult{
		RunID:     "run-pending",
		SpecID:    "spec-alpha",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  "proposal-pending",
				Type:                "validation_gap",
				Title:               "Add missing integration tests",
				WhatHappened:        "Several code paths were not covered by tests",
				WhatWasMissing:      "Integration tests for the payment flow",
				ProposedChange:      "Add comprehensive integration tests covering success and failure paths",
				Rationale:           "Improves test coverage and prevents regressions",
				Confidence:          "high",
				ConfidenceRationale: "The gap is clear and solutions are straightforward",
				EvidenceReferences:  []string{"Coverage report showed 45% gap in payment module"},
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 30, 0, 0, time.UTC),
	}
	pData, err := json.MarshalIndent(proposals, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), pData, 0o644); err != nil {
		t.Fatalf("write proposals: %v", err)
	}

	// Note: no decision file created, so proposal is pending

	// === Invoke ===
	showCmd := newReviewProposalsShowCmd()
	showCmd.SetArgs([]string{"proposal-pending", "--store-dir", tmp})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = showCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===
	// Verify proposal fields
	if !strings.Contains(outputStr, "proposal-pending") {
		t.Error("expected proposal ID in output")
	}
	if !strings.Contains(outputStr, "Add missing integration tests") {
		t.Error("expected proposal title in output")
	}
	if !strings.Contains(outputStr, "validation_gap") {
		t.Error("expected proposal type in output")
	}
	if !strings.Contains(outputStr, "Several code paths were not covered by tests") {
		t.Error("expected 'what happened' in output")
	}

	// Verify decision section shows pending status
	if !strings.Contains(outputStr, "Status: pending") {
		t.Error("expected 'Status: pending' in decision section")
	}

	// Should not have accepted/rejected status
	if strings.Contains(outputStr, "Status: accepted") || strings.Contains(outputStr, "Status: rejected") {
		t.Error("should show pending status, not accepted/rejected")
	}
}

func TestReviewProposalsShow_NotFoundError(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	projectID := "test-project"

	// Create projects directory (empty, no proposals)
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// === Invoke ===
	showCmd := newReviewProposalsShowCmd()
	showCmd.SetArgs([]string{"nonexistent-proposal", "--store-dir", tmp})

	// Execute should return error
	err := showCmd.Execute()

	// === Assert ===
	if err == nil {
		t.Error("expected error for nonexistent proposal, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message, got: %v", err)
	}
}

func TestReviewProposalsAccept_ComprehensiveMaterialization(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"
	runID := "run-accept-test"

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

	// Create two proposals: one doctrine_rule, one validation_gap
	doctrineProposalID := "run-accept-test-doctrine-001"
	playbookProposalID := "run-accept-test-playbook-001"

	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-test",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		CreatedAt: time.Date(2026, 3, 21, 10, 20, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  doctrineProposalID,
				Type:                "doctrine_rule",
				Title:               "Original Doctrine Title",
				WhatHappened:        "Doctrine issue happened",
				WhatWasMissing:      "Doctrine context was missing",
				ProposedChange:      "Original doctrine change text",
				Rationale:           "Original doctrine rationale",
				Confidence:          "high",
				ConfidenceRationale: "Doctrine test proposal",
				EvidenceReferences:  []string{},
			},
			{
				ID:                  playbookProposalID,
				Type:                "validation_gap",
				Title:               "Original Playbook Title",
				WhatHappened:        "Playbook issue happened",
				WhatWasMissing:      "Playbook context was missing",
				ProposedChange:      "Original playbook change text",
				Rationale:           "Original playbook rationale",
				Confidence:          "high",
				ConfidenceRationale: "Playbook test proposal",
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
	// Accept the doctrine_rule proposal with field overrides
	doctrineAcceptCmd := newReviewProposalsAcceptCmd()
	doctrineAcceptCmd.SetArgs([]string{
		doctrineProposalID,
		"--store-dir", tmp,
		"--title", "Overridden Doctrine Title",
		"--change", "Overridden doctrine change text",
		"--rationale", "Overridden doctrine rationale",
	})

	// Capture stdout for doctrine acceptance
	oldStdout := os.Stdout
	r1, w1, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w1

	err = doctrineAcceptCmd.Execute()

	w1.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("doctrine accept command failed: %v", err)
	}

	// Read doctrine acceptance output
	output1, err := io.ReadAll(r1)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	doctrineOutputStr := string(output1)

	// === Assertions for Doctrine Acceptance ===

	// 1. Command output shows acceptance and materialized ID
	if !strings.Contains(doctrineOutputStr, "accepted") {
		t.Errorf("expected 'accepted' in doctrine output, got: %s", doctrineOutputStr)
	}
	if !strings.Contains(doctrineOutputStr, "Materialized ID:") {
		t.Errorf("expected 'Materialized ID:' in doctrine output")
	}
	if !strings.Contains(doctrineOutputStr, "doctrine") {
		t.Errorf("expected 'doctrine' in target store output")
	}

	// 2. Doctrine rule was created in doctrine store
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

	doctrineRule := loadedDoctrine.Rules[0]

	// 3. Field overrides were applied to doctrine rule
	if doctrineRule.Summary != "Overridden Doctrine Title" {
		t.Errorf("doctrine rule summary = %q, want 'Overridden Doctrine Title'", doctrineRule.Summary)
	}
	if doctrineRule.Source != "promoted:"+doctrineProposalID {
		t.Errorf("doctrine rule source = %q, want 'promoted:%s'", doctrineRule.Source, doctrineProposalID)
	}
	if doctrineRule.Status != "active" {
		t.Errorf("doctrine rule status = %q, want 'active'", doctrineRule.Status)
	}

	// 4. Decision was saved with correct materialized ID
	decisionsRaw, err := os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read decisions file: %v", err)
	}

	var savedDecisions []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw, &savedDecisions); err != nil {
		t.Fatalf("unmarshal decisions: %v", err)
	}

	var doctrineDecision *proposaltriage.Decision
	for i := range savedDecisions {
		if savedDecisions[i].ProposalID == doctrineProposalID {
			doctrineDecision = &savedDecisions[i]
			break
		}
	}

	if doctrineDecision == nil {
		t.Fatalf("doctrine decision not found")
	}

	if doctrineDecision.ApprovedTitle != "Overridden Doctrine Title" {
		t.Errorf("decision approved_title = %q, want 'Overridden Doctrine Title'", doctrineDecision.ApprovedTitle)
	}
	if doctrineDecision.ApprovedChange != "Overridden doctrine change text" {
		t.Errorf("decision approved_change = %q, want 'Overridden doctrine change text'", doctrineDecision.ApprovedChange)
	}
	if doctrineDecision.MaterializedID != doctrineRule.ID {
		t.Errorf("decision materialized_id = %q, want %q (matching rule ID)", doctrineDecision.MaterializedID, doctrineRule.ID)
	}

	// === Invoke ===
	// Accept the playbook proposal without overrides to test playbook materialization
	playbookAcceptCmd := newReviewProposalsAcceptCmd()
	playbookAcceptCmd.SetArgs([]string{
		playbookProposalID,
		"--store-dir", tmp,
	})

	// Capture stdout for playbook acceptance
	r2, w2, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w2

	err = playbookAcceptCmd.Execute()

	w2.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("playbook accept command failed: %v", err)
	}

	// Read playbook acceptance output
	output2, err := io.ReadAll(r2)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	playbookOutputStr := string(output2)

	// === Assertions for Playbook Acceptance ===

	// 1. Command output shows acceptance for playbook type
	if !strings.Contains(playbookOutputStr, "accepted") {
		t.Errorf("expected 'accepted' in playbook output, got: %s", playbookOutputStr)
	}
	if !strings.Contains(playbookOutputStr, "Materialized ID:") {
		t.Errorf("expected 'Materialized ID:' in playbook output")
	}
	if !strings.Contains(playbookOutputStr, "playbook") {
		t.Errorf("expected 'playbook' in target store output")
	}

	// 2. Playbook entry was created
	playbookDir := filepath.Join(projectDir, "playbook")
	playbookStore := &playbook.Store{Dir: playbookDir}
	loadedPlaybookEntries, err := playbookStore.Load()
	if err != nil {
		t.Fatalf("load playbook: %v", err)
	}
	if len(loadedPlaybookEntries) != 1 {
		t.Fatalf("expected 1 playbook entry, got %d", len(loadedPlaybookEntries))
	}

	playbookEntry := loadedPlaybookEntries[0]

	// 3. Playbook entry has correct properties
	if playbookEntry.Title != "Original Playbook Title" {
		t.Errorf("playbook entry title = %q, want 'Original Playbook Title'", playbookEntry.Title)
	}

	// 4. Playbook decision was saved
	updatedDecisionsRaw, err := os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read updated decisions file: %v", err)
	}

	var updatedDecisions []proposaltriage.Decision
	if err := json.Unmarshal(updatedDecisionsRaw, &updatedDecisions); err != nil {
		t.Fatalf("unmarshal updated decisions: %v", err)
	}

	var playbookDecision *proposaltriage.Decision
	for i := range updatedDecisions {
		if updatedDecisions[i].ProposalID == playbookProposalID {
			playbookDecision = &updatedDecisions[i]
			break
		}
	}

	if playbookDecision == nil {
		t.Fatalf("playbook decision not found")
	}

	if playbookDecision.Action != "accepted" {
		t.Errorf("playbook decision action = %q, want 'accepted'", playbookDecision.Action)
	}
	if playbookDecision.MaterializedID != playbookEntry.ID {
		t.Errorf("playbook decision materialized_id = %q, want %q (matching entry ID)", playbookDecision.MaterializedID, playbookEntry.ID)
	}

	// 5. Both proposals no longer appear in pending list
	pendingAfter, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after accept: %v", err)
	}

	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after accepting both, got %d", len(pendingAfter))
	}
}

func TestReviewProposalsReject_BasicRejection(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"

	// Create projects directory
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Run 1: Create a run with a proposal to reject
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

	evidenceDir := store.RunEvidenceDir("run-1")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	proposalID := "run-1-proposal-to-reject"
	proposals := reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-alpha",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             proposalID,
				Type:           "validation_gap",
				Title:          "Add missing edge case tests",
				ProposedChange: "Test null pointer handling",
				Rationale:      "Current tests don't cover null cases",
				Confidence:     "high",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 30, 0, 0, time.UTC),
	}
	pData, err := json.MarshalIndent(proposals, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), pData, 0o644); err != nil {
		t.Fatalf("write proposals: %v", err)
	}

	// === Invoke: Reject the proposal ===
	rejectCmd := newReviewProposalsRejectCmd()
	rejectCmd.SetArgs([]string{
		"--store-dir", tmp,
		proposalID,
		"--reason", "Too specific to this code path, better covered by scenario tests",
	})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

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

	// === Assertions ===

	// 1. Rejection message appears in output
	if !strings.Contains(outputStr, "rejected") {
		t.Errorf("expected 'rejected' in output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Too specific to this code path") {
		t.Errorf("expected rejection reason in output")
	}

	// 2. Decision was saved to run's evidence directory
	decisionsPath := filepath.Join(evidenceDir, "proposal-decisions.json")
	decisionsRaw, err := os.ReadFile(decisionsPath)
	if err != nil {
		t.Fatalf("read decisions: %v", err)
	}

	var decisions []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw, &decisions); err != nil {
		t.Fatalf("unmarshal decisions: %v", err)
	}

	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}

	decision := decisions[0]
	if decision.ProposalID != proposalID {
		t.Errorf("decision proposal_id = %q, want %q", decision.ProposalID, proposalID)
	}
	if decision.Action != "rejected" {
		t.Errorf("decision action = %q, want 'rejected'", decision.Action)
	}
	if decision.Reason != "Too specific to this code path, better covered by scenario tests" {
		t.Errorf("decision reason = %q, want custom reason", decision.Reason)
	}

	// 3. Proposal no longer appears in pending list
	pendingAfterReject, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after reject: %v", err)
	}

	if len(pendingAfterReject) != 0 {
		t.Fatalf("expected 0 pending proposals after rejection, got %d", len(pendingAfterReject))
	}
}

func TestReviewProposalsReject_RejectAfterAcceptSupersedes(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"

	// Create projects directory
	projectsDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Run 1: Create a run with a proposal to accept then reject
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

	evidenceDir := store.RunEvidenceDir("run-1")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	proposalID := "run-1-proposal-accept-then-reject"
	proposals := reviewdistiller.DistillationResult{
		RunID:     "run-1",
		SpecID:    "spec-alpha",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             proposalID,
				Type:           "planner_heuristic",
				Title:          "Always prefer module boundaries",
				ProposedChange: "Decompose tasks at module boundaries",
				Rationale:      "Reduces coupling",
				Confidence:     "high",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 30, 0, 0, time.UTC),
	}
	pData, err := json.MarshalIndent(proposals, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), pData, 0o644); err != nil {
		t.Fatalf("write proposals: %v", err)
	}

	// === Step 1: Accept the proposal ===
	acceptCmd := newReviewProposalsAcceptCmd()
	acceptCmd.SetArgs([]string{"--store-dir", tmp, proposalID})

	oldStdout := os.Stdout
	r1, w1, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w1

	err = acceptCmd.Execute()

	w1.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("accept command failed: %v", err)
	}

	// Clear the pipe
	io.ReadAll(r1)

	// Verify playbook entry was created
	projectDir := filepath.Join(tmp, "projects", projectID)
	playbookDir := filepath.Join(projectDir, "playbook")
	playbookStore := &playbook.Store{Dir: playbookDir}
	entriesBefore, err := playbookStore.Load()
	if err != nil {
		t.Fatalf("load playbook: %v", err)
	}
	if len(entriesBefore) != 1 {
		t.Fatalf("expected 1 playbook entry after accept, got %d", len(entriesBefore))
	}

	entryBefore := entriesBefore[0]
	if entryBefore.Status != "active" {
		t.Errorf("entry status = %q, want 'active'", entryBefore.Status)
	}

	// Load the accepted decision to get materialized ID
	decisionsRaw, err := os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read decisions: %v", err)
	}

	var decisions []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw, &decisions); err != nil {
		t.Fatalf("unmarshal decisions: %v", err)
	}

	var acceptedDecision *proposaltriage.Decision
	for i := range decisions {
		if decisions[i].ProposalID == proposalID {
			acceptedDecision = &decisions[i]
			break
		}
	}

	if acceptedDecision == nil {
		t.Fatalf("accepted decision not found")
	}

	// === Step 2: Reject the accepted proposal ===
	rejectCmd := newReviewProposalsRejectCmd()
	rejectCmd.SetArgs([]string{
		"--store-dir", tmp,
		proposalID,
		"--reason", "Turns out it causes worse decomposition in practice",
	})

	r2, w2, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w2

	err = rejectCmd.Execute()

	w2.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("reject command failed: %v", err)
	}

	// Read rejection output
	output, err := io.ReadAll(r2)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assertions for Reject-After-Accept ===

	// 1. Rejection message mentions superseding
	if !strings.Contains(outputStr, "rejected") {
		t.Errorf("expected 'rejected' in output")
	}
	if !strings.Contains(outputStr, "superseded") {
		t.Errorf("expected 'superseded' in output when rejecting accepted proposal")
	}

	// 2. Decision was updated in evidence directory
	decisionsRaw, err = os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read updated decisions: %v", err)
	}

	var updatedDecisions []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw, &updatedDecisions); err != nil {
		t.Fatalf("unmarshal updated decisions: %v", err)
	}

	// Should still be 1 decision (overwritten, not added)
	if len(updatedDecisions) != 1 {
		t.Fatalf("expected 1 decision after reject-after-accept, got %d", len(updatedDecisions))
	}

	updatedDecision := updatedDecisions[0]
	if updatedDecision.ProposalID != proposalID {
		t.Errorf("decision proposal_id = %q, want %q", updatedDecision.ProposalID, proposalID)
	}
	if updatedDecision.Action != "rejected" {
		t.Errorf("decision action = %q, want 'rejected'", updatedDecision.Action)
	}
	if updatedDecision.Reason != "Turns out it causes worse decomposition in practice" {
		t.Errorf("decision reason not updated after reject")
	}

	// 3. Playbook entry is now superseded
	entriesAfter, err := playbookStore.Load()
	if err != nil {
		t.Fatalf("load playbook after reject: %v", err)
	}

	if len(entriesAfter) != 1 {
		t.Fatalf("expected 1 playbook entry after reject, got %d", len(entriesAfter))
	}

	entryAfter := entriesAfter[0]
	if entryAfter.Status != "superseded" {
		t.Errorf("entry status = %q, want 'superseded'", entryAfter.Status)
	}
	if entryAfter.SupersededBy == "" {
		t.Errorf("entry superseded_by should be set, got empty")
	}

	// 4. Entry ID hasn't changed (same entry, just marked superseded)
	if entryAfter.ID != entryBefore.ID {
		t.Errorf("entry ID changed during supersession: before=%q, after=%q", entryBefore.ID, entryAfter.ID)
	}
}
