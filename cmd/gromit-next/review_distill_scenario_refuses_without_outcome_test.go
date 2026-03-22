package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_DistillCommandRefusesRunWithoutOutcome(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-107",
		SpecID:                "spec-data-export",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 19, 10, 8, 0, 0, time.UTC),
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

	// Seed evidence directory with review packet artifacts but NO review-outcome.json
	evidenceDir := store.RunEvidenceDir("run-107")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	productReview := map[string]interface{}{
		"run_id":         "run-107",
		"terminal_state": "ready_for_review",
		"summary":        "Data export feature implemented",
	}
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), productReview)

	processReview := map[string]interface{}{
		"trust_level":         "high",
		"automatic_proof":     "All tests passed",
		"machine_review":      "No issues found",
		"recommended_posture": "stamp_if_clean",
		"degraded_flags":      []string{},
		"repair_cycles":       0,
	}
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), processReview)

	manualChecklist := map[string]interface{}{
		"run_id": "run-107",
		"items": []map[string]string{
			{"id": "check-export-format", "title": "Verify export format", "instructions": "Check CSV output"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "manual-checklist.json"), manualChecklist)

	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 30 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// Seed spec.md in run directory
	runDir := store.RunDir("run-107")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	specContent := "# Data Export\n\nExport data to CSV format.\n"
	if err := os.WriteFile(filepath.Join(runDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// Confirm review-outcome.json does NOT exist
	outcomePath := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomePath); err == nil {
		t.Fatal("precondition: review-outcome.json should not exist")
	}

	// === Invoke ===
	mockLLM := &mockLLMCompleter{response: `{"proposals":[]}`}
	distillErr := attemptDistillation("run-107", tmp, reviewdistiller.TierMedium, mockLLM)

	// === Assert ===

	// 1. attemptDistillation must return an error
	if distillErr == nil {
		t.Fatal("expected attemptDistillation to return an error when review-outcome.json is missing, got nil")
	}

	// 2. Error message explains that no review outcome has been recorded
	errMsg := distillErr.Error()
	if !strings.Contains(errMsg, "review outcome") || !strings.Contains(errMsg, "run-107") {
		t.Errorf("error should mention missing review outcome for run-107, got: %s", errMsg)
	}

	// 3. No distillation-proposals.json should have been created
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	if _, err := os.Stat(proposalsPath); err == nil {
		t.Error("distillation-proposals.json should not exist when review-outcome.json is missing")
	}

	// 4. No distillation-proposals.md should have been created
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	if _, err := os.Stat(markdownPath); err == nil {
		t.Error("distillation-proposals.md should not exist when review-outcome.json is missing")
	}

	// 5. Existing review packet artifacts are unaffected
	for _, name := range []string{"product-review.json", "process-review.json", "manual-checklist.json", "validation.json"} {
		path := filepath.Join(evidenceDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("evidence file %s should still exist: %v", name, err)
		}
	}

	// 6. review-outcome.json still does not exist (distill did not create one)
	if _, err := os.Stat(outcomePath); err == nil {
		t.Error("review-outcome.json should still not exist after distill refusal")
	}
}
