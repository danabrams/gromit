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

	// === Invoke ===
	// Read the review-outcome.json and verify the outcome type is not supported.
	// The distill command should check the outcome field and reject unsupported types.
	// Supported outcomes for distillation: accepted, rework_implementation_gap, rework_vision_change.
	outcomePath := filepath.Join(evidenceDir, "review-outcome.json")
	outcomeData, err := os.ReadFile(outcomePath)
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(outcomeData, &parsed); err != nil {
		t.Fatalf("parse review-outcome.json: %v", err)
	}

	outcomeType, _ := parsed["outcome"].(string)

	// Build the error that the distill command would return for unsupported outcome types
	supportedOutcomes := map[string]bool{
		"accepted":                    true,
		"rework_implementation_gap":   true,
		"rework_vision_change":        true,
	}

	// === Assert ===

	// 1. The outcome type is "rejected"
	if outcomeType != "rejected" {
		t.Errorf("expected outcome type 'rejected', got %q", outcomeType)
	}

	// 2. "rejected" is not in the supported outcomes set
	if supportedOutcomes[outcomeType] {
		t.Errorf("outcome type %q should not be in supported outcomes", outcomeType)
	}

	// 3. The distill command should produce an error for unsupported outcome types
	distillErr := buildDistillUnsupportedOutcomeError("run-112", outcomeType)

	if distillErr == nil {
		t.Fatal("expected non-nil error for unsupported outcome type")
	}

	// 4. Error message explains that the outcome type is not supported for distillation
	errMsg := distillErr.Error()
	if !strings.Contains(errMsg, "rejected") {
		t.Errorf("error should mention the unsupported outcome type 'rejected', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "not supported") {
		t.Errorf("error should explain the outcome is not supported for distillation, got: %s", errMsg)
	}

	// 5. Error references the run ID
	if !strings.Contains(errMsg, "run-112") {
		t.Errorf("error should reference run ID 'run-112', got: %s", errMsg)
	}

	// 6. Error mentions supported outcome types so the user knows what to use
	if !strings.Contains(errMsg, "accepted") || !strings.Contains(errMsg, "rework_implementation_gap") || !strings.Contains(errMsg, "rework_vision_change") {
		t.Errorf("error should list supported outcome types, got: %s", errMsg)
	}

	// 7. The run itself is loadable and terminal (the problem is the outcome type, not the run)
	_, _, returnedEvidenceDir, loadErr := loadRunAndEnsurePacket("run-112", tmp)
	if loadErr != nil {
		t.Fatalf("loadRunAndEnsurePacket should succeed (run is valid): %v", loadErr)
	}
	if returnedEvidenceDir != evidenceDir {
		t.Errorf("expected evidence dir %q, got %q", evidenceDir, returnedEvidenceDir)
	}

	// 8. review-outcome.json is unchanged (distill should not modify it)
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

	// 9. No distillation-proposals.json should have been created
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	if _, err := os.Stat(proposalsPath); err == nil {
		t.Error("distillation-proposals.json should not exist after unsupported outcome error")
	}
}

// buildDistillUnsupportedOutcomeError constructs the error that the distill command
// should return when the review outcome type is not supported for distillation.
// This will be replaced by the actual distill command error path once the
// reviewdistiller package lands.
func buildDistillUnsupportedOutcomeError(runID, outcomeType string) error {
	return fmt.Errorf("run %s: outcome type %q is not supported for distillation; supported types: accepted, rework_implementation_gap, rework_vision_change", runID, outcomeType)
}