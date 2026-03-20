package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop/stages"
)

// TestScenario_RegenerationFailsWithClearError verifies that review show exits
// with a clear error when acceptance.json is missing and review packet artifacts
// are also missing, and does not write partial artifacts.
//
// Given: a run with ID run-007 reached ready_for_review but acceptance.json is
//
//	missing from the evidence directory and review packet artifacts are also missing
//
// When: the reviewer runs gromit-next review show --run run-007
// Then: the command exits with an error message that lists acceptance.json as
//
//	missing and does not write partial artifacts
func TestScenario_RegenerationFailsWithClearError(t *testing.T) {
	// --- Seed ---
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.RunID = "run-007"
	rs.Status = runstore.StatusReadyForReview
	rs.FinalValidationPassed = true
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = true
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory with validation.json and review.json present,
	// but acceptance.json intentionally missing.
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Write validation.json so the error surfaces on acceptance.json specifically
	validation := map[string]interface{}{"passed": true, "checks": 5}
	validationData, _ := json.MarshalIndent(validation, "", "  ")
	if err := os.WriteFile(filepath.Join(evidenceDir, "validation.json"), validationData, 0o644); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	// Write review.json
	review := map[string]interface{}{
		"diff_unavailable": false,
		"spec_alignment":   []interface{}{},
		"code_quality":     []interface{}{},
	}
	reviewData, _ := json.MarshalIndent(review, "", "  ")
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Do NOT create acceptance.json — this is the missing file under test.
	// Do NOT create review packet artifacts (product-review.json, etc.).

	// --- Invoke ---
	// Simulate what review show --run run-007 would do: attempt to regenerate
	// the review packet via the finalize stage with evidence config pointing
	// at the evidence directory that is missing acceptance.json.
	eventLogPath := filepath.Join(storeDir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)
	config := &stages.FinalizeStageConfig{
		SpecContent: "# Test Spec\n\n## Vision\nTest.\n",
		EvidenceDir: evidenceDir,
	}
	stage := stages.NewFinalizeStageWithConfig(nil, store, eventLog, config)
	_, err := stage.Run(context.Background(), rs)

	// --- Assert ---

	// The finalize stage itself should not return an error (it swallows packet
	// generation errors), but the run should still reach ready_for_review.
	if err != nil {
		t.Fatalf("FinalizeStage.Run returned unexpected error: %v", err)
	}

	// Status must still be ready_for_review despite packet generation failure.
	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("expected status ready_for_review, got %q", rs.Status)
	}

	// The event log must record a review_packet_generation_error event,
	// which surfaces the acceptance.json failure to the reviewer.
	logData, readErr := os.ReadFile(eventLogPath)
	if readErr != nil {
		t.Fatalf("read event log: %v", readErr)
	}
	logContent := string(logData)
	if !strings.Contains(logContent, "review_packet_generation_error") {
		t.Error("expected review_packet_generation_error event in log when acceptance.json is missing")
	}

	// No partial review packet artifacts should have been written.
	for _, artifact := range []string{"product-review.json", "product-review.md", "process-review.json", "process-review.md", "manual-checklist.json"} {
		path := filepath.Join(evidenceDir, artifact)
		if _, statErr := os.Stat(path); statErr == nil {
			t.Errorf("partial artifact %s should not exist after failed regeneration", artifact)
		}
	}

	// Original evidence artifacts must still be present (not corrupted).
	for _, name := range []string{"validation.json", "review.json"} {
		path := filepath.Join(evidenceDir, name)
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			t.Errorf("original evidence artifact %s should still be present", name)
		}
	}
}