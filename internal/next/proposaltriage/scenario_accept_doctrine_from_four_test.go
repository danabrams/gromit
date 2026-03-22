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

func TestScenario_AcceptDoctrineRuleFromFourProposals(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-201"
	doctrineDir := filepath.Join(tmpDir, "doctrine")
	playbookDir := filepath.Join(tmpDir, "playbook")

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-ui-review",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             "run-201-proposal-a1b2c3d4",
				Type:           "doctrine_rule",
				Title:          "Interactive UI specs must include accessibility scenario checks",
				ProposedChange: "All interactive UI specifications must include at least one accessibility scenario check covering keyboard navigation and screen reader compatibility",
				Rationale:      "Accessibility gaps found during manual review could have been caught earlier with explicit scenario checks",
			},
			{
				ID:             "run-201-proposal-e5f6g7h8",
				Type:           "planner_heuristic",
				Title:          "Decompose UI tasks by component layer",
				ProposedChange: "Split UI implementation tasks into visual, interaction, and accessibility layers",
				Rationale:      "Layer-based decomposition reduces cross-cutting rework",
			},
			{
				ID:             "run-201-proposal-i9j0k1l2",
				Type:           "validation_gap",
				Title:          "Add automated color contrast checks",
				ProposedChange: "Include WCAG contrast ratio validation in UI test suites",
				Rationale:      "Manual contrast checks are error-prone and inconsistent",
			},
			{
				ID:             "run-201-proposal-m3n4o5p6",
				Type:           "doctrine_rule",
				Title:          "UI components must have snapshot tests",
				ProposedChange: "Every new UI component must ship with at least one visual snapshot test",
				Rationale:      "Prevents visual regressions across releases",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC),
	}

	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingBefore, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending before acceptance failed: %v", err)
	}
	if len(pendingBefore) != 4 {
		t.Fatalf("expected 4 pending proposals before acceptance, got %d", len(pendingBefore))
	}

	var targetPP *PendingProposal
	for i := range pendingBefore {
		if pendingBefore[i].Proposal.ID == "run-201-proposal-a1b2c3d4" {
			targetPP = &PendingProposal{
				Proposal: pendingBefore[i].Proposal,
				RunID:    pendingBefore[i].RunID,
				SpecID:   pendingBefore[i].SpecID,
			}
			break
		}
	}
	if targetPP == nil {
		t.Fatal("target proposal run-201-proposal-a1b2c3d4 not found in pending proposals")
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

	loadedDoctrine, err := doctrineStore.Load()
	if err != nil {
		t.Fatalf("failed to load doctrine: %v", err)
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

	if rule.Summary != "Interactive UI specs must include accessibility scenario checks" {
		t.Errorf("rule summary mismatch, got %q", rule.Summary)
	}

	expectedSource := "promoted:run-201-proposal-a1b2c3d4"
	if rule.Source != expectedSource {
		t.Errorf("rule source = %q, want %q", rule.Source, expectedSource)
	}

	if rule.Status != "active" {
		t.Errorf("rule status = %q, want %q", rule.Status, "active")
	}

	decisionsData, err := os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("failed to read proposal-decisions.json: %v", err)
	}

	var savedDecisions []Decision
	if err := json.Unmarshal(decisionsData, &savedDecisions); err != nil {
		t.Fatalf("failed to unmarshal decisions: %v", err)
	}

	if len(savedDecisions) != 1 {
		t.Fatalf("expected 1 decision in proposal-decisions.json, got %d", len(savedDecisions))
	}

	savedDecision := savedDecisions[0]
	if savedDecision.ProposalID != "run-201-proposal-a1b2c3d4" {
		t.Errorf("decision proposal ID = %q, want %q", savedDecision.ProposalID, "run-201-proposal-a1b2c3d4")
	}
	if savedDecision.Action != "accepted" {
		t.Errorf("decision action = %q, want %q", savedDecision.Action, "accepted")
	}
	if savedDecision.MaterializedID != rule.ID {
		t.Errorf("decision MaterializedID = %q, want %q (matching rule ID)", savedDecision.MaterializedID, rule.ID)
	}
	if savedDecision.DecidedAt.IsZero() {
		t.Error("decision DecidedAt should not be zero")
	}

	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance failed: %v", err)
	}
	if len(pendingAfter) != 3 {
		t.Fatalf("expected 3 pending proposals after accepting 1 of 4, got %d", len(pendingAfter))
	}

	for _, p := range pendingAfter {
		if p.Proposal.ID == "run-201-proposal-a1b2c3d4" {
			t.Error("accepted proposal run-201-proposal-a1b2c3d4 should not appear in pending list")
		}
	}

	remainingIDs := make(map[string]bool)
	for _, p := range pendingAfter {
		remainingIDs[p.Proposal.ID] = true
	}
	for _, expectedID := range []string{"run-201-proposal-e5f6g7h8", "run-201-proposal-i9j0k1l2", "run-201-proposal-m3n4o5p6"} {
		if !remainingIDs[expectedID] {
			t.Errorf("expected proposal %q to still be pending", expectedID)
		}
	}

	allAfter, err := DiscoverAll(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverAll after acceptance failed: %v", err)
	}
	if len(allAfter) != 4 {
		t.Fatalf("expected 4 total proposals in DiscoverAll, got %d", len(allAfter))
	}

	for _, ap := range allAfter {
		if ap.Proposal.ID == "run-201-proposal-a1b2c3d4" {
			if ap.Decision == nil {
				t.Error("accepted proposal should have a Decision in DiscoverAll")
			} else if ap.Decision.Action != "accepted" {
				t.Errorf("accepted proposal decision action = %q, want %q", ap.Decision.Action, "accepted")
			}
		} else {
			if ap.Decision != nil {
				t.Errorf("proposal %q should not have a decision yet", ap.Proposal.ID)
			}
		}
	}
}
