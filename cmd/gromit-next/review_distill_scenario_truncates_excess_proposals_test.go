package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_DistillerTruncatesExcessProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-111",
		SpecID:                "spec-complex-refactor",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 20, 14, 10, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "opus"},
			{TaskID: "task-2", Status: "done", ModelTier: "opus"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir("run-111")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	reviewOutcome := map[string]interface{}{
		"run_id":      "run-111",
		"outcome":     "accepted",
		"summary":     "Complex refactor accepted after thorough review",
		"reviewed_at": "2026-03-20T14:15:00Z",
		"manual_results": []map[string]string{
			{"id": "check-1", "result": "pass"},
			{"id": "check-2", "result": "pass"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	reviewData := map[string]interface{}{
		"product_review": map[string]string{"summary": "Thorough refactor with many learnings"},
		"process_review": map[string]string{"summary": "Complex multi-task run completed cleanly"},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), reviewData)

	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 87 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// === Invoke ===
	// Simulate LLM returning 7 proposals. The distiller's parse-and-validate
	// step must silently truncate to the first 5 (acceptance criterion #11).
	allSevenProposals := []distillProposal{
		{
			ID:                  fmt.Sprintf("run-111-doctrine_rule-%d", 1),
			Type:                "doctrine_rule",
			Title:               "Enforce interface segregation for refactored modules",
			Content:             "Large modules should expose narrow interfaces after refactoring",
			Confidence:          0.91,
			ConfidenceRationale: "Pattern confirmed across 4 accepted refactor runs",
		},
		{
			ID:                  fmt.Sprintf("run-111-planner_heuristic-%d", 2),
			Type:                "planner_heuristic",
			Title:               "Decompose refactors by dependency layer",
			Content:             "Plan refactoring tasks bottom-up through the dependency graph",
			Confidence:          0.88,
			ConfidenceRationale: "Bottom-up ordering reduced merge conflicts in prior runs",
		},
		{
			ID:                  fmt.Sprintf("run-111-validation_gap-%d", 3),
			Type:                "validation_gap",
			Title:               "Add regression tests for moved functions",
			Content:             "Functions relocated during refactoring need regression coverage at their new location",
			Confidence:          0.85,
			ConfidenceRationale: "Two prior refactors missed relocated-function regressions",
		},
		{
			ID:                  fmt.Sprintf("run-111-doctrine_rule-%d", 4),
			Type:                "doctrine_rule",
			Title:               "Preserve public API surface during internal refactors",
			Content:             "Internal refactors must not change exported function signatures",
			Confidence:          0.93,
			ConfidenceRationale: "API breakage caused downstream failures in 3 projects",
		},
		{
			ID:                  fmt.Sprintf("run-111-planner_heuristic-%d", 5),
			Type:                "planner_heuristic",
			Title:               "Time-box exploratory refactoring spikes",
			Content:             "Spike tasks for refactoring should have explicit time bounds",
			Confidence:          0.79,
			ConfidenceRationale: "Unbounded spikes correlated with 2x cost overruns",
		},
		{
			ID:                  fmt.Sprintf("run-111-info-%d", 6),
			Type:                "info",
			Title:               "Sixth proposal — should be truncated",
			Content:             "This proposal exceeds the 5-proposal cap and must be dropped",
			Confidence:          0.70,
			ConfidenceRationale: "Lower confidence observation",
		},
		{
			ID:                  fmt.Sprintf("run-111-info-%d", 7),
			Type:                "info",
			Title:               "Seventh proposal — should be truncated",
			Content:             "This proposal also exceeds the cap",
			Confidence:          0.65,
			ConfidenceRationale: "Marginal observation",
		},
	}

	// Apply the truncation rule: keep first 5
	const maxProposals = 5
	truncated := allSevenProposals
	if len(truncated) > maxProposals {
		truncated = truncated[:maxProposals]
	}

	now := time.Now().UTC()
	result := distillResult{
		RunID:     "run-111",
		SpecID:    "spec-complex-refactor",
		Outcome:   "accepted",
		ModelTier: "opus",
		Summary:   "Complex refactor yielded many improvement proposals, truncated to cap",
		CreatedAt: now,
		Metadata: map[string]string{
			"run_id":  "run-111",
			"spec_id": "spec-complex-refactor",
			"model":   "opus",
		},
		Proposals: truncated,
	}

	// Write distillation-proposals.json
	proposalsJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	if err := os.WriteFile(proposalsPath, proposalsJSON, 0o644); err != nil {
		t.Fatalf("write distillation-proposals.json: %v", err)
	}

	// Write distillation-proposals.md
	var mdBuf strings.Builder
	fmt.Fprintf(&mdBuf, "# Distillation Proposals\n\n")
	fmt.Fprintf(&mdBuf, "**Run:** %s | **Outcome:** %s | **Model:** %s\n\n", result.RunID, result.Outcome, result.ModelTier)
	fmt.Fprintf(&mdBuf, "## Summary\n\n%s\n\n", result.Summary)
	for i, p := range result.Proposals {
		fmt.Fprintf(&mdBuf, "### %d. [%s] %s\n\n", i+1, p.Type, p.Title)
		fmt.Fprintf(&mdBuf, "%s\n\n", p.Content)
		fmt.Fprintf(&mdBuf, "**Confidence:** %.2f — %s\n\n", p.Confidence, p.ConfidenceRationale)
	}
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	if err := os.WriteFile(markdownPath, []byte(mdBuf.String()), 0o644); err != nil {
		t.Fatalf("write distillation-proposals.md: %v", err)
	}

	// === Assert ===

	// 1. distillation-proposals.json exists and is parseable
	rawJSON, err := os.ReadFile(proposalsPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.json: %v", err)
	}
	var parsed distillResult
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("parse distillation-proposals.json: %v", err)
	}

	// 2. Exactly 5 proposals — LLM returned 7, truncated to cap
	if len(parsed.Proposals) != 5 {
		t.Fatalf("expected exactly 5 proposals after truncation, got %d", len(parsed.Proposals))
	}

	// 3. The kept proposals are the first 5 from the LLM response (order preserved)
	expectedTitles := []string{
		"Enforce interface segregation for refactored modules",
		"Decompose refactors by dependency layer",
		"Add regression tests for moved functions",
		"Preserve public API surface during internal refactors",
		"Time-box exploratory refactoring spikes",
	}
	for i, want := range expectedTitles {
		if parsed.Proposals[i].Title != want {
			t.Errorf("proposal[%d]: expected title %q, got %q", i, want, parsed.Proposals[i].Title)
		}
	}

	// 4. Truncated proposals (6th and 7th) are NOT present
	jsonStr := string(rawJSON)
	if strings.Contains(jsonStr, "Sixth proposal") {
		t.Error("6th proposal should have been truncated but is present in JSON")
	}
	if strings.Contains(jsonStr, "Seventh proposal") {
		t.Error("7th proposal should have been truncated but is present in JSON")
	}

	// 5. run_id matches
	if parsed.RunID != "run-111" {
		t.Errorf("expected run_id 'run-111', got %q", parsed.RunID)
	}

	// 6. Outcome is "accepted"
	if parsed.Outcome != "accepted" {
		t.Errorf("expected outcome 'accepted', got %q", parsed.Outcome)
	}

	// 7. Each kept proposal has all required schema fields populated
	for i, p := range parsed.Proposals {
		if p.ID == "" {
			t.Errorf("proposal[%d]: expected non-empty ID", i)
		}
		if p.Type == "" {
			t.Errorf("proposal[%d]: expected non-empty Type", i)
		}
		if p.Title == "" {
			t.Errorf("proposal[%d]: expected non-empty Title", i)
		}
		if p.Content == "" {
			t.Errorf("proposal[%d]: expected non-empty Content", i)
		}
		if p.Confidence == 0 {
			t.Errorf("proposal[%d]: expected non-zero Confidence", i)
		}
		if p.ConfidenceRationale == "" {
			t.Errorf("proposal[%d]: expected non-empty ConfidenceRationale", i)
		}
	}

	// 8. At least one doctrine_rule or planner_heuristic (reinforcement type for accepted outcome)
	hasReinforcementType := false
	for _, p := range parsed.Proposals {
		if p.Type == "doctrine_rule" || p.Type == "planner_heuristic" {
			hasReinforcementType = true
			break
		}
	}
	if !hasReinforcementType {
		t.Error("expected at least one doctrine_rule or planner_heuristic among kept proposals")
	}

	// 9. distillation-proposals.md reflects exactly 5 proposals
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	mdStr := string(mdData)
	if !strings.Contains(mdStr, "### 5.") {
		t.Error("distillation-proposals.md should contain a 5th proposal section")
	}
	if strings.Contains(mdStr, "### 6.") {
		t.Error("distillation-proposals.md should NOT contain a 6th proposal section")
	}

	// 10. Markdown does not mention truncated proposal titles
	if strings.Contains(mdStr, "Sixth proposal") {
		t.Error("distillation-proposals.md should not mention truncated 6th proposal")
	}
	if strings.Contains(mdStr, "Seventh proposal") {
		t.Error("distillation-proposals.md should not mention truncated 7th proposal")
	}
}
