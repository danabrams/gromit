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

func TestScenario_AcceptWithDismissGroupClearsSiblings(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "fixture-app"

	// Create 3 runs with identical proposals
	runIDs := []string{"run-302", "run-303", "run-304"}
	proposalIDs := []string{
		"run-302-proposal-exact-match",
		"run-303-proposal-exact-match",
		"run-304-proposal-exact-match",
	}

	// Seed each run with a distillation-proposals.json containing an identical proposal
	identicalType := "doctrine_rule"
	identicalChange := "All API endpoints must include rate-limiting headers"

	for i, runID := range runIDs {
		run := &runstore.RunState{
			RunID:                 runID,
			SpecID:                "spec-api-design",
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
			t.Fatalf("save run %s: %v", runID, err)
		}

		evidenceDir := store.RunEvidenceDir(runID)
		if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
			t.Fatalf("mkdir evidence for %s: %v", runID, err)
		}

		distResult := reviewdistiller.DistillationResult{
			RunID:     runID,
			SpecID:    "spec-api-design",
			Outcome:   "accepted",
			ModelTier: reviewdistiller.TierHigh,
			CreatedAt: time.Date(2026, 3, 21, 10, 20, 0, 0, time.UTC),
			Proposals: []reviewdistiller.Proposal{
				{
					ID:                  proposalIDs[i],
					Type:                identicalType,
					Title:               "Add rate-limiting headers to all API endpoints",
					WhatHappened:        "API endpoints lack rate-limiting protection",
					WhatWasMissing:      "Rate-limiting headers in API responses",
					ProposedChange:      identicalChange,
					Rationale:           "Prevents abuse and improves API resilience",
					Confidence:          "high",
					ConfidenceRationale: "Standard API security practice",
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

	// === Discover and Group ===
	// Discover all pending proposals
	pendingBefore, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending: %v", err)
	}
	if len(pendingBefore) != 3 {
		t.Fatalf("expected 3 pending proposals, got %d", len(pendingBefore))
	}

	// Group by content hash (should create 1 group with 3 proposals)
	groups := proposaltriage.GroupByContentHash(pendingBefore)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group from identical proposals, got %d", len(groups))
	}
	if len(groups[0].Proposals) != 3 {
		t.Fatalf("expected group to contain 3 proposals, got %d", len(groups[0].Proposals))
	}

	// Find run-302 proposal
	var targetPP *proposaltriage.PendingProposal
	for i := range pendingBefore {
		if pendingBefore[i].Proposal.ID == "run-302-proposal-exact-match" {
			targetPP = &pendingBefore[i]
			break
		}
	}
	if targetPP == nil {
		t.Fatal("target proposal run-302-proposal-exact-match not found in pending list")
	}

	// === Invoke: Accept and Dismiss Group ===
	projectDir := filepath.Join(tmp, "projects", projectID)
	doctrineDir := filepath.Join(projectDir, "doctrine")
	playbookDir := filepath.Join(projectDir, "playbook")

	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = doctrineDir
	playbookStore := &playbook.Store{Dir: playbookDir}

	// Promote run-302 proposal
	decision, err := proposaltriage.Promote(
		targetPP,
		"", "", "", // no field overrides
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

	// Save decision to run-302's evidence directory
	evidenceDir302 := filepath.Join(tmp, "runs", "run-302", "evidence")
	if err := proposaltriage.SaveDecisions(evidenceDir302, []proposaltriage.Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions for run-302: %v", err)
	}

	// Dismiss siblings
	dismissedDecisions, err := proposaltriage.DismissSiblings(
		"run-302-proposal-exact-match",
		groups[0],
		tmp,
	)
	if err != nil {
		t.Fatalf("DismissSiblings failed: %v", err)
	}
	if len(dismissedDecisions) != 2 {
		t.Fatalf("expected 2 dismissed decisions, got %d", len(dismissedDecisions))
	}

	// === Assert ===

	// 1. Run-302 proposal is materialized in doctrine
	doctrineStore.Dir = doctrineDir
	loadedDoctrine, err := doctrineStore.Load()
	if err != nil {
		t.Fatalf("load doctrine: %v", err)
	}
	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("expected 1 doctrine rule materialized, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]
	if rule.Summary != "Add rate-limiting headers to all API endpoints" {
		t.Errorf("rule summary = %q, want proposal title", rule.Summary)
	}

	// 2. Run-302's proposal-decisions.json contains accepted decision
	decisionsRaw302, err := os.ReadFile(filepath.Join(evidenceDir302, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read run-302 proposal-decisions.json: %v", err)
	}

	var decisions302 []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw302, &decisions302); err != nil {
		t.Fatalf("unmarshal run-302 decisions: %v", err)
	}

	if len(decisions302) != 1 {
		t.Fatalf("expected 1 decision for run-302, got %d", len(decisions302))
	}
	if decisions302[0].Action != "accepted" {
		t.Errorf("run-302 decision action = %q, want 'accepted'", decisions302[0].Action)
	}
	if decisions302[0].MaterializedID != rule.ID {
		t.Errorf("run-302 decision materialized_id should match rule ID")
	}

	// 3. Run-303's proposal-decisions.json contains dismissed decision with DismissedBy pointing to run-302 proposal
	evidenceDir303 := filepath.Join(tmp, "runs", "run-303", "evidence")
	decisionsRaw303, err := os.ReadFile(filepath.Join(evidenceDir303, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read run-303 proposal-decisions.json: %v", err)
	}

	var decisions303 []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw303, &decisions303); err != nil {
		t.Fatalf("unmarshal run-303 decisions: %v", err)
	}

	if len(decisions303) != 1 {
		t.Fatalf("expected 1 decision for run-303, got %d", len(decisions303))
	}
	if decisions303[0].Action != "dismissed" {
		t.Errorf("run-303 decision action = %q, want 'dismissed'", decisions303[0].Action)
	}
	if decisions303[0].DismissedBy != "run-302-proposal-exact-match" {
		t.Errorf("run-303 decision dismissed_by = %q, want 'run-302-proposal-exact-match'", decisions303[0].DismissedBy)
	}

	// 4. Run-304's proposal-decisions.json contains dismissed decision with DismissedBy pointing to run-302 proposal
	evidenceDir304 := filepath.Join(tmp, "runs", "run-304", "evidence")
	decisionsRaw304, err := os.ReadFile(filepath.Join(evidenceDir304, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read run-304 proposal-decisions.json: %v", err)
	}

	var decisions304 []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw304, &decisions304); err != nil {
		t.Fatalf("unmarshal run-304 decisions: %v", err)
	}

	if len(decisions304) != 1 {
		t.Fatalf("expected 1 decision for run-304, got %d", len(decisions304))
	}
	if decisions304[0].Action != "dismissed" {
		t.Errorf("run-304 decision action = %q, want 'dismissed'", decisions304[0].Action)
	}
	if decisions304[0].DismissedBy != "run-302-proposal-exact-match" {
		t.Errorf("run-304 decision dismissed_by = %q, want 'run-302-proposal-exact-match'", decisions304[0].DismissedBy)
	}

	// 5. All 3 proposals no longer appear in pending list
	pendingAfter, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after dismiss group: %v", err)
	}

	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after dismiss group, got %d", len(pendingAfter))
	}
}
