package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"testing"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/proposaltriage"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_AcceptCLIWithGlobalScopePlaybook(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "fixture-app"
	runID := "run-202"

	run := &runstore.RunState{
		RunID:                 runID,
		SpecID:                "spec-planning-decomposition",
		ProjectID:             projectID,
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 21, 14, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 21, 14, 20, 0, 0, time.UTC),
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

	// Seed distillation-proposals.json with a planner_heuristic proposal
	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-planning-decomposition",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 14, 25, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  "run-202-proposal-x1y2z3a4",
				Type:                "planner_heuristic",
				Title:               "Decompose large tasks by component boundary",
				WhatHappened:        "Large planning tasks required multiple fix cycles to complete",
				WhatWasMissing:      "Component-level task decomposition strategy",
				ProposedChange:      "When breaking down large features, split work along component boundaries to reduce iteration cycles",
				Rationale:           "Smaller, focused tasks reduce coordination overhead and improve planning accuracy",
				Confidence:          "high",
				ConfidenceRationale: "Pattern observed consistently across multiple planning-heavy runs",
				EvidenceReferences:  []string{"planning-review.json"},
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

	// Set up global store paths
	globalPlaybookDir := filepath.Join(tmp, "global", "playbook")

	// Verify precondition: proposal appears as pending
	pendingBefore, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before accept: %v", err)
	}
	if len(pendingBefore) != 1 {
		t.Fatalf("expected 1 pending proposal before accept, got %d", len(pendingBefore))
	}

	// Find the target planner_heuristic proposal
	var targetPP *proposaltriage.PendingProposal
	for i := range pendingBefore {
		if pendingBefore[i].Proposal.ID == "run-202-proposal-x1y2z3a4" {
			targetPP = &pendingBefore[i]
			break
		}
	}
	if targetPP == nil {
		t.Fatal("target proposal run-202-proposal-x1y2z3a4 not found in pending list")
	}

	// === Invoke ===
	playbookStore := &playbook.Store{Dir: globalPlaybookDir}

	decision, err := proposaltriage.Promote(
		targetPP,
		"", "", "", // no field overrides — use proposal defaults
		nil, // no doctrine store needed for planner_heuristic
		playbookStore,
		"global", // use global scope
	)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if decision == nil {
		t.Fatal("Accept returned nil decision")
	}

	// Save decision to run-202's evidence directory
	if err := proposaltriage.SaveDecisions(evidenceDir, []proposaltriage.Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// === Assert ===

	// 1. New entry appears in global playbook/entries.json
	loadedEntries, err := playbookStore.Load()
	if err != nil {
		t.Fatalf("load playbook entries: %v", err)
	}
	if len(loadedEntries) != 1 {
		t.Fatalf("expected 1 playbook entry, got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]

	// Entry ID is deterministic based on type and content
	expectedID := playbook.ComputeID("planner_heuristic", entry.Content)
	if entry.ID != expectedID {
		t.Errorf("entry ID %q doesn't match computed playbook ID %q", entry.ID, expectedID)
	}

	// Entry type matches proposal type
	if entry.Type != "planner_heuristic" {
		t.Errorf("entry type = %q, want 'planner_heuristic'", entry.Type)
	}

	// Entry title is the proposal's title
	if entry.Title != "Decompose large tasks by component boundary" {
		t.Errorf("entry title = %q, want proposal title", entry.Title)
	}

	// Entry content is the proposal's proposed change
	if entry.Content != "When breaking down large features, split work along component boundaries to reduce iteration cycles" {
		t.Errorf("entry content mismatch")
	}

	// Entry rationale is the proposal's rationale
	if entry.Rationale != "Smaller, focused tasks reduce coordination overhead and improve planning accuracy" {
		t.Errorf("entry rationale mismatch")
	}

	// Entry source is the proposal ID
	if entry.SourceProposalID != "run-202-proposal-x1y2z3a4" {
		t.Errorf("entry source_proposal_id = %q, want 'run-202-proposal-x1y2z3a4'", entry.SourceProposalID)
	}

	// Entry run ID
	if entry.SourceRunID != runID {
		t.Errorf("entry source_run_id = %q, want %q", entry.SourceRunID, runID)
	}

	// Entry spec ID
	if entry.SourceSpecID != "spec-planning-decomposition" {
		t.Errorf("entry source_spec_id = %q, want 'spec-planning-decomposition'", entry.SourceSpecID)
	}

	// Entry is active
	if entry.Status != "active" {
		t.Errorf("entry status = %q, want 'active'", entry.Status)
	}

	// **KEY ASSERTION**: Entry scope is "global"
	if entry.Scope != "global" {
		t.Errorf("entry scope = %q, want 'global'", entry.Scope)
	}

	// Entry timestamp is recent
	if entry.CreatedAt.IsZero() {
		t.Error("entry created_at should not be zero")
	}

	// 2. proposal-decisions.json in run-202's evidence directory contains accepted decision
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
	if savedDecision.ProposalID != "run-202-proposal-x1y2z3a4" {
		t.Errorf("decision proposal_id = %q, want 'run-202-proposal-x1y2z3a4'", savedDecision.ProposalID)
	}
	if savedDecision.Action != "accepted" {
		t.Errorf("decision action = %q, want 'accepted'", savedDecision.Action)
	}
	if savedDecision.MaterializedID != entry.ID {
		t.Errorf("decision materialized_id = %q, want %q (matching playbook entry)", savedDecision.MaterializedID, entry.ID)
	}
	if savedDecision.DecidedAt.IsZero() {
		t.Error("decision decided_at should not be zero")
	}

	// 3. The accepted proposal no longer appears in review proposals list (pending)
	pendingAfter, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after accept: %v", err)
	}

	// 0 of 1 proposals should remain pending
	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after accept, got %d", len(pendingAfter))
	}

	// Verify entry file location
	entriesPath := filepath.Join(globalPlaybookDir, "entries.json")
	if _, err := os.Stat(entriesPath); err != nil {
		t.Fatalf("entries.json should exist at %s: %v", entriesPath, err)
	}
}

func TestScenario_AcceptCLIWithLocalScopeDefault(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "fixture-app"
	runID := "run-203"

	run := &runstore.RunState{
		RunID:                 runID,
		SpecID:                "spec-coding-standards",
		ProjectID:             projectID,
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 21, 15, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 21, 15, 15, 0, 0, time.UTC),
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

	// Seed distillation-proposals.json with a doctrine_rule proposal
	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-coding-standards",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 15, 20, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  "run-203-proposal-p1q2r3s4",
				Type:                "doctrine_rule",
				Title:               "Use explicit error handling in error returns",
				WhatHappened:        "Unclear error messages during debugging caused delays",
				WhatWasMissing:      "Standard for explicit error context in return values",
				ProposedChange:      "Always include context when returning errors from helper functions",
				Rationale:           "Explicit error handling reduces debugging time and improves code clarity",
				Confidence:          "high",
				ConfidenceRationale: "Pattern observed consistently across multiple debugging sessions",
				EvidenceReferences:  []string{"error-analysis.json"},
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

	// Set up project-local paths
	projectDir := filepath.Join(tmp, "projects", projectID)
	doctrineDir := filepath.Join(projectDir, "doctrine")

	// Verify precondition: proposal appears as pending
	pendingBefore, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before accept: %v", err)
	}
	if len(pendingBefore) != 1 {
		t.Fatalf("expected 1 pending proposal before accept, got %d", len(pendingBefore))
	}

	// Find the target doctrine_rule proposal
	var targetPP *proposaltriage.PendingProposal
	for i := range pendingBefore {
		if pendingBefore[i].Proposal.ID == "run-203-proposal-p1q2r3s4" {
			targetPP = &pendingBefore[i]
			break
		}
	}
	if targetPP == nil {
		t.Fatal("target proposal run-203-proposal-p1q2r3s4 not found in pending list")
	}

	// === Invoke ===
	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = doctrineDir

	decision, err := proposaltriage.Promote(
		targetPP,
		"", "", "", // no field overrides — use proposal defaults
		doctrineStore,
		nil, // no playbook store needed for doctrine_rule
		"",  // empty scope — should default to "*"
	)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if decision == nil {
		t.Fatal("Accept returned nil decision")
	}

	// Save decision to run-203's evidence directory
	if err := proposaltriage.SaveDecisions(evidenceDir, []proposaltriage.Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// === Assert ===

	// 1. New rule appears in project doctrine/rules.json
	loadedDoctrine, err := doctrineStore.Load()
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
	if rule.Summary != "Use explicit error handling in error returns" {
		t.Errorf("rule summary = %q, want proposal title", rule.Summary)
	}

	// Rule source is promoted:<proposal-id>
	expectedSource := "promoted:run-203-proposal-p1q2r3s4"
	if rule.Source != expectedSource {
		t.Errorf("rule source = %q, want %q", rule.Source, expectedSource)
	}

	// **KEY ASSERTION**: Rule scope defaults to "*" when no scope is specified
	if rule.Scope != "*" {
		t.Errorf("rule scope = %q, want '*' (default when no scope specified)", rule.Scope)
	}

	// Rule is active
	if rule.Status != "active" {
		t.Errorf("rule status = %q, want 'active'", rule.Status)
	}

	// Rule timestamp is recent
	if rule.CreatedAt.IsZero() {
		t.Error("rule created_at should not be zero")
	}

	// 2. proposal-decisions.json in run-203's evidence directory contains accepted decision
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
	if savedDecision.ProposalID != "run-203-proposal-p1q2r3s4" {
		t.Errorf("decision proposal_id = %q, want 'run-203-proposal-p1q2r3s4'", savedDecision.ProposalID)
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

	// 0 of 1 proposals should remain pending
	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after accept, got %d", len(pendingAfter))
	}

	// Verify rule file location
	rulesPath := filepath.Join(doctrineDir, "rules.json")
	if _, err := os.Stat(rulesPath); err != nil {
		t.Fatalf("rules.json should exist at %s: %v", rulesPath, err)
	}
}

func TestScenario_AcceptCLIRejectsInvalidScope(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "fixture-app"
	runID := "run-204"

	run := &runstore.RunState{
		RunID:                 runID,
		SpecID:                "spec-error-handling",
		ProjectID:             projectID,
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 21, 16, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 21, 16, 15, 0, 0, time.UTC),
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

	// Seed distillation-proposals.json with a planner_heuristic proposal
	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-error-handling",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		CreatedAt: time.Date(2026, 3, 21, 16, 20, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  "run-204-proposal-m1n2o3p4",
				Type:                "planner_heuristic",
				Title:               "Validate scope parameter in proposal acceptance",
				WhatHappened:        "Proposal acceptance accepted any scope value without validation",
				WhatWasMissing:      "Scope validation to restrict to valid values",
				ProposedChange:      "Validate scope parameter is either 'local' or 'global' before accepting proposals",
				Rationale:           "Restricting scope to valid values prevents configuration errors and ensures consistent behavior",
				Confidence:          "high",
				ConfidenceRationale: "Invalid scope values could lead to unexpected proposal storage locations",
				EvidenceReferences:  []string{"proposal-analysis.json"},
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

	// Set up global store paths
	globalPlaybookDir := filepath.Join(tmp, "global", "playbook")

	// Verify precondition: proposal appears as pending
	pendingBefore, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before accept: %v", err)
	}
	if len(pendingBefore) != 1 {
		t.Fatalf("expected 1 pending proposal before accept, got %d", len(pendingBefore))
	}

	// Find the target proposal
	var targetPP *proposaltriage.PendingProposal
	for i := range pendingBefore {
		if pendingBefore[i].Proposal.ID == "run-204-proposal-m1n2o3p4" {
			targetPP = &pendingBefore[i]
			break
		}
	}
	if targetPP == nil {
		t.Fatal("target proposal run-204-proposal-m1n2o3p4 not found in pending list")
	}

	// === Invoke with invalid scope ===
	playbookStore := &playbook.Store{Dir: globalPlaybookDir}

	// Attempt to promote with an invalid scope value
	decision, err := proposaltriage.Promote(
		targetPP,
		"", "", "", // no field overrides
		nil, // no doctrine store needed for planner_heuristic
		playbookStore,
		"invalid-value", // invalid scope — should be "local" or "global"
	)

	// === Assert ===

	// 1. Decision should be nil when scope validation fails
	if decision != nil {
		t.Fatal("Promote should return nil decision when scope is invalid")
	}

	// 2. Error should be returned about invalid scope
	if err == nil {
		t.Fatal("Promote should return an error for invalid scope")
	}

	// 3. Error message should mention the invalid scope
	if !strings.Contains(err.Error(), "invalid scope") {
		t.Errorf("error message should mention 'invalid scope', got: %v", err)
	}

	if !strings.Contains(err.Error(), "invalid-value") {
		t.Errorf("error message should mention the scope value 'invalid-value', got: %v", err)
	}

	// 4. No playbook entry should have been created
	loadedEntries, err := playbookStore.Load()
	if err != nil {
		// It's okay if the file doesn't exist yet (no entries saved)
		if !strings.Contains(err.Error(), "no such file") {
			t.Fatalf("unexpected error loading playbook: %v", err)
		}
		loadedEntries = []playbook.Entry{}
	}

	if len(loadedEntries) != 0 {
		t.Errorf("expected 0 playbook entries after invalid scope rejection, got %d", len(loadedEntries))
	}
}
