package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// failingLLMCompleter is a mock LLMCompleter that returns an error to simulate
// LLM endpoint failures (e.g., connection refused, rate limit, etc.).
type failingLLMCompleter struct {
	failureMsg string
}

func (f *failingLLMCompleter) Complete(_ interface{}, _ string) (string, error) {
	return "", errors.New(f.failureMsg)
}

var _ reviewdistiller.LLMCompleter = (*failingLLMCompleter)(nil)

func TestScenario_DistillationFailureDoesNotBlockOutcomeRecording(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-104",
		SpecID:                "spec-auth-refactor",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 17, 9, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 17, 9, 8, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "opus"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed evidence directory with review packet artifacts
	evidenceDir := store.RunEvidenceDir("run-104")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Seed run directory with spec.md (required by attemptDistillation)
	runDir := store.RunDir("run-104")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	specContent := "# Auth Refactor Spec\n\nRefactor authentication middleware for performance and security."
	if err := os.WriteFile(filepath.Join(runDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	productReview := map[string]interface{}{
		"run_id":        "run-104",
		"is_diagnostic": false,
		"summary":       "Auth refactor meets all acceptance criteria",
	}
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), productReview)

	processReview := map[string]interface{}{
		"trust_level":         "high",
		"automatic_proof":     "All tests passed",
		"machine_review":      "No issues",
		"recommended_posture": "accept",
		"degraded_flags":      []string{},
		"repair_cycles":       0,
	}
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), processReview)

	manualChecklist := map[string]interface{}{
		"items": []map[string]string{
			{"id": "check-1", "title": "Auth flows work", "instructions": "Verify login/logout"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "manual-checklist.json"), manualChecklist)

	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 55 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	reviewOutcome := map[string]interface{}{
		"run_id":      "run-104",
		"outcome":     "accepted",
		"summary":     "All checks passed, auth refactor is solid",
		"reviewed_at": "2026-03-17T09:15:00Z",
		"manual_results": []map[string]interface{}{
			{"id": "check-1", "result": "pass", "notes": ""},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")

	// === Invoke ===
	// Call attemptDistillation with a failing LLMCompleter.
	// The distillation should fail, but review-outcome.json should remain intact.
	failingCompleter := &failingLLMCompleter{
		failureMsg: "LLM endpoint unreachable: connection refused",
	}

	distillErr := attemptDistillation("run-104", tmp, reviewdistiller.TierMedium, failingCompleter)
	if distillErr == nil {
		t.Fatalf("expected attemptDistillation to return an error, got nil")
	}

	// === Assert ===

	// 1. review-outcome.json exists and is parseable
	rawOutcome, err := os.ReadFile(outcomeFile)
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}
	var parsedOutcome map[string]interface{}
	if err := json.Unmarshal(rawOutcome, &parsedOutcome); err != nil {
		t.Fatalf("parse review-outcome.json: %v", err)
	}

	// 2. Outcome is "accepted"
	if parsedOutcome["outcome"] != "accepted" {
		t.Errorf("expected outcome 'accepted', got %q", parsedOutcome["outcome"])
	}

	// 3. run_id is "run-104"
	if parsedOutcome["run_id"] != "run-104" {
		t.Errorf("expected run_id 'run-104', got %q", parsedOutcome["run_id"])
	}

	// 4. summary is populated
	if parsedOutcome["summary"] == nil || parsedOutcome["summary"] == "" {
		t.Error("expected non-empty summary in review-outcome.json")
	}

	// 5. reviewed_at is populated
	if parsedOutcome["reviewed_at"] == nil || parsedOutcome["reviewed_at"] == "" {
		t.Error("expected non-empty reviewed_at in review-outcome.json")
	}

	// 6. manual_results is populated
	manualResults, ok := parsedOutcome["manual_results"].([]interface{})
	if !ok || len(manualResults) == 0 {
		t.Error("expected non-empty manual_results in review-outcome.json")
	}

	// 7. distillation-proposals.json is absent
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	if _, err := os.Stat(proposalsPath); err == nil {
		t.Error("distillation-proposals.json should be absent when distillation fails")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking distillation-proposals.json: %v", err)
	}

	// 8. distillation-proposals.md is absent
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	if _, err := os.Stat(markdownPath); err == nil {
		t.Error("distillation-proposals.md should be absent when distillation fails")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking distillation-proposals.md: %v", err)
	}

	// 9. Other evidence files are unaffected by the distillation failure
	for _, name := range []string{"product-review.json", "process-review.json", "manual-checklist.json", "validation.json"} {
		path := filepath.Join(evidenceDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("evidence file %s should still exist after distillation failure: %v", name, err)
		}
	}
}
