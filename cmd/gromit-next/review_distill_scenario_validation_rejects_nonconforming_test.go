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

func TestScenario_DistillCommandRejectsUnrecognizedOutcomeType(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-112",
		SpecID:                "spec-widget-refactor",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 20, 11, 10, 0, 0, time.UTC),
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

	// Seed evidence directory with review packet artifacts
	evidenceDir := store.RunEvidenceDir("run-112")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// review-outcome.json with unsupported outcome type "rejected"
	reviewOutcome := map[string]interface{}{
		"run_id":      "run-112",
		"outcome":     "rejected",
		"summary":     "Work does not meet requirements",
		"reviewed_at": "2026-03-20T11:15:00Z",
		"manual_results": []map[string]string{
			{"id": "check-1", "result": "fail", "notes": "Requirements not met"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	// Product review
	productReview := map[string]interface{}{
		"run_id":         "run-112",
		"terminal_state": "ready_for_review",
		"summary":        "Widget refactor incomplete",
		"facets":         []map[string]string{},
	}
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), productReview)

	// Process review
	processReview := map[string]interface{}{
		"run_id":      "run-112",
		"trust_level": "medium",
		"summary":     "Multiple rework cycles observed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), processReview)

	// Manual checklist
	manualChecklist := map[string]interface{}{
		"run_id": "run-112",
		"items": []map[string]string{
			{"id": "check-1", "title": "Verify widget rendering", "instructions": "Check output"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "manual-checklist.json"), manualChecklist)

	// Validation evidence
	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 25 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// Seed spec.md in the run directory (required by distiller)
	runDir := store.RunDir("run-112")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	specPath := filepath.Join(runDir, "spec.md")
	specContent := `# Widget Refactor Spec

## Vision
Refactor widget rendering system.

## Scenarios
### Scenario: widget renders correctly
**Given** a widget
**When** rendered
**Then** output matches expected structure
`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// === Invoke ===
	// Load review outcome data
	outcomePath := filepath.Join(evidenceDir, "review-outcome.json")
	outcomeData, err := os.ReadFile(outcomePath)
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}

	// Build DistillerInputs with the unsupported outcome type
	inputs := &reviewdistiller.DistillerInputs{
		RunID:         "run-112",
		SpecID:        "spec-widget-refactor",
		SpecContent:   specContent,
		ReviewOutcome: json.RawMessage(outcomeData),
	}

	// Create a mock LLM completer that returns mock proposals
	mockLLM := &mockLLMCompleter{
		response: `{
  "proposals": [
    {
      "type": "doctrine_rule",
      "title": "Test doctrine",
      "what_happened": "Something",
      "what_was_missing": "Missing thing",
      "proposed_change": "Change it",
      "rationale": "Good reason",
      "confidence": "high",
      "confidence_rationale": "Clear reason",
      "evidence_references": []
    }
  ]
}`,
	}

	// Attempt distillation with unsupported outcome type
	result, distillErr := reviewdistiller.Distill(inputs, mockLLM, reviewdistiller.TierMedium)

	// === Assert ===

	// 1. Distill should return an error (not nil)
	if distillErr == nil {
		t.Fatal("expected error from Distill for unsupported outcome type 'rejected', got nil")
	}

	// 2. Result should be nil when error occurs
	if result != nil {
		t.Errorf("expected nil result when error occurs, got %v", result)
	}

	// 3. Error message explains that the outcome type is not supported
	errMsg := distillErr.Error()
	if !strings.Contains(errMsg, "rejected") {
		t.Errorf("error should mention the unsupported outcome type 'rejected', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "unsupported") && !strings.Contains(errMsg, "unrecognized") {
		t.Errorf("error should indicate unsupported/unrecognized outcome type, got: %s", errMsg)
	}

	// 4. Error mentions supported outcome types so the user knows what to use
	if !strings.Contains(errMsg, "accepted") || !strings.Contains(errMsg, "rework_implementation_gap") || !strings.Contains(errMsg, "rework_vision_change") {
		t.Errorf("error should list supported outcome types (accepted, rework_implementation_gap, rework_vision_change), got: %s", errMsg)
	}

	// 5. The run itself is loadable and terminal (the problem is the outcome type, not the run)
	_, _, returnedEvidenceDir, loadErr := loadRunAndEnsurePacket("run-112", tmp)
	if loadErr != nil {
		t.Fatalf("loadRunAndEnsurePacket should succeed (run is valid): %v", loadErr)
	}
	if returnedEvidenceDir != evidenceDir {
		t.Errorf("expected evidence dir %q, got %q", evidenceDir, returnedEvidenceDir)
	}

	// 6. review-outcome.json is unchanged (distill should not modify it)
	afterData, err := os.ReadFile(outcomePath)
	if err != nil {
		t.Fatalf("re-read review-outcome.json: %v", err)
	}
	var afterParsed map[string]interface{}
	if err := json.Unmarshal(afterData, &afterParsed); err != nil {
		t.Fatalf("re-parse review-outcome.json: %v", err)
	}
	if afterParsed["outcome"] != "rejected" {
		t.Errorf("outcome should still be 'rejected' after distill refusal, got %q", afterParsed["outcome"])
	}

	// 7. No distillation-proposals.json should have been created
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	if _, err := os.Stat(proposalsPath); err == nil {
		t.Error("distillation-proposals.json should not exist after unsupported outcome error")
	}
}
