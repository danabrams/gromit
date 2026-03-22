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

func TestScenario_ReworkVisionChangeProducesRefinementProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-103",
		SpecID:                "spec-inline-editing",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 17, 9, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 17, 9, 12, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: false,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "opus"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir("run-103")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	outcomeSummary := "Product direction shifted — we no longer want inline editing"

	reviewOutcome := map[string]interface{}{
		"run_id":      "run-103",
		"outcome":     "rework_vision_change",
		"summary":     outcomeSummary,
		"reviewed_at": "2026-03-17T09:20:00Z",
		"manual_results": []map[string]string{
			{"id": "check-ux-direction", "result": "fail", "notes": "Inline editing no longer desired"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	processReview := map[string]interface{}{
		"trust_level":         "high",
		"automatic_proof":     "All tests passed",
		"machine_review":      "No blocking issues",
		"recommended_posture": "stamp_if_clean",
		"degraded_flags":      []string{},
		"repair_cycles":       0,
	}
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), processReview)

	productReview := map[string]interface{}{
		"run_id":        "run-103",
		"is_diagnostic": false,
		"summary":       "Implementation of inline editing is complete and functional, but product direction has changed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), productReview)

	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 55 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	specContent := "# Spec: Inline Editing\n\nAdd inline editing to all table cells so users can edit without opening a modal.\n\n## Acceptance Criteria\n- Users can click any cell to edit\n- Changes auto-save on blur\n"
	specDir := store.RunDir("run-103")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// === Invoke ===
	llmResponse := `{
		"proposals": [
			{
				"type": "refinement_guidance",
				"title": "Revise spec to replace inline editing with modal-based editing",
				"what_happened": "The outcome summary indicates product direction shifted away from inline editing implementation",
				"what_was_missing": "Spec refinement to align with new UX direction",
				"proposed_change": "Update the spec acceptance criteria to use modal-based editing instead of inline editing",
				"rationale": "Product direction shifted — we no longer want inline editing. The spec content describes inline editing as the approach, but this is no longer desired.",
				"confidence": "high",
				"confidence_rationale": "Outcome explicitly states product direction changed away from inline editing",
				"evidence_references": ["review-outcome.json", "spec.md"]
			},
			{
				"type": "refinement_guidance",
				"title": "Remove auto-save on blur acceptance criterion",
				"what_happened": "The spec lists 'Changes auto-save on blur' as an acceptance criterion specific to inline editing",
				"what_was_missing": "Acknowledgment that this criterion is tied to abandoned UX approach",
				"proposed_change": "Remove the auto-save on blur criterion and replace with explicit save action in modal workflow",
				"rationale": "Auto-save on blur is specific to inline editing UX which is being abandoned per the outcome summary: product direction shifted.",
				"confidence": "high",
				"confidence_rationale": "Auto-save on blur is specific to inline editing UX which is being abandoned per outcome summary",
				"evidence_references": ["review-outcome.json", "spec.md", "product-review.json"]
			},
			{
				"type": "refinement_guidance",
				"title": "Add stakeholder confirmation checkpoint for UX-heavy specs",
				"what_happened": "Implementation was completed successfully but rejected due to vision shift",
				"what_was_missing": "Early stakeholder alignment checkpoint before implementation",
				"proposed_change": "Specs with significant UX changes require stakeholder direction confirmation before starting implementation",
				"rationale": "The spec described inline editing features that were fully implemented, but product direction shifted making the work obsolete.",
				"confidence": "high",
				"confidence_rationale": "This run completed successfully but was rejected due to a direction change that could have been caught earlier",
				"evidence_references": ["review-outcome.json"]
			}
		]
	}`

	mockLLM := &mockLLMCompleter{response: llmResponse}

	reviewOutcomeData, err := os.ReadFile(filepath.Join(evidenceDir, "review-outcome.json"))
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}

	inputs := &reviewdistiller.DistillerInputs{
		RunID:         "run-103",
		SpecID:        "spec-inline-editing",
		SpecContent:   specContent,
		ReviewOutcome: json.RawMessage(reviewOutcomeData),
	}

	result, err := reviewdistiller.Distill(inputs, mockLLM, reviewdistiller.TierHigh)
	if err != nil {
		t.Fatalf("Distill() returned error: %v", err)
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
	fmt.Fprintf(&mdBuf, "**Run:** %s | **Outcome:** %s | **Model:** %s\n\n", result.RunID, result.Outcome, string(result.ModelTier))
	fmt.Fprintf(&mdBuf, "## Proposals\n\n")
	for i, p := range result.Proposals {
		fmt.Fprintf(&mdBuf, "### %d. [%s] %s\n\n", i+1, p.Type, p.Title)
		fmt.Fprintf(&mdBuf, "**What Happened:** %s\n\n", p.WhatHappened)
		fmt.Fprintf(&mdBuf, "**What Was Missing:** %s\n\n", p.WhatWasMissing)
		fmt.Fprintf(&mdBuf, "**Proposed Change:** %s\n\n", p.ProposedChange)
		fmt.Fprintf(&mdBuf, "**Rationale:** %s\n\n", p.Rationale)
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

	// 2. run_id is "run-103"
	if parsed.RunID != "run-103" {
		t.Errorf("expected run_id 'run-103', got %q", parsed.RunID)
	}

	// 3. Outcome is "rework_vision_change"
	if parsed.Outcome != "rework_vision_change" {
		t.Errorf("expected outcome 'rework_vision_change', got %q", parsed.Outcome)
	}

	// 4. At least one proposal exists
	if len(parsed.Proposals) == 0 {
		t.Fatal("expected at least one proposal, got 0")
	}

	// 5. At least one proposal of type refinement_guidance
	hasRefinementGuidance := false
	for _, p := range parsed.Proposals {
		if p.Type == "refinement_guidance" {
			hasRefinementGuidance = true
			break
		}
	}
	if !hasRefinementGuidance {
		types := make([]string, len(parsed.Proposals))
		for i, p := range parsed.Proposals {
			types[i] = p.Type
		}
		t.Errorf("expected at least one refinement_guidance proposal, got types: %v", types)
	}

	// 6. Proposals reference the outcome summary in their rationale
	hasOutcomeSummaryRef := false
	for _, p := range parsed.Proposals {
		if strings.Contains(p.Rationale, "direction shifted") || strings.Contains(p.Rationale, "inline editing") {
			hasOutcomeSummaryRef = true
			break
		}
	}
	if !hasOutcomeSummaryRef {
		t.Error("expected at least one proposal rationale to reference the outcome summary (direction shifted / inline editing)")
	}

	// 7. Proposals reference spec content in their rationale
	hasSpecContentRef := false
	for _, p := range parsed.Proposals {
		if strings.Contains(p.Rationale, "spec") || strings.Contains(p.Rationale, "inline editing") || strings.Contains(p.Rationale, "auto-save") {
			hasSpecContentRef = true
			break
		}
	}
	if !hasSpecContentRef {
		t.Error("expected at least one proposal rationale to reference spec content")
	}

	// 8. model_tier is populated
	if parsed.ModelTier == "" {
		t.Error("expected non-empty model_tier")
	}

	// 9. CreatedAt is non-zero
	if parsed.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}

	// 10. Each proposal has all required schema fields populated
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

	// 11. distillation-proposals.md exists and is non-empty
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	if len(mdData) == 0 {
		t.Error("expected non-empty distillation-proposals.md")
	}

	// 12. Markdown contains outcome
	if !strings.Contains(string(mdData), "rework_vision_change") {
		t.Error("distillation-proposals.md should mention rework_vision_change outcome")
	}

	// 13. Markdown mentions refinement_guidance proposal type
	if !strings.Contains(string(mdData), "refinement_guidance") {
		t.Error("distillation-proposals.md should mention refinement_guidance proposal type")
	}

	// 14. Markdown references the vision change context
	if !strings.Contains(string(mdData), "inline editing") {
		t.Error("distillation-proposals.md should reference inline editing from the vision change")
	}
}
