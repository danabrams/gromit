package proposaltriage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestScenario_AcceptDoctrineRuleIntoProjectLocalStore(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-201"
	doctrineDir := filepath.Join(tmpDir, "doctrine")
	playbookDir := filepath.Join(tmpDir, "playbook")

	targetProposalID := "run-201-proposal-a1b2c3d4"
	targetTitle := "Interactive UI specs must include accessibility scenario checks"

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-ui-enhancements",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  targetProposalID,
				Type:                "doctrine_rule",
				Title:               targetTitle,
				WhatHappened:        "UI spec was implemented without accessibility checks",
				WhatWasMissing:      "No accessibility scenario validation in the spec",
				ProposedChange:      "Every interactive UI spec must include at least one accessibility scenario check",
				Rationale:           "Ensures accessibility is a first-class concern in all UI work",
				Confidence:          "high",
				ConfidenceRationale: "Observed repeatedly across UI-focused specs",
				EvidenceReferences:  []string{"review-outcome.json"},
			},
			{
				ID:                  "run-201-proposal-b2c3d4e5",
				Type:                "validation_gap",
				Title:               "Add visual regression tests for responsive layouts",
				WhatHappened:        "Responsive layout passed manual review but had subtle breakpoints",
				WhatWasMissing:      "Automated visual regression for breakpoint transitions",
				ProposedChange:      "Include screenshot comparison tests for each breakpoint",
				Rationale:           "Catches layout regressions missed by functional tests",
				Confidence:          "medium",
				ConfidenceRationale: "Visual regressions are hard to detect without screenshots",
				EvidenceReferences:  []string{"review-outcome.json"},
			},
			{
				ID:                  "run-201-proposal-c3d4e5f6",
				Type:                "planner_heuristic",
				Title:               "Decompose UI tasks by component boundary",
				WhatHappened:        "UI implementation was split by feature instead of component",
				WhatWasMissing:      "Guidance on component-based task decomposition for UI work",
				ProposedChange:      "Plan UI tasks along component boundaries to reduce cross-cutting changes",
				Rationale:           "Component-scoped tasks are easier to review and test",
				Confidence:          "high",
				ConfidenceRationale: "Component boundaries align with natural code boundaries",
				EvidenceReferences:  []string{"plan.md"},
			},
			{
				ID:                  "run-201-proposal-d4e5f6g7",
				Type:                "refinement_guidance",
				Title:               "Clarify hover state expectations in interactive specs",
				WhatHappened:        "Hover states were ambiguous in the spec leading to inconsistent implementation",
				WhatWasMissing:      "Explicit hover state definitions in spec language",
				ProposedChange:      "Require explicit hover/focus/active state descriptions in interactive UI specs",
				Rationale:           "Reduces ambiguity and implementation rework",
				Confidence:          "medium",
				ConfidenceRationale: "Ambiguous specs lead to rework cycles",
				EvidenceReferences:  []string{"review-outcome.json"},
			},
		},
		CreatedAt: time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingBefore, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before acceptance: %v", err)
	}
	if len(pendingBefore) != 4 {
		t.Fatalf("expected 4 pending proposals before acceptance, got %d", len(pendingBefore))
	}

	var targetPP *PendingProposal
	for i := range pendingBefore {
		if pendingBefore[i].Proposal.ID == targetProposalID {
			targetPP = &PendingProposal{
				Proposal: pendingBefore[i].Proposal,
				RunID:    pendingBefore[i].RunID,
				SpecID:   pendingBefore[i].SpecID,
			}
			break
		}
	}
	if targetPP == nil {
		t.Fatal("target proposal not found in pending proposals")
	}

	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = doctrineDir
	playbookStore := &playbook.Store{Dir: playbookDir}
	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")

	decision, err := Promote(
		targetPP,
		"", // no title override
		"", // no change override
		"", // no rationale override
		doctrineStore,
		playbookStore,
	)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if decision == nil {
		t.Fatal("Accept returned nil decision")
	}

	if err := SaveDecisions(evidenceDir, []Decision{*decision}); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	doctrineData, err := os.ReadFile(filepath.Join(doctrineDir, "rules.json"))
	if err != nil {
		t.Fatalf("read doctrine/rules.json: %v", err)
	}

	var loadedDoctrine doctrine.Doctrine
	if err := json.Unmarshal(doctrineData, &loadedDoctrine); err != nil {
		t.Fatalf("unmarshal doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("expected 1 rule in doctrine, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]

	if !strings.HasPrefix(rule.ID, "promoted-") {
		t.Errorf("rule ID should start with 'promoted-', got %q", rule.ID)
	}

	if rule.ID != decision.MaterializedID {
		t.Errorf("rule ID %q does not match decision MaterializedID %q", rule.ID, decision.MaterializedID)
	}

	if rule.Summary != targetTitle {
		t.Errorf("rule summary: got %q, want %q", rule.Summary, targetTitle)
	}

	expectedSource := "promoted:" + targetProposalID
	if rule.Source != expectedSource {
		t.Errorf("rule source: got %q, want %q", rule.Source, expectedSource)
	}

	if rule.Status != "active" {
		t.Errorf("rule status: got %q, want %q", rule.Status, "active")
	}

	decisionsData, err := os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read proposal-decisions.json: %v", err)
	}

	var savedDecisions []Decision
	if err := json.Unmarshal(decisionsData, &savedDecisions); err != nil {
		t.Fatalf("unmarshal decisions: %v", err)
	}

	if len(savedDecisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(savedDecisions))
	}

	d := savedDecisions[0]
	if d.ProposalID != targetProposalID {
		t.Errorf("decision ProposalID: got %q, want %q", d.ProposalID, targetProposalID)
	}
	if d.Action != "accepted" {
		t.Errorf("decision Action: got %q, want %q", d.Action, "accepted")
	}
	if d.MaterializedID != rule.ID {
		t.Errorf("decision MaterializedID: got %q, want rule ID %q", d.MaterializedID, rule.ID)
	}
	if d.DecidedAt.IsZero() {
		t.Error("decision DecidedAt should not be zero")
	}
	if d.DuplicateOf != "" {
		t.Errorf("decision DuplicateOf should be empty for new proposal, got %q", d.DuplicateOf)
	}

	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance: %v", err)
	}

	if len(pendingAfter) != 3 {
		t.Fatalf("expected 3 pending proposals after acceptance, got %d", len(pendingAfter))
	}

	for _, p := range pendingAfter {
		if p.Proposal.ID == targetProposalID {
			t.Errorf("accepted proposal %q should not appear in pending list", targetProposalID)
		}
	}

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
			t.Errorf("expected proposal %q to still be pending", expectedID)
		}
	}

	allAfter, err := DiscoverAll(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverAll after acceptance: %v", err)
	}

	if len(allAfter) != 4 {
		t.Fatalf("expected 4 total proposals in DiscoverAll, got %d", len(allAfter))
	}

	var foundAccepted bool
	for _, ap := range allAfter {
		if ap.Proposal.ID == targetProposalID {
			foundAccepted = true
			if ap.Decision == nil {
				t.Error("accepted proposal should have a decision in DiscoverAll")
			} else if ap.Decision.Action != "accepted" {
				t.Errorf("accepted proposal decision action: got %q, want %q", ap.Decision.Action, "accepted")
			}
		}
	}
	if !foundAccepted {
		t.Error("accepted proposal should still appear in DiscoverAll")
	}
}
