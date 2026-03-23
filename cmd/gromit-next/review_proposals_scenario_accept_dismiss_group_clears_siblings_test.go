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

func TestScenario_AcceptWithDismissGroup_ClearsSiblings(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-isolation-app"

	runIDs := []string{"run-302", "run-303", "run-304"}
	proposalIDs := []string{
		"run-302-proposal-11223344",
		"run-303-proposal-55667788",
		"run-304-proposal-99aabbcc",
	}

	// All three proposals suggest identical doctrine about test isolation
	identicalChange := "Each test must create its own isolated fixture state; shared mutable fixtures are prohibited"

	for i, runID := range runIDs {
		rs := &runstore.RunState{
			RunID:                 runID,
			SpecID:                "spec-test-isolation",
			ProjectID:             projectID,
			Status:                runstore.StatusReadyForReview,
			StartedAt:             time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
			EndedAt:               time.Date(2026, 3, 20, 10, 30, 0, 0, time.UTC),
			FinalValidationPassed: true,
			FinalReviewPassed:     true,
			FinalAcceptancePassed: true,
			Tasks: []runstore.Task{
				{TaskID: "task-1", Status: "done", ModelTier: "sonnet"},
			},
		}
		if err := store.Save(rs); err != nil {
			t.Fatalf("save run %s: %v", runID, err)
		}

		evidenceDir := store.RunEvidenceDir(runID)
		if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
			t.Fatalf("mkdir evidence for %s: %v", runID, err)
		}

		distResult := reviewdistiller.DistillationResult{
			RunID:     runID,
			SpecID:    "spec-test-isolation",
			Outcome:   "accepted",
			ModelTier: reviewdistiller.TierHigh,
			CreatedAt: time.Date(2026, 3, 20, 10, 35, 0, 0, time.UTC),
			Proposals: []reviewdistiller.Proposal{
				{
					ID:                  proposalIDs[i],
					Type:                "doctrine_rule",
					Title:               "Test isolation doctrine",
					WhatHappened:        "Tests sharing mutable state caused flaky failures",
					WhatWasMissing:      "Enforcement of test isolation boundaries",
					ProposedChange:      identicalChange,
					Rationale:           "Prevents cross-test contamination and flaky CI",
					Confidence:          "high",
					ConfidenceRationale: "Observed in multiple runs across different specs",
					EvidenceReferences:  []string{"review-outcome.json"},
				},
			},
		}

		proposalsJSON, err := json.MarshalIndent(distResult, "", "  ")
		if err != nil {
			t.Fatalf("marshal proposals for %s: %v", runID, err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), proposalsJSON, 0o644); err != nil {
			t.Fatalf("write proposals for %s: %v", runID, err)
		}
	}

	// === Verify precondition: 3 pending proposals ===
	pendingBefore, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before: %v", err)
	}
	if len(pendingBefore) != 3 {
		t.Fatalf("expected 3 pending proposals before accept, got %d", len(pendingBefore))
	}

	// Verify grouping produces 1 group of 3 (identical type+proposed_change)
	groups := proposaltriage.GroupByContentHash(pendingBefore)
	if len(groups) != 1 {
		t.Fatalf("expected 1 content-hash group, got %d", len(groups))
	}
	if len(groups[0].Proposals) != 3 {
		t.Fatalf("expected 3 proposals in group, got %d", len(groups[0].Proposals))
	}

	// === Invoke: accept run-302 proposal with --dismiss-group ===
	// Find run-302 proposal
	var targetPP *proposaltriage.PendingProposal
	for i := range pendingBefore {
		if pendingBefore[i].Proposal.ID == "run-302-proposal-11223344" {
			targetPP = &pendingBefore[i]
			break
		}
	}
	if targetPP == nil {
		t.Fatal("target proposal run-302-proposal-11223344 not found")
	}

	// Set up doctrine and playbook stores
	projectDir := filepath.Join(tmp, "projects", projectID)
	doctrineDir := filepath.Join(projectDir, "doctrine")
	playbookDir := filepath.Join(projectDir, "playbook")

	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = doctrineDir
	playbookStore := &playbook.Store{Dir: playbookDir}

	// Promote the run-302 proposal into doctrine (scope=local)
	decision, err := proposaltriage.Promote(
		targetPP,
		"", "", "", // no overrides
		doctrineStore,
		playbookStore,
		"local",
	)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}
	if decision.Action != "accepted" {
		t.Fatalf("expected accepted decision, got %q", decision.Action)
	}

	// Save accepted decision to run-302 evidence
	evidenceDir302 := store.RunEvidenceDir("run-302")
	if err := proposaltriage.SaveDecisions(evidenceDir302, []proposaltriage.Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions for run-302: %v", err)
	}

	// Dismiss siblings in the group
	dismissedDecisions, err := proposaltriage.DismissSiblings(
		"run-302-proposal-11223344",
		groups[0],
		store,
	)
	if err != nil {
		t.Fatalf("DismissSiblings failed: %v", err)
	}
	if len(dismissedDecisions) != 2 {
		t.Fatalf("expected 2 dismissed decisions, got %d", len(dismissedDecisions))
	}

	// === Assert ===

	// 1. Doctrine materialized with correct content
	loadedDoctrine, err := doctrineStore.Load()
	if err != nil {
		t.Fatalf("load doctrine: %v", err)
	}
	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("expected 1 doctrine rule, got %d", len(loadedDoctrine.Rules))
	}
	rule := loadedDoctrine.Rules[0]
	if rule.Summary != "Test isolation doctrine" {
		t.Errorf("rule summary = %q, want 'Test isolation doctrine'", rule.Summary)
	}
	if rule.Scope != "local" {
		t.Errorf("rule scope = %q, want 'local'", rule.Scope)
	}

	// 2. Run-302 decision is accepted with materialized ID
	raw302, err := os.ReadFile(filepath.Join(evidenceDir302, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read run-302 decisions: %v", err)
	}
	var decisions302 []proposaltriage.Decision
	if err := json.Unmarshal(raw302, &decisions302); err != nil {
		t.Fatalf("unmarshal run-302 decisions: %v", err)
	}
	if len(decisions302) != 1 {
		t.Fatalf("expected 1 decision for run-302, got %d", len(decisions302))
	}
	if decisions302[0].Action != "accepted" {
		t.Errorf("run-302 decision action = %q, want 'accepted'", decisions302[0].Action)
	}
	if decisions302[0].MaterializedID == "" {
		t.Error("run-302 decision should have a materialized ID")
	}
	if decisions302[0].MaterializedID != rule.ID {
		t.Errorf("materialized ID %q does not match doctrine rule ID %q", decisions302[0].MaterializedID, rule.ID)
	}

	// 3. Run-303 decision is dismissed with DismissedBy pointing to run-302 proposal
	evidenceDir303 := filepath.Join(tmp, "runs", "run-303", "evidence")
	raw303, err := os.ReadFile(filepath.Join(evidenceDir303, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read run-303 decisions: %v", err)
	}
	var decisions303 []proposaltriage.Decision
	if err := json.Unmarshal(raw303, &decisions303); err != nil {
		t.Fatalf("unmarshal run-303 decisions: %v", err)
	}
	if len(decisions303) != 1 {
		t.Fatalf("expected 1 decision for run-303, got %d", len(decisions303))
	}
	if decisions303[0].Action != "dismissed" {
		t.Errorf("run-303 decision action = %q, want 'dismissed'", decisions303[0].Action)
	}
	if decisions303[0].DismissedBy != "run-302-proposal-11223344" {
		t.Errorf("run-303 dismissed_by = %q, want 'run-302-proposal-11223344'", decisions303[0].DismissedBy)
	}

	// 4. Run-304 decision is dismissed with DismissedBy pointing to run-302 proposal
	evidenceDir304 := filepath.Join(tmp, "runs", "run-304", "evidence")
	raw304, err := os.ReadFile(filepath.Join(evidenceDir304, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read run-304 decisions: %v", err)
	}
	var decisions304 []proposaltriage.Decision
	if err := json.Unmarshal(raw304, &decisions304); err != nil {
		t.Fatalf("unmarshal run-304 decisions: %v", err)
	}
	if len(decisions304) != 1 {
		t.Fatalf("expected 1 decision for run-304, got %d", len(decisions304))
	}
	if decisions304[0].Action != "dismissed" {
		t.Errorf("run-304 decision action = %q, want 'dismissed'", decisions304[0].Action)
	}
	if decisions304[0].DismissedBy != "run-302-proposal-11223344" {
		t.Errorf("run-304 dismissed_by = %q, want 'run-302-proposal-11223344'", decisions304[0].DismissedBy)
	}

	// 5. None of the three appear in future default list output
	pendingAfter, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after: %v", err)
	}
	if len(pendingAfter) != 0 {
		ids := make([]string, len(pendingAfter))
		for i, p := range pendingAfter {
			ids[i] = p.Proposal.ID
		}
		t.Fatalf("expected 0 pending proposals after dismiss-group, got %d: %v", len(pendingAfter), ids)
	}
}
