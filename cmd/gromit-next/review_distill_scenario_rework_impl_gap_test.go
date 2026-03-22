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

func TestScenario_ReworkImplementationGapProducesGuardrailProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-102",
		SpecID:                "spec-keyboard-nav",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 16, 14, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 16, 14, 10, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: false,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "sonnet"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed evidence files
	evidenceDir := store.RunEvidenceDir("run-102")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// review-outcome.json: rework_implementation_gap with 1 failed manual check
	reviewOutcome := map[string]interface{}{
		"run_id":      "run-102",
		"outcome":     "rework_implementation_gap",
		"summary":     "Keyboard navigation is broken in the modal component",
		"reviewed_at": "2026-03-16T14:15:00Z",
		"manual_results": []map[string]string{
			{"id": "check-a11y-keyboard", "result": "fail", "notes": "Keyboard nav broken"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	// process-review.json with trust level "medium"
	processReview := map[string]interface{}{
		"trust_level":         "medium",
		"automatic_proof":     "Tests passed but coverage incomplete",
		"machine_review":      "No blocking issues found",
		"recommended_posture": "manual_check_carefully",
		"degraded_flags":      []string{"incomplete_coverage"},
		"repair_cycles":       1,
	}
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), processReview)

	// product-review.json
	productReview := map[string]interface{}{
		"run_id":        "run-102",
		"is_diagnostic": false,
		"summary":       "Implementation complete but accessibility gaps remain",
	}
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), productReview)

	// validation.json
	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "28 of 30 tests passed, 2 skipped",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// === Invoke ===
	// Create LLM response JSON with 4 proposals including validation_gap, doctrine_rule, and planner_heuristic
	llmResponse := `{
		"proposals": [
			{
				"type": "validation_gap",
				"title": "Add keyboard navigation integration tests for modal components",
				"what_happened": "Automated tests did not cover keyboard navigation flows",
				"what_was_missing": "Integration tests that simulate Tab, Escape, and Enter key interactions",
				"proposed_change": "Create integration tests for modal keyboard navigation patterns",
				"rationale": "Manual check explicitly failed on keyboard nav — no automated equivalent exists",
				"confidence": "high",
				"confidence_rationale": "Clear failure pattern with defined remediation",
				"evidence_references": ["review-outcome.json", "check-a11y-keyboard", "process-review.json"]
			},
			{
				"type": "doctrine_rule",
				"title": "Require a11y checks for all interactive UI components",
				"what_happened": "Keyboard navigation acceptance criteria were not included",
				"what_was_missing": "Systematic a11y validation requirements in spec",
				"proposed_change": "Add a11y checks as mandatory acceptance criteria for all interactive UI specs",
				"rationale": "Failed manual check indicates systematic gap in acceptance criteria",
				"confidence": "high",
				"confidence_rationale": "Consistent pattern across similar failures",
				"evidence_references": ["review-outcome.json", "check-a11y-keyboard"]
			},
			{
				"type": "planner_heuristic",
				"title": "Split UI tasks into visual and interaction sub-tasks",
				"what_happened": "Task focused on visual rendering without explicit interaction layer validation",
				"what_was_missing": "Separate sub-task planning for keyboard and screen-reader interactions",
				"proposed_change": "When planning interactive UI tasks, create separate sub-tasks for visual rendering and interaction validation",
				"rationale": "Implementation focused on visual correctness but missed interaction layer",
				"confidence": "medium",
				"confidence_rationale": "Empirical pattern from this failure, helps prevent recurrence",
				"evidence_references": ["review-outcome.json", "check-a11y-keyboard", "validation.json"]
			},
			{
				"type": "validation_gap",
				"title": "Add focus-trap validation for modal dialogs",
				"what_happened": "Modal implementation did not include focus management",
				"what_was_missing": "Automated focus-trap validation in test suite",
				"proposed_change": "Add automated tests for focus trapping and restoration patterns in modals",
				"rationale": "Common a11y pattern missing from validation suite, related to keyboard nav failure",
				"confidence": "medium",
				"confidence_rationale": "Standard a11y pattern, observable through manual testing",
				"evidence_references": ["check-a11y-keyboard", "process-review.json"]
			}
		]
	}`

	// Create DistillerInputs from evidence data
	reviewOutcomeBytes, err := json.Marshal(reviewOutcome)
	if err != nil {
		t.Fatalf("marshal review outcome: %v", err)
	}

	productReviewBytes, err := json.Marshal(productReview)
	if err != nil {
		t.Fatalf("marshal product review: %v", err)
	}

	processReviewBytes, err := json.Marshal(processReview)
	if err != nil {
		t.Fatalf("marshal process review: %v", err)
	}

	validationBytes, err := json.Marshal(validationData)
	if err != nil {
		t.Fatalf("marshal validation: %v", err)
	}

	inputs := &reviewdistiller.DistillerInputs{
		RunID:         "run-102",
		SpecID:        "spec-keyboard-nav",
		ReviewOutcome: json.RawMessage(reviewOutcomeBytes),
		ProductReview: json.RawMessage(productReviewBytes),
		ProcessReview: json.RawMessage(processReviewBytes),
		Validation:    json.RawMessage(validationBytes),
	}

	// Create mock LLM and call Distill
	mock := &mockLLMCompleter{response: llmResponse}
	result, err := reviewdistiller.Distill(inputs, mock, reviewdistiller.TierMedium)
	if err != nil {
		t.Fatalf("Distill() returned error: %v", err)
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
	for i, p := range result.Proposals {
		fmt.Fprintf(&mdBuf, "### %d. [%s] %s\n\n", i+1, p.Type, p.Title)
		fmt.Fprintf(&mdBuf, "**What Happened:** %s\n\n", p.WhatHappened)
		fmt.Fprintf(&mdBuf, "**What Was Missing:** %s\n\n", p.WhatWasMissing)
		fmt.Fprintf(&mdBuf, "**Proposed Change:** %s\n\n", p.ProposedChange)
		fmt.Fprintf(&mdBuf, "**Rationale:** %s\n\n", p.Rationale)
		fmt.Fprintf(&mdBuf, "**Confidence:** %s — %s\n\n", p.Confidence, p.ConfidenceRationale)
		if len(p.EvidenceReferences) > 0 {
			fmt.Fprintf(&mdBuf, "**Evidence:** %s\n\n", strings.Join(p.EvidenceReferences, ", "))
		}
	}
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	if err := os.WriteFile(markdownPath, []byte(mdBuf.String()), 0o644); err != nil {
		t.Fatalf("write distillation-proposals.md: %v", err)
	}

	// === Assert ===

	// 1. Result is not nil and has correct RunID/SpecID/Outcome
	if result == nil {
		t.Fatal("expected non-nil result from Distill()")
	}
	if result.RunID != "run-102" {
		t.Errorf("expected RunID 'run-102', got %q", result.RunID)
	}
	if result.SpecID != "spec-keyboard-nav" {
		t.Errorf("expected SpecID 'spec-keyboard-nav', got %q", result.SpecID)
	}
	if result.Outcome != "rework_implementation_gap" {
		t.Errorf("expected Outcome 'rework_implementation_gap', got %q", result.Outcome)
	}

	// 2. 3-5 proposals
	if len(result.Proposals) < 3 || len(result.Proposals) > 5 {
		t.Errorf("expected 3-5 proposals, got %d", len(result.Proposals))
	}

	// 3. Verify ModelTier is set
	if result.ModelTier != reviewdistiller.TierMedium {
		t.Errorf("expected ModelTier TierMedium, got %q", result.ModelTier)
	}

	// 4. Verify CreatedAt is set
	if result.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// 5. At least one validation_gap, doctrine_rule, or planner_heuristic
	guardrailTypes := map[string]int{}
	for _, p := range result.Proposals {
		guardrailTypes[p.Type]++
	}
	if guardrailTypes["validation_gap"] == 0 && guardrailTypes["doctrine_rule"] == 0 && guardrailTypes["planner_heuristic"] == 0 {
		types := make([]string, 0, len(guardrailTypes))
		for t := range guardrailTypes {
			types = append(types, t)
		}
		t.Errorf("expected at least one validation_gap, doctrine_rule, or planner_heuristic, got: %v", types)
	}

	// 6. At least one proposal references the failed manual check item "check-a11y-keyboard"
	hasFailedCheckRef := false
	for _, p := range result.Proposals {
		for _, ref := range p.EvidenceReferences {
			if ref == "check-a11y-keyboard" {
				hasFailedCheckRef = true
				break
			}
		}
		if hasFailedCheckRef {
			break
		}
	}
	if !hasFailedCheckRef {
		t.Error("expected at least one proposal to reference 'check-a11y-keyboard' in evidence_references")
	}

	// 7. At least one proposal references an evidence file
	hasEvidenceFileRef := false
	evidenceFiles := map[string]bool{
		"review-outcome.json": true,
		"process-review.json": true,
		"validation.json":     true,
		"product-review.json": true,
	}
	for _, p := range result.Proposals {
		for _, ref := range p.EvidenceReferences {
			if evidenceFiles[ref] {
				hasEvidenceFileRef = true
				break
			}
		}
		if hasEvidenceFileRef {
			break
		}
	}
	if !hasEvidenceFileRef {
		t.Error("expected at least one proposal to reference an evidence file in evidence_references")
	}

	// 8. Each proposal has required schema fields populated
	for i, p := range result.Proposals {
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
		if len(p.EvidenceReferences) == 0 {
			t.Errorf("proposal[%d]: expected non-empty EvidenceReferences", i)
		}
	}

	// 9. evidence_references field present in raw JSON
	rawJSON, err := os.ReadFile(proposalsPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.json: %v", err)
	}
	jsonStr := string(rawJSON)
	if !strings.Contains(jsonStr, "evidence_references") {
		t.Error("expected evidence_references field in JSON output")
	}

	// 10. distillation-proposals.md exists and contains outcome
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	if len(mdData) == 0 {
		t.Error("expected non-empty distillation-proposals.md")
	}
	mdContent := string(mdData)
	if !strings.Contains(mdContent, "What Happened") && !strings.Contains(mdContent, "what_happened") {
		t.Error("distillation-proposals.md should mention what happened")
	}

	// 11. Markdown references the failed check
	if !strings.Contains(mdContent, "check-a11y-keyboard") {
		t.Error("distillation-proposals.md should reference the failed check item")
	}

	// 12. Markdown mentions keyboard nav content
	if !strings.Contains(mdContent, "keyboard") {
		t.Error("distillation-proposals.md should mention keyboard-related content")
	}
}
