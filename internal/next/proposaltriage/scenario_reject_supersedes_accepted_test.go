package proposaltriage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestScenario_RejectPreviouslyAcceptedProposal_SupersedesMaterializedEntry(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "test-project"
	runID := "run-205"
	proposalID := "run-205-proposal-aabbccdd"
	playbookDir := filepath.Join(tmpDir, "playbook")

	proposals := &reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-205",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		Proposals: []reviewdistiller.Proposal{
			{
				ID:             proposalID,
				Type:           "planner_heuristic",
				Title:          "Split tasks by file boundary",
				ProposedChange: "Decompose tasks so each task touches at most one file",
				Rationale:      "Reduces merge conflicts and simplifies validation",
			},
		},
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	helperCreateRunWithProposals(t, tmpDir, projectID, runID, proposals, nil)

	pendingBefore, err := DiscoverPending(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending: %v", err)
	}
	if len(pendingBefore) != 1 {
		t.Fatalf("expected 1 pending proposal, got %d", len(pendingBefore))
	}

	pp := &PendingProposal{
		Proposal: pendingBefore[0].Proposal,
		RunID:    pendingBefore[0].RunID,
		SpecID:   pendingBefore[0].SpecID,
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	acceptedDecision, err := Promote(pp, "", "", "", nil, pbStore,
		"local", // use local scope
		"",      // evidenceDir
	)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if acceptedDecision.MaterializedID == "" {
		t.Fatal("accepted decision should have a materialized ID")
	}

	evidenceDir := filepath.Join(tmpDir, "runs", runID, "evidence")
	if err := SaveDecisions(evidenceDir, []Decision{*acceptedDecision}); err != nil {
		t.Fatalf("SaveDecisions (accepted): %v", err)
	}

	entriesBefore, err := pbStore.Load()
	if err != nil {
		t.Fatalf("load playbook before reject: %v", err)
	}
	if len(entriesBefore) != 1 {
		t.Fatalf("expected 1 playbook entry, got %d", len(entriesBefore))
	}
	if entriesBefore[0].Status != "active" {
		t.Fatalf("precondition: entry status should be active, got %q", entriesBefore[0].Status)
	}
	if entriesBefore[0].ID != acceptedDecision.MaterializedID {
		t.Fatalf("precondition: entry ID %q should match materialized ID %q", entriesBefore[0].ID, acceptedDecision.MaterializedID)
	}

	allBefore, err := DiscoverAll(tmpDir, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverAll before reject: %v", err)
	}
	if len(allBefore) != 1 {
		t.Fatalf("expected 1 proposal in DiscoverAll, got %d", len(allBefore))
	}
	if allBefore[0].Decision == nil || allBefore[0].Decision.Action != "accepted" {
		t.Fatal("precondition: proposal should have accepted decision")
	}

	rejectionReason := "Turned out to cause worse task splits"

	rejectionDecision, err := Reject(pp, rejectionReason)
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}

	err = RejectAfterAccept(acceptedDecision, rejectionDecision, nil, pbStore)
	if err != nil {
		t.Fatalf("RejectAfterAccept: %v", err)
	}

	if err := SaveDecisions(evidenceDir, []Decision{*rejectionDecision}); err != nil {
		t.Fatalf("SaveDecisions (rejected): %v", err)
	}

	entriesAfter, err := pbStore.Load()
	if err != nil {
		t.Fatalf("load playbook after reject: %v", err)
	}
	if len(entriesAfter) != 1 {
		t.Fatalf("expected 1 playbook entry after reject, got %d", len(entriesAfter))
	}
	if entriesAfter[0].Status != "superseded" {
		t.Errorf("entry status should be superseded, got %q", entriesAfter[0].Status)
	}
	if entriesAfter[0].SupersededBy != rejectionDecision.ProposalID {
		t.Errorf("entry SupersededBy should be %q, got %q", rejectionDecision.ProposalID, entriesAfter[0].SupersededBy)
	}

	activeAfter := playbook.ActiveEntries(entriesAfter)
	if len(activeAfter) != 0 {
		t.Errorf("expected 0 active entries after superseding, got %d", len(activeAfter))
	}

	decisionsData, err := os.ReadFile(filepath.Join(evidenceDir, "proposal-decisions.json"))
	if err != nil {
		t.Fatalf("read proposal-decisions.json: %v", err)
	}
	var savedDecisions []Decision
	if err := json.Unmarshal(decisionsData, &savedDecisions); err != nil {
		t.Fatalf("unmarshal decisions: %v", err)
	}

	var foundRejection *Decision
	for i, d := range savedDecisions {
		if d.ProposalID == proposalID {
			foundRejection = &savedDecisions[i]
			break
		}
	}
	if foundRejection == nil {
		t.Fatal("expected to find decision for proposal in proposal-decisions.json")
	}
	if foundRejection.Action != "rejected" {
		t.Errorf("decision action should be rejected, got %q", foundRejection.Action)
	}
	if foundRejection.Reason != rejectionReason {
		t.Errorf("decision reason should be %q, got %q", rejectionReason, foundRejection.Reason)
	}

	if rejectionDecision.ProposalID != proposalID {
		t.Errorf("rejection decision ProposalID should be %q, got %q", proposalID, rejectionDecision.ProposalID)
	}

	if rejectionDecision.DecidedAt.IsZero() {
		t.Error("rejection decision DecidedAt should be non-zero")
	}
}
