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

func TestScenario_UnrecognizedOutcomeTypeReturnsError(t *testing.T) {
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

	evidenceDir := store.RunEvidenceDir("run-112")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// review-outcome.json with unsupported outcome "rejected"
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

	// Seed review packet artifacts (required by attemptDistillation → loadRunAndEnsurePacket)
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), map[string]interface{}{
		"run_id":         "run-112",
		"terminal_state": "ready_for_review",
		"summary":        "Widget refactor incomplete",
	})
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), map[string]interface{}{
		"run_id":      "run-112",
		"trust_level": "medium",
		"summary":     "Multiple rework cycles observed",
	})
	writeJSON(t, filepath.Join(evidenceDir, "manual-checklist.json"), map[string]interface{}{
		"run_id": "run-112",
		"items": []map[string]string{
			{"id": "check-1", "title": "Verify widget rendering"},
		},
	})

	// Seed spec.md in run directory
	runDir := store.RunDir("run-112")
	specContent := "# Widget Refactor\n\nRefactor widget rendering.\n"
	if err := os.WriteFile(filepath.Join(runDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// === Invoke ===
	outcomeData, err := os.ReadFile(filepath.Join(evidenceDir, "review-outcome.json"))
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}

	inputs := &reviewdistiller.DistillerInputs{
		RunID:         "run-112",
		SpecID:        "spec-widget-refactor",
		SpecContent:   specContent,
		ReviewOutcome: json.RawMessage(outcomeData),
	}

	mockLLM := &mockLLMCompleter{
		response: `{"proposals": [{"type": "doctrine_rule", "title": "Test", "what_happened": "x", "what_was_missing": "y", "proposed_change": "z", "rationale": "r", "confidence": "high", "confidence_rationale": "c", "evidence_references": []}]}`,
	}

	result, distillErr := reviewdistiller.Distill(inputs, mockLLM, reviewdistiller.TierMedium)

	// === Assert ===

	// 1. Distill returns an error
	if distillErr == nil {
		t.Fatal("expected error from Distill for unsupported outcome type 'rejected', got nil")
	}

	// 2. Result is nil
	if result != nil {
		t.Errorf("expected nil result when error occurs, got %v", result)
	}

	// 3. Error mentions the unsupported outcome type
	errMsg := distillErr.Error()
	if !strings.Contains(errMsg, "rejected") {
		t.Errorf("error should mention 'rejected', got: %s", errMsg)
	}

	// 4. Error indicates the outcome is unrecognized or unsupported
	if !strings.Contains(errMsg, "unrecognized") && !strings.Contains(errMsg, "not supported") && !strings.Contains(errMsg, "unsupported") {
		t.Errorf("error should indicate unrecognized/unsupported outcome, got: %s", errMsg)
	}

	// 5. Error lists supported outcome types
	if !strings.Contains(errMsg, "accepted") || !strings.Contains(errMsg, "rework_implementation_gap") || !strings.Contains(errMsg, "rework_vision_change") {
		t.Errorf("error should list supported outcome types, got: %s", errMsg)
	}

	// 6. The run is still loadable (problem is outcome type, not run state)
	_, _, _, loadErr := loadRunAndEnsurePacket("run-112", tmp)
	if loadErr != nil {
		t.Fatalf("loadRunAndEnsurePacket should succeed for valid run: %v", loadErr)
	}

	// 7. review-outcome.json is unchanged
	afterData, err := os.ReadFile(filepath.Join(evidenceDir, "review-outcome.json"))
	if err != nil {
		t.Fatalf("re-read review-outcome.json: %v", err)
	}
	var afterParsed map[string]interface{}
	if err := json.Unmarshal(afterData, &afterParsed); err != nil {
		t.Fatalf("re-parse review-outcome.json: %v", err)
	}
	if afterParsed["outcome"] != "rejected" {
		t.Errorf("outcome should still be 'rejected', got %q", afterParsed["outcome"])
	}

	// 8. No distillation-proposals.json created
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	if _, err := os.Stat(proposalsPath); err == nil {
		t.Error("distillation-proposals.json should not exist after unsupported outcome error")
	}
}
