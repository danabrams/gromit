package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_OutcomeSpecificValidationRejectsNonConformingProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-110",
		SpecID:                "spec-dashboard-redesign",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 20, 10, 15, 0, 0, time.UTC),
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

	evidenceDir := store.RunEvidenceDir("run-110")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// review-outcome.json with outcome "rework_vision_change"
	reviewOutcome := map[string]interface{}{
		"run_id":      "run-110",
		"outcome":     "rework_vision_change",
		"summary":     "Dashboard redesign direction changed — stakeholders want a different layout",
		"reviewed_at": "2026-03-20T10:20:00Z",
		"manual_results": []map[string]string{
			{"id": "check-layout", "result": "fail", "notes": "Layout no longer matches desired direction"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	productReview := map[string]interface{}{
		"run_id":         "run-110",
		"terminal_state": "ready_for_review",
		"summary":        "Dashboard redesign completed but direction shifted",
	}
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), productReview)

	processReview := map[string]interface{}{
		"run_id":      "run-110",
		"trust_level": "high",
		"summary":     "Clean implementation, no process issues",
	}
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), processReview)

	manualChecklist := map[string]interface{}{
		"run_id": "run-110",
		"items": []map[string]string{
			{"id": "check-layout", "title": "Verify layout direction", "instructions": "Check dashboard layout"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "manual-checklist.json"), manualChecklist)

	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 40 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// Seed spec.md in the run directory
	runDir := store.RunDir("run-110")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	specContent := "# Dashboard Redesign\n\nRedesign the main dashboard layout.\n"
	if err := os.WriteFile(filepath.Join(runDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// === Invoke ===
	// Mock LLM returns 4 proposals, NONE of which is type "refinement_guidance".
	// For rework_vision_change, at least one refinement_guidance is required.
	nonConformingCompleter := &mockLLMCompleter{
		response: `{
  "proposals": [
    {
      "type": "doctrine_rule",
      "title": "Require stakeholder sign-off before UI redesigns",
      "what_happened": "Dashboard redesign was completed without final stakeholder approval",
      "what_was_missing": "Stakeholder sign-off step",
      "proposed_change": "Add mandatory stakeholder approval before starting UI redesign work",
      "rationale": "Prevents wasted effort on direction changes",
      "confidence": "high",
      "confidence_rationale": "Direction changed after implementation was complete",
      "evidence_references": ["review-outcome.json"]
    },
    {
      "type": "planner_heuristic",
      "title": "Break UI redesigns into wireframe and implementation phases",
      "what_happened": "Full implementation was done before layout direction was confirmed",
      "what_was_missing": "Phased approach with early validation",
      "proposed_change": "Plan UI redesigns in wireframe-first phases",
      "rationale": "Catches direction misalignment before full implementation",
      "confidence": "high",
      "confidence_rationale": "Standard UI design practice",
      "evidence_references": ["review-outcome.json"]
    },
    {
      "type": "validation_gap",
      "title": "Add layout conformance checks",
      "what_happened": "Layout did not match stakeholder expectations",
      "what_was_missing": "Automated layout conformance validation",
      "proposed_change": "Add snapshot tests for layout structure",
      "rationale": "Catches layout regressions early",
      "confidence": "medium",
      "confidence_rationale": "Useful but may not catch all direction issues",
      "evidence_references": ["validation.json"]
    },
    {
      "type": "doctrine_rule",
      "title": "Document design rationale in specs",
      "what_happened": "Spec did not explain the reasoning behind the chosen layout",
      "what_was_missing": "Design rationale documentation",
      "proposed_change": "Require design rationale section in UI specs",
      "rationale": "Makes it easier to evaluate direction changes",
      "confidence": "medium",
      "confidence_rationale": "Good practice for UI-heavy specs",
      "evidence_references": ["review-outcome.json"]
    }
  ]
}`,
	}

	distillErr := attemptDistillation("run-110", tmp, reviewdistiller.TierMedium, nonConformingCompleter)

	// === Assert ===

	// 1. Distillation must return an error
	if distillErr == nil {
		t.Fatal("expected attemptDistillation to return an error for non-conforming proposals, got nil")
	}

	// 2. Error message identifies the missing required proposal type "refinement_guidance"
	errMsg := distillErr.Error()
	if !strings.Contains(errMsg, "refinement_guidance") {
		t.Errorf("error should mention the missing required type 'refinement_guidance', got: %s", errMsg)
	}

	// 3. distillation-proposals.json is NOT written
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	if _, err := os.Stat(proposalsPath); err == nil {
		t.Error("distillation-proposals.json should not exist when validation rejects proposals")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking distillation-proposals.json: %v", err)
	}

	// 4. distillation-proposals.md is NOT written
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	if _, err := os.Stat(markdownPath); err == nil {
		t.Error("distillation-proposals.md should not exist when validation rejects proposals")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking distillation-proposals.md: %v", err)
	}

	// 5. review-outcome.json is unchanged
	outcomeData, err := os.ReadFile(filepath.Join(evidenceDir, "review-outcome.json"))
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}
	var parsedOutcome map[string]interface{}
	if err := json.Unmarshal(outcomeData, &parsedOutcome); err != nil {
		t.Fatalf("parse review-outcome.json: %v", err)
	}
	if parsedOutcome["outcome"] != "rework_vision_change" {
		t.Errorf("review-outcome.json outcome should still be 'rework_vision_change', got %q", parsedOutcome["outcome"])
	}

	// 6. Other evidence files are unaffected
	for _, name := range []string{"product-review.json", "process-review.json", "manual-checklist.json", "validation.json"} {
		path := filepath.Join(evidenceDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("evidence file %s should still exist after validation rejection: %v", name, err)
		}
	}

	// 7. The run itself is still loadable
	_, _, returnedEvidenceDir, loadErr := loadRunAndEnsurePacket("run-110", tmp)
	if loadErr != nil {
		t.Fatalf("loadRunAndEnsurePacket should succeed (run is valid): %v", loadErr)
	}
	if returnedEvidenceDir != evidenceDir {
		t.Errorf("expected evidence dir %q, got %q", evidenceDir, returnedEvidenceDir)
	}

	// 8. Verify the distiller itself also returns validation error (direct call)
	outcomeJSON, _ := json.Marshal(reviewOutcome)
	inputs := &reviewdistiller.DistillerInputs{
		RunID:         "run-110",
		SpecID:        "spec-dashboard-redesign",
		SpecContent:   specContent,
		ReviewOutcome: json.RawMessage(outcomeJSON),
	}
	result, directErr := reviewdistiller.Distill(inputs, nonConformingCompleter, reviewdistiller.TierMedium)
	if directErr == nil {
		t.Fatal("expected Distill to return error for non-conforming proposals, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result when validation fails, got %v", result)
	}
	if !strings.Contains(directErr.Error(), "refinement_guidance") {
		t.Errorf("direct Distill error should mention 'refinement_guidance', got: %s", directErr.Error())
	}
}
