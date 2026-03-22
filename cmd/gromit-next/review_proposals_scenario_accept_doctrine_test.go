package main

import (
	"encoding/json"
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

func TestScenario_AcceptDoctrineRuleProposalIntoProjectLocalStore(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "fixture-app"
	runID := "run-201"

	run := &runstore.RunState{
		RunID:                 runID,
		SpecID:                "spec-ui-redesign",
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

	evidenceDir := store.RunEvidenceDir(runID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Seed distillation-proposals.json with 4 proposals, one doctrine_rule target
	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-ui-redesign",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 10, 20, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  "run-201-proposal-a1b2c3d4",
				Type:                "doctrine_rule",
				Title:               "Interactive UI specs must include accessibility scenario checks",
				WhatHappened:        "UI redesign spec passed review without accessibility scenario validation",
				WhatWasMissing:      "Accessibility scenario checks for interactive components",
				ProposedChange:      "Require accessibility scenarios in all interactive UI specs",
				Rationale:           "Ensures inclusive design and WCAG compliance",
				Confidence:          "high",
				ConfidenceRationale: "Pattern observed across multiple UI spec reviews",
				EvidenceReferences:  []string{"review-outcome.json"},
			},
			{
				ID:                  "run-201-proposal-b2c3d4e5",
				Type:                "planner_heuristic",
				Title:               "Decompose UI work by component boundary",
				WhatHappened:        "Large UI tasks required multiple fix cycles",
				WhatWasMissing:      "Component-level task decomposition",
				ProposedChange:      "Split UI implementation tasks along component boundaries",
				Rationale:           "Reduces fix cycle churn",
				Confidence:          "medium",
				ConfidenceRationale: "Observed in 2 UI-heavy runs",
				EvidenceReferences:  []string{"review-outcome.json"},
			},
			{
				ID:                  "run-201-proposal-c3d4e5f6",
				Type:                "validation_gap",
				Title:               "Missing contrast ratio validation",
				WhatHappened:        "Low contrast text elements passed automated validation",
				WhatWasMissing:      "Contrast ratio checks against WCAG AA standards",
				ProposedChange:      "Add contrast ratio validation step to UI spec validation",
				Rationale:           "WCAG compliance requirement",
				Confidence:          "high",
				ConfidenceRationale: "Accessibility audits consistently flag this gap",
				EvidenceReferences:  []string{"validation.json"},
			},
			{
				ID:                  "run-201-proposal-d4e5f6g7",
				Type:                "refinement_guidance",
				Title:               "Prefer semantic HTML elements over generic divs",
				WhatHappened:        "Div-heavy markup flagged in code review",
				WhatWasMissing:      "Guidance on semantic element usage",
				ProposedChange:      "Use semantic HTML elements where applicable for accessibility",
				Rationale:           "Improves screen reader compatibility",
				Confidence:          "medium",
				ConfidenceRationale: "Best practice alignment with accessibility standards",
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

	// Set up project cell paths
	projectDir := filepath.Join(tmp, "projects", projectID)
	doctrineDir := filepath.Join(projectDir, "doctrine")
	playbookDir := filepath.Join(projectDir, "playbook")

	// Verify precondition: all 4 proposals appear as pending
	pendingBefore, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before accept: %v", err)
	}
	if len(pendingBefore) != 4 {
		t.Fatalf("expected 4 pending proposals before accept, got %d", len(pendingBefore))
	}

	// Find the target doctrine_rule proposal
	var targetPP *proposaltriage.PendingProposal
	for i := range pendingBefore {
		if pendingBefore[i].Proposal.ID == "run-201-proposal-a1b2c3d4" {
			targetPP = &pendingBefore[i]
			break
		}
	}
	if targetPP == nil {
		t.Fatal("target proposal run-201-proposal-a1b2c3d4 not found in pending list")
	}

	// === Invoke ===
	doctrineStore := doctrine.NewFSStore()
	playbookStore := &playbook.Store{Dir: playbookDir}

	decision, err := proposaltriage.Accept(
		targetPP,
		"", "", "", // no field overrides — use proposal defaults
		doctrineStore,
		playbookStore,
		doctrineDir,
		playbookDir,
		evidenceDir,
	)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if decision == nil {
		t.Fatal("Accept returned nil decision")
	}

	// Save decision to run-201's evidence directory
	if err := proposaltriage.SaveDecisions(evidenceDir, []proposaltriage.Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// === Assert ===

	// 1. New rule appears in project cell's doctrine/rules.json
	loadedDoctrine, err := doctrineStore.Load(doctrineDir)
	if err != nil {
		t.Fatalf("load doctrine: %v", err)
	}
	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("expected 1 doctrine rule, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]

	// Rule ID is promoted-<hash>
	if !strings.HasPrefix(rule.ID, "promoted-") {
		t.Errorf("rule ID should start with 'promoted-', got %q", rule.ID)
	}

	// Rule summary is the proposal's title
	if rule.Summary != "Interactive UI specs must include accessibility scenario checks" {
		t.Errorf("rule summary = %q, want proposal title", rule.Summary)
	}

	// Rule source is promoted:<proposal-id>
	expectedSource := "promoted:run-201-proposal-a1b2c3d4"
	if rule.Source != expectedSource {
		t.Errorf("rule source = %q, want %q", rule.Source, expectedSource)
	}

	// Rule is active
	if rule.Status != "active" {
		t.Errorf("rule status = %q, want 'active'", rule.Status)
	}

	// 2. proposal-decisions.json in run-201's evidence directory contains accepted decision
	decisionsRaw, err := os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read proposal-decisions.json: %v", err)
	}

	var savedDecisions []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw, &savedDecisions); err != nil {
		t.Fatalf("unmarshal decisions: %v", err)
	}

	if len(savedDecisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(savedDecisions))
	}

	savedDecision := savedDecisions[0]
	if savedDecision.ProposalID != "run-201-proposal-a1b2c3d4" {
		t.Errorf("decision proposal_id = %q, want 'run-201-proposal-a1b2c3d4'", savedDecision.ProposalID)
	}
	if savedDecision.Action != "accepted" {
		t.Errorf("decision action = %q, want 'accepted'", savedDecision.Action)
	}
	if savedDecision.MaterializedID != rule.ID {
		t.Errorf("decision materialized_id = %q, want %q (matching doctrine rule)", savedDecision.MaterializedID, rule.ID)
	}
	if savedDecision.DecidedAt.IsZero() {
		t.Error("decision decided_at should not be zero")
	}

	// 3. The accepted proposal no longer appears in review proposals list (pending)
	pendingAfter, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after accept: %v", err)
	}

	// 3 of 4 proposals should remain pending
	if len(pendingAfter) != 3 {
		t.Fatalf("expected 3 pending proposals after accept, got %d", len(pendingAfter))
	}

	for _, p := range pendingAfter {
		if p.Proposal.ID == "run-201-proposal-a1b2c3d4" {
			t.Error("accepted proposal should not appear in pending list")
		}
	}

	// Verify the remaining 3 proposals are the non-accepted ones
	remainingIDs := make(map[string]bool)
	for _, p := range pendingAfter {
		remainingIDs[p.Proposal.ID] = true
	}
	for _, expectedID := range []string{
		"run-201-proposal-b2c3d4e5",
		"run-201-proposal-c3d4e5f6",
		"run-201-proposal-d4e5f6g7",
	} {
		if !remainingIDs[expectedID] {
			t.Errorf("expected %q to remain pending after accept", expectedID)
		}
	}
}
