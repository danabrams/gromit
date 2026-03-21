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

func TestScenario_DistillerAcceptsFewerThanThreeProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-108",
		SpecID:                "spec-trivial",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 20, 9, 1, 0, 0, time.UTC),
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

	evidenceDir := store.RunEvidenceDir("run-108")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	reviewOutcome := map[string]interface{}{
		"run_id":      "run-108",
		"outcome":     "accepted",
		"summary":     "Trivially simple spec, accepted without issues",
		"reviewed_at": "2026-03-20T09:05:00Z",
		"manual_results": []map[string]string{
			{"id": "check-1", "result": "pass"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	reviewData := map[string]interface{}{
		"product_review": map[string]string{"summary": "Simple change, meets requirements"},
		"process_review": map[string]string{"summary": "Clean single-task run"},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), reviewData)

	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 5 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// === Invoke ===
	// Mock LLM returns exactly 2 proposals — the lower bound of 3 is advisory, not enforced.
	mockCompleter := &mockLLMCompleter{
		response: `{
  "proposals": [
    {
      "type": "doctrine_rule",
      "title": "Keep trivial specs single-task",
      "what_happened": "Spec was simple enough for single task",
      "what_was_missing": "Prior guidance on single-task mapping",
      "proposed_change": "Establish rule: single-criterion specs map to one task",
      "rationale": "Reduces planning overhead for trivial cases",
      "confidence": "high",
      "confidence_rationale": "Consistent with prior simple runs that completed in one cycle",
      "evidence_references": []
    },
    {
      "type": "validation_gap",
      "title": "Sonnet tier sufficient for trivial specs",
      "what_happened": "Spec completed successfully with sonnet tier",
      "what_was_missing": "Confidence threshold for tier selection",
      "proposed_change": "Add tier-selection guidance: sonnet is sufficient for trivial specs",
      "rationale": "Reduces unnecessary model tier escalation",
      "confidence": "medium",
      "confidence_rationale": "Run completed under budget and on first attempt",
      "evidence_references": []
    }
  ]
}`,
	}

	outcomeData, _ := json.Marshal(reviewOutcome)
	inputs := &reviewdistiller.DistillerInputs{
		RunID:         "run-108",
		SpecID:        "spec-trivial",
		SpecContent:   "# Trivial Spec\nCalculate 1 + 1",
		ReviewOutcome: outcomeData,
		Validation: json.RawMessage(`{
  "pass": true,
  "build_errors": [],
  "test_results": "All 5 tests passed"
}`),
	}

	result, err := reviewdistiller.Distill(inputs, mockCompleter, reviewdistiller.TierMedium)
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
	fmt.Fprintf(&mdBuf, "**Run:** %s | **Outcome:** %s | **Model Tier:** %s\n\n", result.RunID, result.Outcome, result.ModelTier)
	fmt.Fprintf(&mdBuf, "## Summary\n\nDistilled %s review outcome into %d improvement proposals.\n\n", result.Outcome, len(result.Proposals))
	for _, p := range result.Proposals {
		fmt.Fprintf(&mdBuf, "---\n\n")
		fmt.Fprintf(&mdBuf, "## %s\n\n", p.Title)
		fmt.Fprintf(&mdBuf, "**Type:** %s\n", p.Type)
		fmt.Fprintf(&mdBuf, "**Confidence:** %s\n\n", p.Confidence)
		fmt.Fprintf(&mdBuf, "### What Happened\n\n%s\n\n", p.WhatHappened)
		fmt.Fprintf(&mdBuf, "### What Was Missing\n\n%s\n\n", p.WhatWasMissing)
		fmt.Fprintf(&mdBuf, "### Proposed Change\n\n%s\n\n", p.ProposedChange)
		fmt.Fprintf(&mdBuf, "### Rationale\n\n%s\n\n", p.Rationale)
		fmt.Fprintf(&mdBuf, "**Confidence Rationale:** %s\n\n", p.ConfidenceRationale)
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

	// 2. run_id matches
	if parsed.RunID != "run-108" {
		t.Errorf("expected run_id 'run-108', got %q", parsed.RunID)
	}

	// 3. Outcome is "accepted"
	if parsed.Outcome != "accepted" {
		t.Errorf("expected outcome 'accepted', got %q", parsed.Outcome)
	}

	// 4. Exactly 2 proposals — the lower bound of 3 is advisory, not enforced
	if len(parsed.Proposals) != 2 {
		t.Errorf("expected exactly 2 proposals (advisory lower bound not enforced), got %d", len(parsed.Proposals))
	}

	// 5. Both proposals have all required schema fields populated
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
		if p.WhatWasMissing == "" {
			t.Errorf("proposal[%d]: expected non-empty WhatWasMissing", i)
		}
		if p.ProposedChange == "" {
			t.Errorf("proposal[%d]: expected non-empty ProposedChange", i)
		}
		if p.Rationale == "" {
			t.Errorf("proposal[%d]: expected non-empty Rationale", i)
		}
		if p.Confidence == "" {
			t.Errorf("proposal[%d]: expected non-empty Confidence", i)
		}
		if p.ConfidenceRationale == "" {
			t.Errorf("proposal[%d]: expected non-empty ConfidenceRationale", i)
		}
	}

	// 6. At least one doctrine_rule or planner_heuristic (reinforcement type)
	hasReinforcementType := false
	for _, p := range parsed.Proposals {
		if p.Type == "doctrine_rule" || p.Type == "planner_heuristic" {
			hasReinforcementType = true
			break
		}
	}
	if !hasReinforcementType {
		types := make([]string, len(parsed.Proposals))
		for i, p := range parsed.Proposals {
			types[i] = p.Type
		}
		t.Errorf("expected at least one doctrine_rule or planner_heuristic, got types: %v", types)
	}

	// 7. CreatedAt is non-zero
	if parsed.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}

	// 8. distillation-proposals.md exists and reflects 2 proposals
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	mdStr := string(mdData)
	if !strings.Contains(mdStr, "accepted") {
		t.Error("distillation-proposals.md should mention accepted outcome")
	}
	if !strings.Contains(mdStr, "Keep trivial specs single-task") {
		t.Error("distillation-proposals.md should contain first proposal title")
	}
	if !strings.Contains(mdStr, "Sonnet tier sufficient for trivial specs") {
		t.Error("distillation-proposals.md should contain second proposal title")
	}
	if strings.Contains(mdStr, "Error rendering markdown") {
		t.Error("distillation-proposals.md should not contain rendering error")
	}
}
