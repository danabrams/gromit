package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
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
	// Mock LLM returns 7 proposals; distiller must truncate to first 5.
	mockCompleter := &mockDistillerLLM{
		response: `{
  "proposals": [
    {
      "type": "doctrine_rule",
      "title": "Enforce interface segregation for refactored modules",
      "what_happened": "Large modules were refactored without clear interface boundaries",
      "what_was_missing": "A doctrine rule on interface segregation after refactoring",
      "proposed_change": "Add doctrine rule: refactored modules must expose narrow interfaces",
      "rationale": "Large interfaces increase coupling and maintainability burden",
      "confidence": "high",
      "confidence_rationale": "Pattern confirmed across 4 accepted refactor runs",
      "evidence_references": []
    },
    {
      "type": "planner_heuristic",
      "title": "Decompose refactors by dependency layer",
      "what_happened": "Refactors were planned top-down, causing merge conflicts",
      "what_was_missing": "A heuristic for planning refactors bottom-up through dependency graph",
      "proposed_change": "Add planner heuristic: decompose refactoring bottom-up by layer",
      "rationale": "Bottom-up reduces merge conflicts and enables incremental testing",
      "confidence": "high",
      "confidence_rationale": "Bottom-up ordering reduced merge conflicts in prior runs",
      "evidence_references": []
    },
    {
      "type": "validation_gap",
      "title": "Add regression tests for moved functions",
      "what_happened": "Functions were moved during refactoring, regression coverage was missed",
      "what_was_missing": "Regression testing at new function locations",
      "proposed_change": "Add validation gap: moved functions require regression coverage at new location",
      "rationale": "Prevents silent breakage of relocated functionality",
      "confidence": "high",
      "confidence_rationale": "Two prior refactors missed relocated-function regressions",
      "evidence_references": []
    },
    {
      "type": "doctrine_rule",
      "title": "Preserve public API surface during internal refactors",
      "what_happened": "Internal refactoring inadvertently changed exported function signatures",
      "what_was_missing": "A doctrine rule protecting exported APIs during refactoring",
      "proposed_change": "Add doctrine rule: internal refactors must preserve exported function signatures",
      "rationale": "Public API stability is critical for downstream projects",
      "confidence": "high",
      "confidence_rationale": "API breakage caused downstream failures in 3 projects",
      "evidence_references": []
    },
    {
      "type": "planner_heuristic",
      "title": "Time-box exploratory refactoring spikes",
      "what_happened": "Refactoring spike tasks had no explicit time bound and exceeded estimates",
      "what_was_missing": "A heuristic for bounded refactoring spikes",
      "proposed_change": "Add planner heuristic: spike tasks for refactoring should have explicit time bounds",
      "rationale": "Prevents unbounded exploration and scope creep",
      "confidence": "medium",
      "confidence_rationale": "Unbounded spikes correlated with 2x cost overruns",
      "evidence_references": []
    },
    {
      "type": "info",
      "title": "Sixth proposal — should be truncated",
      "what_happened": "This is a sixth observation",
      "what_was_missing": "Irrelevant",
      "proposed_change": "This proposal exceeds the 5-proposal cap and must be dropped",
      "rationale": "Lower confidence observation",
      "confidence": "low",
      "confidence_rationale": "Marginal evidence",
      "evidence_references": []
    },
    {
      "type": "info",
      "title": "Seventh proposal — should be truncated",
      "what_happened": "This is a seventh observation",
      "what_was_missing": "Irrelevant",
      "proposed_change": "This proposal also exceeds the cap and must be dropped",
      "rationale": "Even lower confidence",
      "confidence": "low",
      "confidence_rationale": "Very marginal evidence",
      "evidence_references": []
    }
  ]
}`,
	}

	reviewOutcomeJSON, _ := json.Marshal(reviewOutcome)
	reviewDataJSON, _ := json.Marshal(reviewData)

	inputs := &reviewdistiller.DistillerInputs{
		RunID:         "run-111",
		SpecID:        "spec-complex-refactor",
		SpecContent:   "# Test Spec",
		ReviewOutcome: reviewOutcomeJSON,
		MachineReview: reviewDataJSON,
	}

	result, err := reviewdistiller.Distill(inputs, mockCompleter, reviewdistiller.TierHigh)
	if err != nil {
		t.Fatalf("distillation failed: %v", err)
	}

	// Write distillation-proposals.json
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	proposalsJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := os.WriteFile(proposalsPath, proposalsJSON, 0o644); err != nil {
		t.Fatalf("write distillation-proposals.json: %v", err)
	}

	// Write distillation-proposals.md
	var mdBuf strings.Builder
	fmt.Fprintf(&mdBuf, "# Distillation Proposals\n\n")
	fmt.Fprintf(&mdBuf, "**Run:** %s | **Outcome:** %s | **Model:** %s\n\n", result.RunID, result.Outcome, result.ModelTier)
	fmt.Fprintf(&mdBuf, "## Summary\n\nDistilled %s review outcome into %d improvement proposals.\n\n", result.Outcome, len(result.Proposals))
	for i, p := range result.Proposals {
		fmt.Fprintf(&mdBuf, "### %d. [%s] %s\n\n", i+1, p.Type, p.Title)
		fmt.Fprintf(&mdBuf, "**Confidence:** %s — %s\n\n", p.Confidence, p.ConfidenceRationale)
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
	var parsed reviewdistiller.DistillationResult
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
		if p.WhatHappened == "" {
			t.Errorf("proposal[%d]: expected non-empty WhatHappened", i)
		}
		if p.Confidence == "" {
			t.Errorf("proposal[%d]: expected non-empty Confidence", i)
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

// mockDistillerLLM is a test stub for reviewdistiller.LLMCompleter.
// It returns a canned JSON response with 7 proposals.
type mockDistillerLLM struct {
	response string
}

func (m *mockDistillerLLM) Complete(ctx interface{}, prompt string) (string, error) {
	return m.response, nil
}
