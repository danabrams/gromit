package proposaltriage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestScenario_AcceptDoctrineRuleIntoProjectStore(t *testing.T) {
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
				ProposedChange: "All interactive UI specifications must include at least one accessibility scenario",
				Rationale:      "Ensures accessibility is considered from the spec phase, not as an afterthought",
				Confidence:     "high",
			},
			{
				ID:             "run-201-proposal-bbbb1111",
				Type:           "validation_gap",
				Title:          "Missing contrast ratio validation",
				ProposedChange: "Add automated contrast ratio checks",
				Rationale:      "Catches WCAG violations early",
				Confidence:     "medium",
			},
			{
				ID:             "run-201-proposal-cccc2222",
				Type:           "planner_heuristic",
				Title:          "Decompose UI tasks by component",
				ProposedChange: "Split UI implementation by component boundaries",
				Rationale:      "Reduces merge conflicts and improves parallel work",
				Confidence:     "high",
			},
			{
				ID:             "run-201-proposal-dddd3333",
				Type:           "refinement_guidance",
				Title:          "Include screen reader testing notes",
				ProposedChange: "Add screen reader testing guidance to review checklist",
				Rationale:      "Manual screen reader testing catches issues automated tools miss",
				Confidence:     "medium",
			},
		},
		CreatedAt: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
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
	for _, pp := range pendingBefore {
		if pp.Proposal.ID == "run-201-proposal-a1b2c3d4" {
			targetPP = &PendingProposal{
				Proposal: pp.Proposal,
				RunID:    pp.RunID,
				SpecID:   pp.SpecID,
			}
			break
		}
	}
	if targetPP == nil {
		t.Fatal("target proposal run-201-proposal-a1b2c3d4 not found in pending list")
	}

	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = doctrineDir
	playbookStore := &playbook.Store{Dir: playbookDir}
	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")

	decision, err := Promote(targetPP, "", "", "", doctrineStore, playbookStore)
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

	var doc doctrine.Doctrine
	if err := json.Unmarshal(doctrineData, &doc); err != nil {
		t.Fatalf("unmarshal doctrine: %v", err)
	}

	if len(doc.Rules) != 1 {
		t.Fatalf("expected 1 rule in doctrine, got %d", len(doc.Rules))
	}

	rule := doc.Rules[0]

	if !isPromotedID(rule.ID) {
		t.Errorf("rule ID should start with 'promoted-', got %q", rule.ID)
	}

	if rule.Summary != "Interactive UI specs must include accessibility scenario checks" {
		t.Errorf("rule summary = %q, want proposal title", rule.Summary)
	}

	expectedSource := "promoted:run-201-proposal-a1b2c3d4"
	if rule.Source != expectedSource {
		t.Errorf("rule source = %q, want %q", rule.Source, expectedSource)
	}

	if rule.Status != "active" {
		t.Errorf("rule status = %q, want 'active'", rule.Status)
	}

	if decision.MaterializedID != rule.ID {
		t.Errorf("decision MaterializedID %q != rule ID %q", decision.MaterializedID, rule.ID)
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
	if d.ProposalID != "run-201-proposal-a1b2c3d4" {
		t.Errorf("decision proposal ID = %q, want run-201-proposal-a1b2c3d4", d.ProposalID)
	}
	if d.Action != "accepted" {
		t.Errorf("decision action = %q, want 'accepted'", d.Action)
	}
	if d.MaterializedID != rule.ID {
		t.Errorf("decision materialized ID = %q, want %q", d.MaterializedID, rule.ID)
	}

	pendingAfter, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after acceptance: %v", err)
	}

	if len(pendingAfter) != 3 {
		t.Fatalf("expected 3 pending proposals after acceptance, got %d", len(pendingAfter))
	}

	for _, pp := range pendingAfter {
		if pp.Proposal.ID == "run-201-proposal-a1b2c3d4" {
			t.Error("accepted proposal should not appear in pending list")
		}
	}

	remainingIDs := make(map[string]bool)
	for _, pp := range pendingAfter {
		remainingIDs[pp.Proposal.ID] = true
	}
	for _, expectedID := range []string{
		"run-201-proposal-bbbb1111",
		"run-201-proposal-cccc2222",
		"run-201-proposal-dddd3333",
	} {
		if !remainingIDs[expectedID] {
			t.Errorf("expected %q to still be pending", expectedID)
		}
	}
}
