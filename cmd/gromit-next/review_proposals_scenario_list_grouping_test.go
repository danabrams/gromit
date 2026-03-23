package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/proposaltriage"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ListGroupsProposalsByDeterministicHashAndLLMClustering(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "fixture-app"

	// Identical content for proposals 1 and 2
	identicalType := "doctrine_rule"
	identicalChange := "All API endpoints must validate authentication tokens before processing"

	// Semantically similar content for proposals 3 and 4
	similarChange1 := "Ensure all database queries use parameterized statements to prevent injection"
	similarChange2 := "Use parameterized queries for all database operations to avoid SQL injection"

	// Unique content for proposals 5 and 6
	uniqueChange1 := "Implement retry logic with exponential backoff for external service calls"
	uniqueChange2 := "Add structured logging with correlation IDs throughout the request pipeline"

	// Run 1: First identical proposal
	run1ID := "run-601"
	seedRunWithProposals(t, store, tmp, projectID, run1ID, "spec-auth", []reviewdistiller.Proposal{
		{
			ID:             "run-601-proposal-aaaa1111",
			Type:           identicalType,
			Title:          "Auth Token Validation Rule",
			ProposedChange: identicalChange,
			Rationale:      "Prevent unauthorized access",
			Confidence:     "high",
		},
	})

	// Run 2: Second identical proposal + first unique proposal
	run2ID := "run-602"
	seedRunWithProposals(t, store, tmp, projectID, run2ID, "spec-auth", []reviewdistiller.Proposal{
		{
			ID:             "run-602-proposal-bbbb2222",
			Type:           identicalType,
			Title:          "Auth Token Validation Rule",
			ProposedChange: identicalChange,
			Rationale:      "Security best practice",
			Confidence:     "high",
		},
		{
			ID:             "run-602-proposal-eeee5555",
			Type:           "planner_heuristic",
			Title:          "Retry with Backoff",
			ProposedChange: uniqueChange1,
			Rationale:      "Improve resilience",
			Confidence:     "medium",
		},
	})

	// Run 3: First semantically similar proposal
	run3ID := "run-603"
	seedRunWithProposals(t, store, tmp, projectID, run3ID, "spec-db", []reviewdistiller.Proposal{
		{
			ID:             "run-603-proposal-cccc3333",
			Type:           "validation_gap",
			Title:          "Parameterized DB Queries",
			ProposedChange: similarChange1,
			Rationale:      "Prevent SQL injection",
			Confidence:     "high",
		},
	})

	// Run 4: Second semantically similar proposal + second unique proposal
	run4ID := "run-604"
	seedRunWithProposals(t, store, tmp, projectID, run4ID, "spec-db", []reviewdistiller.Proposal{
		{
			ID:             "run-604-proposal-dddd4444",
			Type:           "validation_gap",
			Title:          "SQL Injection Prevention",
			ProposedChange: similarChange2,
			Rationale:      "Database security",
			Confidence:     "high",
		},
		{
			ID:             "run-604-proposal-ffff6666",
			Type:           "planner_heuristic",
			Title:          "Structured Logging",
			ProposedChange: uniqueChange2,
			Rationale:      "Improve observability",
			Confidence:     "medium",
		},
	})

	// === Invoke ===

	// Discover all pending proposals
	pendingProposals, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending failed: %v", err)
	}

	if len(pendingProposals) != 6 {
		t.Fatalf("expected 6 pending proposals, got %d", len(pendingProposals))
	}

	// Stub LLM that clusters the two semantically similar proposals
	stubLLM := &stubLLMCompleterForCmd{
		response: `{
  "clusters": [
    {
      "proposal_ids": ["run-603-proposal-cccc3333", "run-604-proposal-dddd4444"],
      "description": "Database query parameterization for SQL injection prevention"
    }
  ]
}`,
	}

	// Run the full grouping pipeline
	groups, warnings := proposaltriage.GroupProposals(context.Background(), pendingProposals, stubLLM)

	// === Assert ===

	// No warnings expected
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// Expect 4 groups: 1 exact_match + 1 semantic cluster + 2 singletons
	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}

	// Classify groups
	var exactMatchGroup *proposaltriage.ProposalGroup
	var semanticClusterGroup *proposaltriage.ProposalGroup
	var singletonGroups []proposaltriage.ProposalGroup

	for i := range groups {
		g := &groups[i]
		switch {
		case g.GroupReason == "exact_match" && len(g.Proposals) == 2:
			exactMatchGroup = g
		case g.GroupReason == "Database query parameterization for SQL injection prevention":
			semanticClusterGroup = g
		case g.GroupReason == "singleton" && len(g.Proposals) == 1:
			singletonGroups = append(singletonGroups, *g)
		default:
			t.Logf("unclassified group: reason=%q size=%d", g.GroupReason, len(g.Proposals))
		}
	}

	// Verify exact_match group contains the two identical proposals
	if exactMatchGroup == nil {
		t.Fatal("expected exact_match group not found")
	}
	exactIDs := proposalIDSet(exactMatchGroup.Proposals)
	if !exactIDs["run-601-proposal-aaaa1111"] || !exactIDs["run-602-proposal-bbbb2222"] {
		t.Errorf("exact_match group should contain the two identical proposals, got %v", exactIDs)
	}

	// Verify semantic cluster group contains the two similar proposals
	if semanticClusterGroup == nil {
		t.Fatal("expected semantic cluster group not found")
	}
	if len(semanticClusterGroup.Proposals) != 2 {
		t.Fatalf("semantic cluster group should have 2 proposals, got %d", len(semanticClusterGroup.Proposals))
	}
	semanticIDs := proposalIDSet(semanticClusterGroup.Proposals)
	if !semanticIDs["run-603-proposal-cccc3333"] || !semanticIDs["run-604-proposal-dddd4444"] {
		t.Errorf("semantic cluster group should contain the two similar proposals, got %v", semanticIDs)
	}

	// Verify 2 singleton groups for the unique proposals
	if len(singletonGroups) != 2 {
		t.Fatalf("expected 2 singleton groups, got %d", len(singletonGroups))
	}
	singletonIDs := make(map[string]bool)
	for _, sg := range singletonGroups {
		if sg.Proposals[0].Proposal != nil {
			singletonIDs[sg.Proposals[0].Proposal.ID] = true
		}
	}
	if !singletonIDs["run-602-proposal-eeee5555"] || !singletonIDs["run-604-proposal-ffff6666"] {
		t.Errorf("singleton groups should contain the two unique proposals, got %v", singletonIDs)
	}

	// Verify all groups have non-empty GroupIDs (deterministic hashes)
	for i, g := range groups {
		if g.GroupID == "" {
			t.Errorf("group %d should have a non-empty GroupID", i)
		}
	}

	// Verify exact_match GroupID is deterministic by re-running grouping
	groups2, _ := proposaltriage.GroupProposals(context.Background(), pendingProposals, stubLLM)
	for _, g2 := range groups2 {
		if g2.GroupReason == "exact_match" && len(g2.Proposals) == 2 {
			if g2.GroupID != exactMatchGroup.GroupID {
				t.Errorf("exact_match GroupID not deterministic: %s vs %s", g2.GroupID, exactMatchGroup.GroupID)
			}
			break
		}
	}

	// Verify displayPendingProposals renders without error and shows expected content
	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displayErr := displayPendingProposals(groups)

	w.Close()
	os.Stdout = origStdout
	buf.ReadFrom(r)

	if displayErr != nil {
		t.Fatalf("displayPendingProposals failed: %v", displayErr)
	}

	output := buf.String()

	// Verify output contains group headers
	if !strings.Contains(output, "exact_match") {
		t.Error("output should contain exact_match group reason")
	}
	if !strings.Contains(output, "Database query parameterization for SQL injection prevention") {
		t.Error("output should contain semantic cluster description")
	}
	if !strings.Contains(output, "singleton") {
		t.Error("output should contain singleton group reason")
	}

	// Verify output contains proposal titles
	if !strings.Contains(output, "Auth Token Validation Rule") {
		t.Error("output should contain identical proposal title")
	}
	if !strings.Contains(output, "Retry with Backoff") {
		t.Error("output should contain first unique proposal title")
	}
	if !strings.Contains(output, "Structured Logging") {
		t.Error("output should contain second unique proposal title")
	}
}

// seedRunWithProposals creates a run and its distillation-proposals.json in the store.
func seedRunWithProposals(t *testing.T, store *runstore.Store, storeDir, projectID, runID, specID string, proposals []reviewdistiller.Proposal) {
	t.Helper()

	run := &runstore.RunState{
		RunID:                 runID,
		SpecID:                specID,
		ProjectID:             projectID,
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 22, 10, 15, 0, 0, time.UTC),
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
		SpecID:    specID,
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierHigh,
		CreatedAt: time.Date(2026, 3, 22, 10, 20, 0, 0, time.UTC),
		Proposals: proposals,
	}

	data, err := json.MarshalIndent(distResult, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals for %s: %v", runID, err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), data, 0o644); err != nil {
		t.Fatalf("write proposals for %s: %v", runID, err)
	}
}

// proposalIDSet extracts proposal IDs from a slice of PendingProposals into a set.
func proposalIDSet(proposals []proposaltriage.PendingProposal) map[string]bool {
	ids := make(map[string]bool)
	for _, pp := range proposals {
		if pp.Proposal != nil {
			ids[pp.Proposal.ID] = true
		}
	}
	return ids
}

// stubLLMCompleterForCmd implements reviewdistiller.LLMCompleter for testing in cmd package.
type stubLLMCompleterForCmd struct {
	response string
	err      error
}

func (s *stubLLMCompleterForCmd) Complete(_ context.Context, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.response, nil
}
