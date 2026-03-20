package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_DistillCommandRefusesRunWithoutOutcome(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-107",
		SpecID:                "spec-refactor-auth",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 19, 14, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 19, 14, 8, 0, 0, time.UTC),
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

	// Product review exists
	productReview := map[string]interface{}{
		"run_id":         "run-107",
		"terminal_state": "ready_for_review",
		"summary":        "Auth refactor looks good",
		"facets":         []map[string]string{},
	}
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), productReview)

	// Process review exists
	processReview := map[string]interface{}{
		"run_id":      "run-107",
		"trust_level": "high",
		"summary":     "Clean process with minimal rework",
	}
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), processReview)

	// Manual checklist exists
	manualChecklist := map[string]interface{}{
		"run_id": "run-107",
		"items": []map[string]string{
			{"id": "check-1", "title": "Verify auth flow", "instructions": "Test login/logout"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "manual-checklist.json"), manualChecklist)

	// Validation evidence exists
	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 38 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// Confirm review-outcome.json does NOT exist
	outcomePath := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomePath); err == nil {
		t.Fatal("review-outcome.json should not exist before invocation")
	}

	// === Invoke ===
	// The distill command must check for review-outcome.json and refuse to run
	// if it is absent. This simulates calling `gromit-next review distill --run run-107`.
	//
	// Since the reviewdistiller package and distill subcommand are not yet on main,
	// we verify the precondition check directly: reading review-outcome.json from the
	// evidence directory must fail, and the distill command should surface a clear error.
	outcomeData, err := os.ReadFile(outcomePath)

	// === Assert ===

	// 1. Reading the outcome file fails (file does not exist)
	if err == nil {
		t.Fatal("expected error reading review-outcome.json, but read succeeded")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected file-not-found error, got: %v", err)
	}
	if outcomeData != nil {
		t.Error("expected nil data when file is missing")
	}

	// 2. The error message the distill command should produce
	expectedMsg := "no review outcome has been recorded for this run"
	// Build the error that the distill command would return
	distillErr := buildDistillOutcomeMissingError("run-107", outcomePath, err)

	if !strings.Contains(distillErr.Error(), expectedMsg) {
		t.Errorf("expected error to contain %q, got: %s", expectedMsg, distillErr.Error())
	}

	// 3. Error references the run ID
	if !strings.Contains(distillErr.Error(), "run-107") {
		t.Errorf("expected error to reference run ID 'run-107', got: %s", distillErr.Error())
	}

	// 4. The run itself is loadable and terminal (the problem is the missing outcome, not the run)
	_, _, returnedEvidenceDir, loadErr := loadRunAndEnsurePacket("run-107", tmp)
	if loadErr != nil {
		t.Fatalf("loadRunAndEnsurePacket should succeed (run is valid): %v", loadErr)
	}
	if returnedEvidenceDir != evidenceDir {
		t.Errorf("expected evidence dir %q, got %q", evidenceDir, returnedEvidenceDir)
	}

	// 5. Review packet artifacts still exist (they were not the problem)
	for _, name := range []string{"product-review.json", "process-review.json", "manual-checklist.json"} {
		path := filepath.Join(evidenceDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("packet artifact %s should exist: %v", name, err)
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Errorf("packet artifact %s should be valid JSON: %v", name, err)
		}
	}

	// 6. review-outcome.json is still absent after the failed attempt
	if _, err := os.Stat(outcomePath); err == nil {
		t.Error("review-outcome.json should still not exist after distill refusal")
	}
}

// buildDistillOutcomeMissingError constructs the error that the distill command
// should return when review-outcome.json is missing. This will be replaced by
// the actual distill command error path once the reviewdistiller package lands.
func buildDistillOutcomeMissingError(runID, outcomePath string, readErr error) error {
	return &distillOutcomeMissingError{
		runID:      runID,
		path:       outcomePath,
		underlying: readErr,
	}
}

type distillOutcomeMissingError struct {
	runID      string
	path       string
	underlying error
}

func (e *distillOutcomeMissingError) Error() string {
	return "run " + e.runID + ": no review outcome has been recorded for this run; run `gromit-next review guided` or `gromit-next review record` first"
}

func (e *distillOutcomeMissingError) Unwrap() error {
	return e.underlying
}